package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/observability"
)

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *metricsResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		mrw := &metricsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(mrw, r)

		tenant := ""
		if t := GetTenant(r.Context()); t != nil {
			tenant = t.ID
		}

		observability.RequestDuration.WithLabelValues(
			r.Method,
			r.URL.Path,
			strconv.Itoa(mrw.statusCode),
			tenant,
		).Observe(time.Since(start).Seconds())
	})
}
