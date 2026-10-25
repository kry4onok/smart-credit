package listproducts

import (
	"context"
	"fmt"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/google/uuid"
)

type Repository interface {
	ListActive(context.Context) ([]*commonpb.Product, error)
}

type Handler struct {
	repository Repository
}

func New(repository Repository) *Handler {
	return &Handler{repository: repository}
}

func (h *Handler) Handle(ctx context.Context, _ generated.ListBankProductsRequestObject) (generated.ListBankProductsResponseObject, error) {
	products, err := h.repository.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list bank products: %w", err)
	}
	items := make([]common.Product, 0, len(products))
	for _, product := range products {
		id, err := uuid.Parse(product.GetProductId())
		if err != nil {
			return nil, fmt.Errorf("invalid bank product id: %w", err)
		}
		purposeTags := make([]common.PurposeTag, 0, len(product.GetPurposeTags()))
		for _, tag := range product.GetPurposeTags() {
			purposeTags = append(purposeTags, common.PurposeTag(strings.TrimPrefix(tag.String(), "PURPOSE_TAG_")))
		}
		documentTypes := make([]common.DocumentType, 0, len(product.GetRequiredDocumentTypes()))
		for _, documentType := range product.GetRequiredDocumentTypes() {
			documentTypes = append(documentTypes, common.DocumentType(strings.TrimPrefix(documentType.String(), "DOCUMENT_TYPE_")))
		}
		rate := int(product.GetIndicativeRateBps())
		items = append(items, common.Product{
			Id: id, Version: product.GetVersion(), Code: product.GetCode(), Name: product.GetName(),
			Description: product.GetDescription(), PurposeTags: purposeTags,
			RequiredDocumentTypes: documentTypes,
			MinAmount:             common.Money{AmountKopecks: product.GetMinAmount().GetAmountKopecks(), Currency: common.MoneyCurrencyRUB},
			MaxAmount:             common.Money{AmountKopecks: product.GetMaxAmount().GetAmountKopecks(), Currency: common.MoneyCurrencyRUB},
			MinTermMonths:         int(product.GetMinTermMonths()), MaxTermMonths: int(product.GetMaxTermMonths()),
			IndicativeRateBps: &rate,
		})
	}
	return generated.ListBankProducts200JSONResponse{Items: items}, nil
}
