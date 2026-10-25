package content

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	GetDocumentContent(context.Context, model.GetDocumentContentParams) (model.DocumentContent, error)
}

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, params model.GetDocumentContentParams) (model.DocumentContent, error) {
	if params.ApplicationID == uuid.Nil || params.DocumentID == uuid.Nil {
		return model.DocumentContent{}, domain.ErrDocumentNotFound
	}
	document, err := u.repository.GetDocumentContent(ctx, params)
	if err != nil {
		return model.DocumentContent{}, err
	}
	document.ContentType = contentType(document.FileName, document.Content)
	return document, nil
}

func contentType(name string, content []byte) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return http.DetectContentType(content)
	}
}
