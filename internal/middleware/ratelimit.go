package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/budget"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// RateLimit checks per-tenant request rate and token budget.
func RateLimit(tracker *budget.Tracker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := GetTenant(r.Context())
			if tenant == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check request rate limit
			if tenant.RateLimit.RequestsPerMinute > 0 {
				ok, err := tracker.CheckRateLimit(r.Context(), tenant.ID, tenant.RateLimit.RequestsPerMinute)
				if err != nil {
					slog.Error("rate limit check failed", "error", err, "tenant", tenant.ID)
					// Fail open: allow request on error
				} else if !ok {
					writeRateLimitError(w, "rate limit exceeded: too many requests per minute")
					return
				}
			}

			// Check token budget
			if tenant.TokenBudget.DailyLimit > 0 {
				remaining, ok, err := tracker.CheckTokenBudget(r.Context(), tenant.ID, tenant.TokenBudget.DailyLimit)
				if err != nil {
					slog.Error("budget check failed", "error", err, "tenant", tenant.ID)
				} else if !ok {
					writeRateLimitError(w, "daily token budget exceeded")
					return
				} else {
					slog.Debug("token budget", "tenant", tenant.ID, "remaining", remaining)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimitError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(openai.ErrorResponse{
		Error: openai.ErrorDetail{
			Message: msg,
			Type:    "rate_limit_error",
		},
	})
}
