package domainerror

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
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

func Map(ctx context.Context, err error) common.Problem {
	switch {
	case errors.Is(err, domain.ErrCompanyNotFound):
		return Problem(ctx, http.StatusNotFound, "COMPANY_NOT_FOUND", "Профиль компании не найден.")
	case errors.Is(err, domain.ErrProductNotFound):
		return Problem(ctx, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Продукт не найден.")
	case errors.Is(err, domain.ErrApplicationNotFound):
		return Problem(ctx, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Заявка не найдена.")
	case errors.Is(err, domain.ErrApplicationVersionConflict):
		return Problem(ctx, http.StatusConflict, "APPLICATION_VERSION_CONFLICT", "Заявка уже изменена; обновите карточку и повторите запрос.")
	case errors.Is(err, domain.ErrApplicationNotDraft):
		return Problem(ctx, http.StatusConflict, "APPLICATION_STATE_CONFLICT", "Редактировать можно только черновик заявки.")
	case errors.Is(err, domain.ErrApplicationDocumentState):
		return Problem(ctx, http.StatusConflict, "APPLICATION_STATE_CONFLICT", "Документ нельзя добавить в текущем состоянии заявки.")
	case errors.Is(err, domain.ErrCompanyAlreadyExists):
		return Problem(ctx, http.StatusConflict, "COMPANY_ALREADY_EXISTS", "Профиль компании уже существует; укажите актуальный If-Match.")
	case errors.Is(err, domain.ErrCompanyVersionConflict):
		return Problem(ctx, http.StatusConflict, "COMPANY_VERSION_CONFLICT", "Профиль компании уже изменён; обновите данные и повторите запрос.")
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return Problem(ctx, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Этот Idempotency-Key уже использован с другим запросом.")
	case errors.Is(err, domain.ErrInvalidCompanyProfile):
		return Problem(ctx, http.StatusUnprocessableEntity, "INVALID_COMPANY_PROFILE", "Профиль компании заполнен некорректно.")
	case errors.Is(err, domain.ErrInvalidApplication):
		return Problem(ctx, http.StatusUnprocessableEntity, "INVALID_APPLICATION", "Заявка не может быть создана.")
	case errors.Is(err, domain.ErrInvalidApplicationPatch):
		return Problem(ctx, http.StatusUnprocessableEntity, "INVALID_APPLICATION_FORM", "Форма заявки заполнена некорректно.")
	case errors.Is(err, domain.ErrApplicationDocumentType):
		return Problem(ctx, http.StatusUnprocessableEntity, "INVALID_DOCUMENT_TYPE", "Тип документа не разрешён для этой заявки.")
	case errors.Is(err, domain.ErrApplicationInfoConflict):
		return Problem(ctx, http.StatusConflict, "APPLICATION_STATE_CONFLICT", "Запрос банка уже обработан или заявка находится в другом состоянии.")
	case errors.Is(err, domain.ErrInvalidInfoResponse):
		return Problem(ctx, http.StatusUnprocessableEntity, "INVALID_INFO_RESPONSE", "Ответ на запрос банка заполнен некорректно.")
	case errors.Is(err, domain.ErrInvalidDocumentUpload):
		return Problem(ctx, http.StatusUnprocessableEntity, "INVALID_DOCUMENT", "Файл документа заполнен некорректно.")
	}
	return Problem(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось обработать запрос.")
}

func Write(w http.ResponseWriter, p common.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
