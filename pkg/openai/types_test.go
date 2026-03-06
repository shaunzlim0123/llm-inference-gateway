package openai

import (
	"encoding/json"
	"testing"
)

func TestChatCompletionRequest_JSON(t *testing.T) {
	input := `{
		"model": "gpt-4o-mini",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello"}
		],
		"temperature": 0.7,
		"stream": false
	}`

	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if req.Model != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %s", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got %s", req.Messages[0].Role)
	}
	if req.Messages[1].Content != "Hello" {
		t.Errorf("expected second message content 'Hello', got %s", req.Messages[1].Content)
	}
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7")
	}
	if req.Stream {
		t.Error("expected stream false")
	}
}

func TestChatCompletionRequest_OmitsEmpty(t *testing.T) {
	req := ChatCompletionRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	// model should be omitted when empty
	if _, ok := raw["model"]; ok {
		t.Error("expected model to be omitted when empty")
	}
	// temperature should be omitted when nil
	if _, ok := raw["temperature"]; ok {
		t.Error("expected temperature to be omitted when nil")
	}
}

func TestChatCompletionResponse_JSON(t *testing.T) {
	resp := ChatCompletionResponse{
		ID:      "chatcmpl-abc",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "gpt-4o-mini",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "Hello!"},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ChatCompletionResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != "chatcmpl-abc" {
		t.Errorf("expected ID chatcmpl-abc, got %s", decoded.ID)
	}
	if decoded.Usage.TotalTokens != 15 {
		t.Errorf("expected total_tokens 15, got %d", decoded.Usage.TotalTokens)
	}
	if decoded.Choices[0].Message.Content != "Hello!" {
		t.Errorf("expected content 'Hello!', got %s", decoded.Choices[0].Message.Content)
	}
}

func TestErrorResponse_JSON(t *testing.T) {
	errResp := ErrorResponse{
		Error: ErrorDetail{
			Message: "invalid API key",
			Type:    "authentication_error",
		},
	}

	data, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ErrorResponse
	json.Unmarshal(data, &decoded)

	if decoded.Error.Message != "invalid API key" {
		t.Errorf("expected message 'invalid API key', got %s", decoded.Error.Message)
	}
	if decoded.Error.Code != nil {
		t.Error("expected code to be nil")
	}
}

func TestChatCompletionChunk_JSON(t *testing.T) {
	input := `{
		"id": "chatcmpl-abc",
		"object": "chat.completion.chunk",
		"created": 1700000000,
		"model": "gpt-4o-mini",
		"choices": [
			{
				"index": 0,
				"delta": {"role": "assistant", "content": "Hi"},
				"finish_reason": null
			}
		]
	}`

	var chunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(input), &chunk); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if chunk.ID != "chatcmpl-abc" {
		t.Errorf("expected ID chatcmpl-abc, got %s", chunk.ID)
	}
	if chunk.Choices[0].Delta.Content != "Hi" {
		t.Errorf("expected delta content 'Hi', got %s", chunk.Choices[0].Delta.Content)
	}
	if chunk.Choices[0].FinishReason != nil {
		t.Error("expected finish_reason to be nil")
	}
}
