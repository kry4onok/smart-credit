package content

import (
	"context"
	"mime"
	"net/http"
	"strconv"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func (h *Handler) Handle(ctx context.Context, request generated.GetInternalApplicationDocumentContentRequestObject) (generated.GetInternalApplicationDocumentContentResponseObject, error) {
	if _, ok := middleware.Staff(ctx); !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия сотрудника банка.")}, nil
	}
	if request.ApplicationId == uuid.Nil || request.DocumentId == uuid.Nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_DOCUMENT_REQUEST", "ID заявки и документа должны быть ненулевыми UUID.")}, nil
	}
	document, err := h.usecase.Execute(ctx, model.GetDocumentContentParams{
		ApplicationID: request.ApplicationId, DocumentID: request.DocumentId,
	})
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return documentResponse{document}, nil
}

type documentResponse struct{ model.DocumentContent }

func (r documentResponse) VisitGetInternalApplicationDocumentContentResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", r.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(r.Content)))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": r.FileName}))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.Content)
	return err
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitGetInternalApplicationDocumentContentResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
