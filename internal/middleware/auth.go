package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

const TenantKey contextKey = "tenant"

func GetTenant(ctx context.Context) *config.TenantConfig {
	if t, ok := ctx.Value(TenantKey).(*config.TenantConfig); ok {
		return t
	}
	return nil
}

// Auth builds an authentication middleware from the tenant list.
// It checks X-API-Key header first, then Authorization: Bearer.
func Auth(tenants []config.TenantConfig) func(http.Handler) http.Handler {
	// Build lookup map: api_key -> tenant
	lookup := make(map[string]*config.TenantConfig, len(tenants))
	for i := range tenants {
		if tenants[i].APIKey != "" {
			lookup[tenants[i].APIKey] = &tenants[i]
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
					key = strings.TrimPrefix(auth, "Bearer ")
				}
			}

			if key == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing API key")
				return
			}

			tenant, ok := lookup[key]
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), TenantKey, tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(openai.ErrorResponse{
		Error: openai.ErrorDetail{
			Message: msg,
			Type:    "authentication_error",
		},
	})
}
