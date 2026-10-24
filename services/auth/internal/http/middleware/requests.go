package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/domainerror"
	"github.com/go-chi/chi/v5"
)

func Requests(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &responseWriter{ResponseWriter: w}
			start := time.Now()
			defer func() {
				status := response.status
				if status == 0 {
					status = http.StatusOK
				}
				route := "<unmatched>"
				if ctx := chi.RouteContext(r.Context()); ctx != nil && ctx.RoutePattern() != "" {
					route = ctx.RoutePattern()
				}
				logger.InfoContext(r.Context(), "auth request", "method", r.Method, "route", route,
					"status", status, "duration_ms", time.Since(start).Milliseconds(), "trace_id", domainerror.TraceID(r.Context()))
			}()
			next.ServeHTTP(response, r)
		})
	}
}
