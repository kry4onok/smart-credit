package document

import (
	"context"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	AddDocument(context.Context, model.AddApplicationDocumentParams) (model.ApplicationDocument, error)
}

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, params model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
	if params.ApplicationID == uuid.Nil || params.OrgID == uuid.Nil || params.UserID == uuid.Nil || params.IdempotencyKey == uuid.Nil {
		return model.ApplicationDocument{}, domain.ErrInvalidDocumentUpload
	}
	if !validDocumentType(params.DocumentType) || strings.TrimSpace(params.FileName) == "" || len(params.Content) > MaxDocumentSize {
		return model.ApplicationDocument{}, domain.ErrInvalidDocumentUpload
	}
	return u.repository.AddDocument(ctx, params)
}
