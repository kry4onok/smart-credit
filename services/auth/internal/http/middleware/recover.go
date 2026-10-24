package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/domainerror"
)

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response, ok := w.(*responseWriter)
			if !ok {
				response = &responseWriter{ResponseWriter: w}
			}
			defer func() {
				if value := recover(); value != nil {
					if err, ok := value.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(http.ErrAbortHandler)
					}
					// Panic values may contain credentials; never log their contents.
					logger.ErrorContext(r.Context(), "auth request panic", "trace_id", domainerror.TraceID(r.Context()))
					if response.status != 0 {
						panic(http.ErrAbortHandler)
					}
					for key := range response.Header() {
						response.Header().Del(key)
					}
					problem := domainerror.Problem(r.Context(), http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось обработать запрос.")
					response.Header().Set("X-Trace-ID", problem.TraceId)
					response.Header().Set("Cache-Control", "no-store")
					domainerror.Write(response, problem)
				}
			}()
			next.ServeHTTP(response, r)
		})
	}
}
