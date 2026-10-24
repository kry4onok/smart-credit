package domainerror

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/google/uuid"
)

type traceKey struct{}

func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set("X-Trace-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), traceKey{}, id)))
	})
}

func Problem(ctx context.Context, status int, code, detail string) common.Problem {
	id, _ := ctx.Value(traceKey{}).(string)
	if id == "" {
		id = uuid.NewString()
	}
	return common.Problem{Type: "about:blank", Title: http.StatusText(status), Status: status,
		Code: code, Detail: &detail, TraceId: id}
}

func Write(w http.ResponseWriter, p common.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

func Map(ctx context.Context, err error) common.Problem {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Нет действительной сессии.")
	case errors.Is(err, domain.ErrInvalidRequest):
		return Problem(ctx, http.StatusBadRequest, "BAD_REQUEST", "Некорректный запрос.")
	default:
		return Problem(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось обработать запрос.")
	}
}
