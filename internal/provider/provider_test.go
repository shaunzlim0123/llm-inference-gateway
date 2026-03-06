package provider

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// mockProvider is a test double for the Provider interface.
type mockProvider struct {
	name      string
	healthy   bool
	response  *openai.ChatCompletionResponse
	err       error
	callCount int
}

func (m *mockProvider) Name() string  { return m.name }
func (m *mockProvider) Healthy() bool { return m.healthy }
func (m *mockProvider) ChatCompletion(_ context.Context, _ *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	m.callCount++
	return m.response, m.err
}
func (m *mockProvider) ChatCompletionStream(_ context.Context, _ *openai.ChatCompletionRequest) (*http.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	mock := &mockProvider{name: "test-provider", healthy: true}

	reg.Register(mock)

	p, err := reg.Get("test-provider")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if p.Name() != "test-provider" {
		t.Errorf("expected name 'test-provider', got %s", p.Name())
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
}

func TestRegistry_All(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "a", healthy: true})
	reg.Register(&mockProvider{name: "b", healthy: true})

	all := reg.All()
	if len(all) != 2 {
		t.Errorf("expected 2 providers, got %d", len(all))
	}
}

func TestGuardedProvider_Success(t *testing.T) {
	mock := &mockProvider{
		name:    "test",
		healthy: true,
		response: &openai.ChatCompletionResponse{
			ID:    "resp-1",
			Model: "gpt-4o-mini",
		},
	}

	guarded := NewGuardedProvider(mock, config.CircuitBreakerConfig{
		MaxRequests:      5,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		FailureThreshold: 3,
	})

	resp, err := guarded.ChatCompletion(context.Background(), &openai.ChatCompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "resp-1" {
		t.Errorf("expected resp-1, got %s", resp.ID)
	}
	if !guarded.Healthy() {
		t.Error("expected guarded provider to be healthy")
	}
}

func TestGuardedProvider_CircuitBreaker_Opens(t *testing.T) {
	mock := &mockProvider{
		name:    "test",
		healthy: true,
		err:     fmt.Errorf("upstream error"),
	}

	guarded := NewGuardedProvider(mock, config.CircuitBreakerConfig{
		MaxRequests:      1,
		Interval:         60 * time.Second,
		Timeout:          1 * time.Second,
		FailureThreshold: 3,
	})

	// Trigger 3 consecutive failures to trip the breaker
	for i := 0; i < 3; i++ {
		guarded.ChatCompletion(context.Background(), &openai.ChatCompletionRequest{})
	}

	if guarded.Healthy() {
		t.Error("expected guarded provider to be unhealthy after circuit breaker opens")
	}
}

func TestFailoverChain_FirstProviderSucceeds(t *testing.T) {
	chain := NewFailoverChain([]Provider{
		&mockProvider{
			name:     "primary",
			healthy:  true,
			response: &openai.ChatCompletionResponse{ID: "from-primary"},
		},
		&mockProvider{
			name:     "secondary",
			healthy:  true,
			response: &openai.ChatCompletionResponse{ID: "from-secondary"},
		},
	})

	resp, name, err := chain.Call(context.Background(), &openai.ChatCompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "primary" {
		t.Errorf("expected primary, got %s", name)
	}
	if resp.ID != "from-primary" {
		t.Errorf("expected from-primary, got %s", resp.ID)
	}
}

func TestFailoverChain_FallsToSecondary(t *testing.T) {
	chain := NewFailoverChain([]Provider{
		&mockProvider{
			name:    "primary",
			healthy: true,
			err:     fmt.Errorf("primary down"),
		},
		&mockProvider{
			name:     "secondary",
			healthy:  true,
			response: &openai.ChatCompletionResponse{ID: "from-secondary"},
		},
	})

	resp, name, err := chain.Call(context.Background(), &openai.ChatCompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "secondary" {
		t.Errorf("expected secondary, got %s", name)
	}
	if resp.ID != "from-secondary" {
		t.Errorf("expected from-secondary, got %s", resp.ID)
	}
}

func TestFailoverChain_SkipsUnhealthy(t *testing.T) {
	unhealthy := &mockProvider{
		name:     "primary",
		healthy:  false,
		response: &openai.ChatCompletionResponse{ID: "should-not-reach"},
	}
	healthy := &mockProvider{
		name:     "secondary",
		healthy:  true,
		response: &openai.ChatCompletionResponse{ID: "from-secondary"},
	}

	chain := NewFailoverChain([]Provider{unhealthy, healthy})

	_, name, err := chain.Call(context.Background(), &openai.ChatCompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "secondary" {
		t.Errorf("expected secondary, got %s", name)
	}
	if unhealthy.callCount != 0 {
		t.Errorf("unhealthy provider should not have been called, got %d calls", unhealthy.callCount)
	}
}

func TestFailoverChain_AllFail(t *testing.T) {
	chain := NewFailoverChain([]Provider{
		&mockProvider{name: "a", healthy: true, err: fmt.Errorf("fail-a")},
		&mockProvider{name: "b", healthy: true, err: fmt.Errorf("fail-b")},
	})

	_, _, err := chain.Call(context.Background(), &openai.ChatCompletionRequest{})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestFailoverChain_NoHealthy(t *testing.T) {
	chain := NewFailoverChain([]Provider{
		&mockProvider{name: "a", healthy: false},
		&mockProvider{name: "b", healthy: false},
	})

	_, _, err := chain.Call(context.Background(), &openai.ChatCompletionRequest{})
	if err == nil {
		t.Fatal("expected error when no providers are healthy")
	}
}
