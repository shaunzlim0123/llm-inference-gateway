package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

const exactCacheKeyPfx = "flowgate:cache:exact:"

// ExactCache provides hash-based exact-match caching as a fast path
// before falling through to the more expensive semantic similarity search.
type ExactCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewExactCache(rdb *redis.Client, ttl time.Duration) *ExactCache {
	return &ExactCache{rdb: rdb, ttl: ttl}
}

func (c *ExactCache) Lookup(ctx context.Context, messages []openai.Message) (*openai.ChatCompletionResponse, bool, error) {
	key := exactCacheKeyPfx + hashMessages(messages)

	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("exact cache lookup: %w", err)
	}

	var resp openai.ChatCompletionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false, fmt.Errorf("decoding cached response: %w", err)
	}

	return &resp, true, nil
}

func (c *ExactCache) Store(ctx context.Context, messages []openai.Message, resp *openai.ChatCompletionResponse) error {
	key := exactCacheKeyPfx + hashMessages(messages)

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}

	return c.rdb.Set(ctx, key, data, c.ttl).Err()
}
