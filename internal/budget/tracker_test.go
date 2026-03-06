package budget

import (
	"strings"
	"testing"
	"time"
)

func TestDailyBudgetKey_Format(t *testing.T) {
	key := dailyBudgetKey("tenant-acme")

	if !strings.HasPrefix(key, "flowgate:budget:tenant-acme:daily:") {
		t.Errorf("unexpected key prefix: %s", key)
	}

	// Should contain today's date
	today := time.Now().UTC().Format("2006-01-02")
	if !strings.Contains(key, today) {
		t.Errorf("key should contain today's date %s, got %s", today, key)
	}
}

func TestRateLimitKey_Format(t *testing.T) {
	key := rateLimitKey("tenant-acme")

	if !strings.HasPrefix(key, "flowgate:ratelimit:tenant-acme:") {
		t.Errorf("unexpected key prefix: %s", key)
	}

	// Should contain current minute
	minute := time.Now().UTC().Format("2006-01-02T15:04")
	if !strings.Contains(key, minute) {
		t.Errorf("key should contain current minute %s, got %s", minute, key)
	}
}

func TestTimeUntilEndOfDay(t *testing.T) {
	d := timeUntilEndOfDay()

	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
	if d > 24*time.Hour {
		t.Errorf("expected duration <= 24h, got %v", d)
	}
}
