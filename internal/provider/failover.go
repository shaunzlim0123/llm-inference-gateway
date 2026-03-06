package provider

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// GuardedProvider wraps a Provider with a circuit breaker.
type GuardedProvider struct {
	inner   Provider
	breaker *gobreaker.CircuitBreaker[*openai.ChatCompletionResponse]
}

func NewGuardedProvider(inner Provider, cbCfg config.CircuitBreakerConfig) *GuardedProvider {
	settings := gobreaker.Settings{
		Name:        inner.Name(),
		MaxRequests: cbCfg.MaxRequests,
		Interval:    cbCfg.Interval,
		Timeout:     cbCfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= uint32(cbCfg.FailureThreshold)
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Warn("circuit breaker state change",
				"provider", name,
				"from", from.String(),
				"to", to.String(),
			)
		},
	}

	return &GuardedProvider{
		inner:   inner,
		breaker: gobreaker.NewCircuitBreaker[*openai.ChatCompletionResponse](settings),
	}
}

func (g *GuardedProvider) Name() string { return g.inner.Name() }

func (g *GuardedProvider) Healthy() bool {
	return g.inner.Healthy() && g.breaker.State() != gobreaker.StateOpen
}

func (g *GuardedProvider) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	resp, err := g.breaker.Execute(func() (*openai.ChatCompletionResponse, error) {
		return g.inner.ChatCompletion(ctx, req)
	})
	if err != nil {
		return nil, fmt.Errorf("circuit breaker [%s]: %w", g.inner.Name(), err)
	}
	return resp, nil
}

// ChatCompletionStream delegates to the inner provider (circuit breaker tracks via non-stream calls).
func (g *GuardedProvider) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest) (*http.Response, error) {
	if g.breaker.State() == gobreaker.StateOpen {
		return nil, fmt.Errorf("circuit breaker [%s]: circuit is open", g.inner.Name())
	}
	return g.inner.ChatCompletionStream(ctx, req)
}

// FailoverChain tries providers in order, with exponential backoff on retries.
type FailoverChain struct {
	providers []Provider
}

func NewFailoverChain(providers []Provider) *FailoverChain {
	return &FailoverChain{providers: providers}
}

func (f *FailoverChain) Call(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, string, error) {
	var lastErr error
	for _, p := range f.providers {
		if !p.Healthy() {
			slog.Debug("skipping unhealthy provider", "provider", p.Name())
			continue
		}

		resp, err := callWithRetry(ctx, p, req, 2)
		if err != nil {
			slog.Error("provider failed", "provider", p.Name(), "error", err)
			lastErr = err
			continue
		}

		return resp, p.Name(), nil
	}

	if lastErr != nil {
		return nil, "", fmt.Errorf("all providers failed: %w", lastErr)
	}
	return nil, "", fmt.Errorf("no healthy providers available")
}

func callWithRetry(ctx context.Context, p Provider, req *openai.ChatCompletionRequest, maxRetries int) (*openai.ChatCompletionResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := p.ChatCompletion(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
