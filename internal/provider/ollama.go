package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// OllamaProvider uses Ollama's OpenAI-compatible API endpoint.
type OllamaProvider struct {
	name    string
	baseURL string
	client  *http.Client
	healthy atomic.Bool
	models  []config.ModelConfig
}

func NewOllama(cfg config.ProviderConfig) *OllamaProvider {
	p := &OllamaProvider{
		name:    "ollama",
		baseURL: cfg.BaseURL,
		client:  &http.Client{},
		models:  cfg.Models,
	}
	p.healthy.Store(true)
	return p
}

func (p *OllamaProvider) Name() string  { return p.name }
func (p *OllamaProvider) Healthy() bool { return p.healthy.Load() }

func (p *OllamaProvider) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	if req.Model == "" && len(p.models) > 0 {
		req.Model = p.models[0].Name
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		p.healthy.Store(false)
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		p.healthy.Store(false)
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	p.healthy.Store(true)

	var result openai.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// ChatCompletionStream returns the raw HTTP response for SSE streaming.
func (p *OllamaProvider) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest) (*http.Response, error) {
	if req.Model == "" && len(p.models) > 0 {
		req.Model = p.models[0].Name
	}
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		p.healthy.Store(false)
		return nil, fmt.Errorf("sending request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		p.healthy.Store(false)
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	p.healthy.Store(true)
	return resp, nil
}
