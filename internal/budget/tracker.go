package budget

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Tracker manages per-tenant token budgets and rate limits in Redis.
type Tracker struct {
	rdb *redis.Client
}

func NewTracker(rdb *redis.Client) *Tracker {
	return &Tracker{rdb: rdb}
}

// CheckTokenBudget returns remaining daily tokens and whether the tenant is within budget.
func (t *Tracker) CheckTokenBudget(ctx context.Context, tenantID string, dailyLimit int64) (remaining int64, ok bool, err error) {
	if dailyLimit <= 0 {
		return 0, true, nil // unlimited
	}

	key := dailyBudgetKey(tenantID)
	used, err := t.rdb.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return 0, false, fmt.Errorf("checking budget: %w", err)
	}

	remaining = dailyLimit - used
	return remaining, remaining > 0, nil
}

// ConsumeTokens records token usage for a tenant.
func (t *Tracker) ConsumeTokens(ctx context.Context, tenantID string, tokens int) error {
	key := dailyBudgetKey(tenantID)

	pipe := t.rdb.Pipeline()
	pipe.IncrBy(ctx, key, int64(tokens))
	// Set expiry to end of day if not already set
	pipe.Expire(ctx, key, timeUntilEndOfDay())
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("consuming tokens: %w", err)
	}
	return nil
}

// CheckRateLimit returns whether the tenant is within their requests-per-minute limit.
func (t *Tracker) CheckRateLimit(ctx context.Context, tenantID string, rpm int) (ok bool, err error) {
	if rpm <= 0 {
		return true, nil // unlimited
	}

	key := rateLimitKey(tenantID)

	count, err := t.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("checking rate limit: %w", err)
	}

	// Set 1-minute expiry on first request in the window
	if count == 1 {
		t.rdb.Expire(ctx, key, time.Minute)
	}

	return count <= int64(rpm), nil
}

func dailyBudgetKey(tenantID string) string {
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("flowgate:budget:%s:daily:%s", tenantID, date)
}

func rateLimitKey(tenantID string) string {
	minute := time.Now().UTC().Format("2006-01-02T15:04")
	return fmt.Sprintf("flowgate:ratelimit:%s:%s", tenantID, minute)
}

func timeUntilEndOfDay() time.Duration {
	now := time.Now().UTC()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return endOfDay.Sub(now)
}
