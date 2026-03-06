package router

import (
	"testing"

	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

func TestScoreComplexity_Simple(t *testing.T) {
	messages := []openai.Message{
		{Role: "user", Content: "hello"},
	}
	score := ScoreComplexity(messages)
	if score > 30 {
		t.Errorf("expected simple score <= 30, got %d", score)
	}
}

func TestScoreComplexity_Complex(t *testing.T) {
	messages := []openai.Message{
		{Role: "user", Content: "Please analyze and compare these two architecture designs for a distributed system. Evaluate the trade-offs between consistency and availability."},
	}
	score := ScoreComplexity(messages)
	if score < 30 {
		t.Errorf("expected complex score > 30, got %d", score)
	}
}

func TestScoreComplexity_MultiTurn(t *testing.T) {
	messages := []openai.Message{
		{Role: "user", Content: "Explain the algorithm for B-tree insertion."},
		{Role: "assistant", Content: "B-tree insertion works by..."},
		{Role: "user", Content: "Now implement it in Go and optimize for concurrent access."},
	}
	score := ScoreComplexity(messages)
	if score < 20 {
		t.Errorf("expected multi-turn score > 20, got %d", score)
	}
}

func TestScoreComplexity_CodeBlock(t *testing.T) {
	messages := []openai.Message{
		{Role: "user", Content: "Review this code:\n```go\nfunc main() {}\n```"},
	}
	score := ScoreComplexity(messages)
	// Should get bonus for code block + review keyword
	if score < 15 {
		t.Errorf("expected code review score > 15, got %d", score)
	}
}

func TestScoreComplexity_Clamping(t *testing.T) {
	// Very simple greeting should clamp at 0
	messages := []openai.Message{
		{Role: "user", Content: "hi"},
	}
	score := ScoreComplexity(messages)
	if score < 0 || score > 100 {
		t.Errorf("expected score in [0, 100], got %d", score)
	}
}
