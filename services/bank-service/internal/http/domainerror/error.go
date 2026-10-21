package domainerror

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/google/uuid"
)

type Error struct {
	Status int
	Code   string
	Title  string
	Detail string
	cause  error
}

func (e *Error) Error() string {
	return e.Detail
}

func (e *Error) Unwrap() error {
	return e.cause
}

func BadRequest(code string, cause error) *Error {
	return &Error{
		Status: http.StatusBadRequest,
		Code:   code,
		Title:  "Некорректный запрос",
		Detail: cause.Error(),
		cause:  cause,
	}
}

func Write(w http.ResponseWriter, r *http.Request, err error) {
	problem := common.Problem{
		Type:   "about:blank",
		Title:  "Внутренняя ошибка",
		Status: http.StatusInternalServerError,
		Code:   "INTERNAL_ERROR",
	}

	var domainErr *Error
	if errors.As(err, &domainErr) {
		problem.Title = domainErr.Title
		problem.Status = domainErr.Status
		problem.Code = domainErr.Code
		problem.Detail = &domainErr.Detail
	}

	traceID := r.Header.Get("X-Request-ID")
	if traceID == "" {
		traceID = uuid.NewString()
	}
	problem.TraceId = traceID

	var body bytes.Buffer
	if encodeErr := json.NewEncoder(&body).Encode(problem); encodeErr != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("X-Request-ID", traceID)
	w.WriteHeader(problem.Status)
	_, _ = body.WriteTo(w)
}
