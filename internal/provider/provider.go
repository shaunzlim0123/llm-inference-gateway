package provider

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// Provider is the interface every LLM backend implements.
type Provider interface {
	Name() string
	ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error)
	ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest) (*http.Response, error)
	Healthy() bool
}

// Registry holds all configured providers keyed by name.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}
