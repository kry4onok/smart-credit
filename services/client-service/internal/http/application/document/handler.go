package document

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	documentusecase "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/document"
	"github.com/google/uuid"
)

func (h *Handler) Handle(ctx context.Context, request generated.AddApplicationDocumentRequestObject) (generated.AddApplicationDocumentResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	if request.ApplicationId == uuid.Nil || request.Params.IdempotencyKey == uuid.Nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_DOCUMENT_REQUEST", "ID заявки и Idempotency-Key должны быть ненулевыми UUID.")}, nil
	}
	if request.Body == nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnprocessableEntity, "INVALID_DOCUMENT", "Необходимо передать multipart-файл.")}, nil
	}
	upload, err := parseMultipart(request.Body)
	if err != nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnprocessableEntity, "INVALID_DOCUMENT", err.Error())}, nil
	}
	document, err := h.usecase.Execute(ctx, model.AddApplicationDocumentParams{
		OrgID: client.OrgID, UserID: client.UserID, ApplicationID: request.ApplicationId,
		IdempotencyKey: request.Params.IdempotencyKey, DocumentType: upload.DocumentType,
		FileName: upload.FileName, Content: upload.Content,
	})
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return generated.AddApplicationDocument201JSONResponse{
		Id: common.Id(document.ID), DocumentType: common.DocumentType(document.DocumentType),
		FileName: document.FileName, Sha256: document.Sha256, IsMock: document.IsMock,
		UploadedAt: document.UploadedAt,
	}, nil
}

type upload struct {
	DocumentType string
	FileName     string
	Content      []byte
}

func parseMultipart(reader *multipart.Reader) (upload, error) {
	var result upload
	var documentTypeSeen, fileSeen bool
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return upload{}, errors.New("не удалось прочитать multipart-файл")
		}
		fieldName := part.FormName()
		switch fieldName {
		case "document_type":
			if documentTypeSeen {
				return upload{}, errors.New("поле document_type передано несколько раз")
			}
			value, err := io.ReadAll(io.LimitReader(part, 65))
			if err != nil || len(value) == 0 || len(value) > 64 {
				return upload{}, errors.New("поле document_type заполнено некорректно")
			}
			result.DocumentType = string(value)
			documentTypeSeen = true
		case "file":
			if fileSeen {
				return upload{}, errors.New("поле file передано несколько раз")
			}
			result.FileName = sanitizeFileName(part.FileName())
			if result.FileName == "" {
				return upload{}, errors.New("у файла отсутствует имя")
			}
			content, err := io.ReadAll(io.LimitReader(part, documentusecase.MaxDocumentSize+1))
			if err != nil {
				return upload{}, errors.New("не удалось прочитать файл")
			}
			if len(content) > documentusecase.MaxDocumentSize {
				return upload{}, errors.New("размер файла не должен превышать 5 MiB")
			}
			if len(content) == 0 {
				return upload{}, errors.New("файл пустой; выберите документ с содержимым")
			}
			result.Content = content
			fileSeen = true
		default:
			return upload{}, errors.New("multipart содержит неизвестное поле")
		}
		_ = part.Close()
	}
	if !documentTypeSeen || !fileSeen {
		return upload{}, errors.New("обязательны поля document_type и file")
	}
	return result, nil
}

func sanitizeFileName(value string) string {
	value = filepath.Base(strings.ReplaceAll(value, `\`, "/"))
	if value == "." || value == ".." || value == string(filepath.Separator) || strings.TrimSpace(value) == "" || len(value) > 256 {
		return ""
	}
	return value
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitAddApplicationDocumentResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
