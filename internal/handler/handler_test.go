package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/provider"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/router"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// mockProvider implements the provider.Provider interface for testing.
type mockProvider struct {
	name     string
	healthy  bool
	response *openai.ChatCompletionResponse
	err      error
}

func (m *mockProvider) Name() string  { return m.name }
func (m *mockProvider) Healthy() bool { return m.healthy }
func (m *mockProvider) ChatCompletion(_ context.Context, _ *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	return m.response, m.err
}
func (m *mockProvider) ChatCompletionStream(_ context.Context, _ *openai.ChatCompletionRequest) (*http.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func newTestHandler(providers ...provider.Provider) *ChatCompletionHandler {
	reg := provider.NewRegistry()
	fallback := make([]string, 0, len(providers))
	providerConfigs := make(map[string]config.ProviderConfig, len(providers))
	for _, p := range providers {
		reg.Register(p)
		fallback = append(fallback, p.Name())
		providerConfigs[p.Name()] = config.ProviderConfig{
			Models: []config.ModelConfig{
				{Name: "test-model", Complexity: "simple"},
			},
		}
	}

	sel := router.NewSelector(config.RoutingConfig{
		Strategy:      "complexity",
		Complexity:    config.ComplexityConfig{SimpleMaxScore: 30, MediumMaxScore: 70},
		FallbackOrder: fallback,
	}, providerConfigs)

	return NewChatCompletionHandler(reg, sel, fallback)
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)

	Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %s", body["status"])
	}
}

func TestReady(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ready", nil)

	Ready(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ready" {
		t.Errorf("expected status 'ready', got %s", body["status"])
	}
}

func TestChatCompletion_InvalidJSON(t *testing.T) {
	h := newTestHandler(&mockProvider{name: "test", healthy: true})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("not json"))

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestChatCompletion_EmptyMessages(t *testing.T) {
	h := newTestHandler(&mockProvider{name: "test", healthy: true})

	rec := httptest.NewRecorder()
	body := `{"messages":[]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var errResp openai.ErrorResponse
	json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error.Type != "invalid_request_error" {
		t.Errorf("expected invalid_request_error, got %s", errResp.Error.Type)
	}
}

func TestChatCompletion_Success(t *testing.T) {
	h := newTestHandler(&mockProvider{
		name:    "test",
		healthy: true,
		response: &openai.ChatCompletionResponse{
			ID:    "chatcmpl-123",
			Model: "test-model",
			Choices: []openai.Choice{
				{
					Index:        0,
					Message:      openai.Message{Role: "assistant", Content: "Hello!"},
					FinishReason: "stop",
				},
			},
		},
	})

	rec := httptest.NewRecorder()
	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("X-FlowGate-Provider") != "test" {
		t.Errorf("expected X-FlowGate-Provider 'test', got %s", rec.Header().Get("X-FlowGate-Provider"))
	}

	var resp openai.ChatCompletionResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ID != "chatcmpl-123" {
		t.Errorf("expected chatcmpl-123, got %s", resp.ID)
	}
}

func TestChatCompletion_AllProvidersFail(t *testing.T) {
	h := newTestHandler(&mockProvider{
		name:    "test",
		healthy: true,
		err:     fmt.Errorf("upstream error"),
	})

	rec := httptest.NewRecorder()
	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rec.Code)
	}
}

func TestChatCompletion_SkipsUnhealthy(t *testing.T) {
	h := newTestHandler(
		&mockProvider{
			name:    "unhealthy",
			healthy: false,
			response: &openai.ChatCompletionResponse{
				ID: "should-not-reach",
			},
		},
		&mockProvider{
			name:    "healthy",
			healthy: true,
			response: &openai.ChatCompletionResponse{
				ID:    "from-healthy",
				Model: "test-model",
			},
		},
	)

	rec := httptest.NewRecorder()
	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp openai.ChatCompletionResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ID != "from-healthy" {
		t.Errorf("expected from-healthy, got %s", resp.ID)
	}
}

func TestWriteError_Format(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusNotFound, "not found", "not_found_error")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var errResp openai.ErrorResponse
	json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error.Message != "not found" {
		t.Errorf("expected 'not found', got %s", errResp.Error.Message)
	}
	if errResp.Error.Type != "not_found_error" {
		t.Errorf("expected 'not_found_error', got %s", errResp.Error.Type)
	}
}
