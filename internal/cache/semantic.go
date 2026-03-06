package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

const (
	vectorSetKey    = "flowgate:cache:vectors"
	responseKeyPfx  = "flowgate:cache:resp:"
)

// SemanticCache provides similarity-based caching using Redis Vector Sets.
type SemanticCache struct {
	rdb       *redis.Client
	embedder  *Embedder
	threshold float64
	ttl       time.Duration
}

func NewSemanticCache(rdb *redis.Client, embedder *Embedder, threshold float64, ttl time.Duration) *SemanticCache {
	return &SemanticCache{
		rdb:       rdb,
		embedder:  embedder,
		threshold: threshold,
		ttl:       ttl,
	}
}

// Lookup checks if a semantically similar request has been cached.
func (c *SemanticCache) Lookup(ctx context.Context, messages []openai.Message) (*openai.ChatCompletionResponse, bool, error) {
	embedding, err := c.embedder.Embed(ctx, messages)
	if err != nil {
		return nil, false, fmt.Errorf("generating embedding: %w", err)
	}

	// VSIM: find nearest neighbor in the vector set
	vecStr := formatVector(embedding)
	results, err := c.rdb.Do(ctx, "VSIM", vectorSetKey, "VALUES", len(embedding), vecStr, "COUNT", 1, "WITHSCORES").StringSlice()
	if err != nil {
		if err == redis.Nil || strings.Contains(err.Error(), "ERR") {
			return nil, false, nil // no results or vector set doesn't exist yet
		}
		return nil, false, fmt.Errorf("VSIM query: %w", err)
	}

	if len(results) < 2 {
		return nil, false, nil
	}

	// results = [element_key, similarity_score]
	elementKey := results[0]
	similarity, err := strconv.ParseFloat(results[1], 64)
	if err != nil {
		return nil, false, fmt.Errorf("parsing similarity score: %w", err)
	}

	slog.Debug("cache similarity", "key", elementKey, "similarity", similarity, "threshold", c.threshold)

	if similarity < c.threshold {
		return nil, false, nil
	}

	// Fetch the cached response
	data, err := c.rdb.Get(ctx, responseKeyPfx+elementKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil // response expired
		}
		return nil, false, fmt.Errorf("fetching cached response: %w", err)
	}

	var resp openai.ChatCompletionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false, fmt.Errorf("decoding cached response: %w", err)
	}

	return &resp, true, nil
}

// Store caches a response with its embedding in the vector set.
func (c *SemanticCache) Store(ctx context.Context, messages []openai.Message, resp *openai.ChatCompletionResponse) error {
	embedding, err := c.embedder.Embed(ctx, messages)
	if err != nil {
		return fmt.Errorf("generating embedding: %w", err)
	}

	key := hashMessages(messages)

	// Store response payload with TTL
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}

	if err := c.rdb.Set(ctx, responseKeyPfx+key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("storing response: %w", err)
	}

	// VADD: add embedding to vector set
	vecStr := formatVector(embedding)
	if err := c.rdb.Do(ctx, "VADD", vectorSetKey, "VALUES", len(embedding), vecStr, key).Err(); err != nil {
		return fmt.Errorf("VADD: %w", err)
	}

	return nil
}

func hashMessages(messages []openai.Message) string {
	h := sha256.New()
	for _, m := range messages {
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func formatVector(vec []float64) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strings.Join(parts, " ")
}
