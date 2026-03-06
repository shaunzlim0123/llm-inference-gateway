package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// Anthropic API types (request/response translation layer)
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type AnthropicProvider struct {
	name    string
	apiKey  string
	baseURL string
	client  *http.Client
	healthy atomic.Bool
	models  []config.ModelConfig
}

func NewAnthropic(cfg config.ProviderConfig) *AnthropicProvider {
	p := &AnthropicProvider{
		name:    "anthropic",
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		client:  &http.Client{},
		models:  cfg.Models,
	}
	p.healthy.Store(true)
	return p
}

func (p *AnthropicProvider) Name() string  { return p.name }
func (p *AnthropicProvider) Healthy() bool { return p.healthy.Load() }

func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	aReq := p.translateRequest(req)

	body, err := json.Marshal(aReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	p.healthy.Store(true)

	var aResp anthropicResponse
	if err := json.Unmarshal(respBody, &aResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return p.translateResponse(&aResp), nil
}

func (p *AnthropicProvider) translateRequest(req *openai.ChatCompletionRequest) *anthropicRequest {
	aReq := &anthropicRequest{
		Model:     req.Model,
		MaxTokens: 4096,
	}

	if aReq.Model == "" && len(p.models) > 0 {
		aReq.Model = p.models[0].Name
	}

	if req.MaxTokens != nil {
		aReq.MaxTokens = *req.MaxTokens
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			aReq.System = msg.Content
			continue
		}
		aReq.Messages = append(aReq.Messages, anthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return aReq
}

func (p *AnthropicProvider) translateResponse(aResp *anthropicResponse) *openai.ChatCompletionResponse {
	content := ""
	if len(aResp.Content) > 0 {
		content = aResp.Content[0].Text
	}

	finishReason := "stop"
	if aResp.StopReason == "max_tokens" {
		finishReason = "length"
	}

	return &openai.ChatCompletionResponse{
		ID:      aResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   aResp.Model,
		Choices: []openai.Choice{
			{
				Index:        0,
				Message:      openai.Message{Role: "assistant", Content: content},
				FinishReason: finishReason,
			},
		},
		Usage: openai.Usage{
			PromptTokens:     aResp.Usage.InputTokens,
			CompletionTokens: aResp.Usage.OutputTokens,
			TotalTokens:      aResp.Usage.InputTokens + aResp.Usage.OutputTokens,
		},
	}
}

// ChatCompletionStream is not yet implemented for Anthropic (requires SSE translation).
func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest) (*http.Response, error) {
	return nil, fmt.Errorf("streaming not yet supported for anthropic provider")
}
