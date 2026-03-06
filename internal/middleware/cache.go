package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/cache"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

const CachedResponseKey contextKey = "cached_response"

type cacheResponseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *cacheResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// SemanticCacheMiddleware checks cache before handler, stores after.
func SemanticCacheMiddleware(exactCache *cache.ExactCache, semanticCache *cache.SemanticCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only cache chat completion requests
			if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
				next.ServeHTTP(w, r)
				return
			}

			// Read and buffer the body so we can parse messages and still pass to handler
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			var req openai.ChatCompletionRequest
			if err := json.Unmarshal(bodyBytes, &req); err != nil || req.Stream {
				// Can't parse or streaming — skip cache
				next.ServeHTTP(w, r)
				return
			}

			// Fast path: exact match
			if resp, hit, err := exactCache.Lookup(r.Context(), req.Messages); err == nil && hit {
				slog.Debug("exact cache hit")
				w.Header().Set("X-FlowGate-Cache", "hit")
				w.Header().Set("X-FlowGate-Cache-Type", "exact")
				writeJSONResponse(w, resp)
				return
			}

			// Semantic similarity lookup
			if resp, hit, err := semanticCache.Lookup(r.Context(), req.Messages); err == nil && hit {
				slog.Debug("semantic cache hit")
				w.Header().Set("X-FlowGate-Cache", "hit")
				w.Header().Set("X-FlowGate-Cache-Type", "semantic")
				writeJSONResponse(w, resp)
				return
			}

			// Cache miss — pass to handler, capture response for storage
			w.Header().Set("X-FlowGate-Cache", "miss")

			crw := &cacheResponseWriter{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				statusCode:     http.StatusOK,
			}

			// Store messages in context for post-handler cache storage
			ctx := context.WithValue(r.Context(), CachedResponseKey, req.Messages)
			next.ServeHTTP(crw, r.WithContext(ctx))

			// Only cache successful responses
			if crw.statusCode == http.StatusOK {
				go func() {
					var resp openai.ChatCompletionResponse
					if err := json.Unmarshal(crw.body.Bytes(), &resp); err != nil {
						slog.Error("failed to parse response for caching", "error", err)
						return
					}

					bgCtx := context.Background()
					if err := exactCache.Store(bgCtx, req.Messages, &resp); err != nil {
						slog.Error("failed to store exact cache", "error", err)
					}
					if err := semanticCache.Store(bgCtx, req.Messages, &resp); err != nil {
						slog.Error("failed to store semantic cache", "error", err)
					}
				}()
			}
		})
	}
}

func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v)
}
