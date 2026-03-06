package router

import (
	"strings"

	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// complexKeywords increase the score when found in messages.
var complexKeywords = map[string]int{
	"analyze":     10,
	"compare":     8,
	"explain":     6,
	"architecture": 10,
	"implement":   8,
	"design":      8,
	"optimize":    8,
	"refactor":    8,
	"debug":       6,
	"review":      6,
	"evaluate":    6,
	"synthesize":  8,
	"critique":    6,
	"recommend":   4,
	"summarize":   4,
	"translate":   4,
	"write":       4,
	"code":        4,
	"algorithm":   8,
	"tradeoff":    6,
	"trade-off":   6,
	"pros and cons": 6,
}

// simpleKeywords decrease the effective complexity.
var simpleKeywords = map[string]int{
	"hello":    -5,
	"hi":       -5,
	"hey":      -5,
	"thanks":   -5,
	"thank you": -5,
	"yes":      -5,
	"no":       -5,
	"ok":       -5,
}

// ScoreComplexity computes a heuristic complexity score for a chat request.
// Higher scores indicate more complex queries.
func ScoreComplexity(messages []openai.Message) int {
	score := 0

	// Factor 1: Message count (more turns = more complex context)
	score += len(messages) * 3

	// Factor 2: Total content length as rough token proxy (~4 chars per token)
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	estimatedTokens := totalChars / 4
	switch {
	case estimatedTokens > 2000:
		score += 30
	case estimatedTokens > 500:
		score += 15
	case estimatedTokens > 100:
		score += 5
	}

	// Factor 3: Keyword analysis on user messages
	for _, m := range messages {
		if m.Role != "user" {
			continue
		}
		lower := strings.ToLower(m.Content)

		for kw, weight := range complexKeywords {
			if strings.Contains(lower, kw) {
				score += weight
			}
		}
		for kw, weight := range simpleKeywords {
			if strings.Contains(lower, kw) {
				score += weight // weight is negative
			}
		}
	}

	// Factor 4: Presence of code blocks suggests technical complexity
	for _, m := range messages {
		if strings.Contains(m.Content, "```") {
			score += 10
			break
		}
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}
