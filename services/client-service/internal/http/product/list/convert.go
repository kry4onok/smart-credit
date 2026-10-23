package list

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func toResponse(catalog model.ProductCatalog) generated.ListClientProducts200JSONResponse {
	items := make([]common.Product, 0, len(catalog.Items))
	for _, product := range catalog.Items {
		items = append(items, common.Product{
			Id:                    common.Id(product.ID),
			Version:               product.Version,
			Code:                  product.Code,
			Name:                  product.Name,
			Description:           product.Description,
			PurposeTags:           purposeTags(product.PurposeTags),
			MinAmount:             common.Money{AmountKopecks: product.MinAmountKopecks, Currency: common.MoneyCurrencyRUB},
			MaxAmount:             common.Money{AmountKopecks: product.MaxAmountKopecks, Currency: common.MoneyCurrencyRUB},
			MinTermMonths:         product.MinTermMonths,
			MaxTermMonths:         product.MaxTermMonths,
			IndicativeRateBps:     product.IndicativeRateBps,
			RequiredDocumentTypes: documentTypes(product.RequiredDocumentTypes),
		})
	}
	return generated.ListClientProducts200JSONResponse{CatalogVersion: catalog.Version, Items: items}
}

func purposeTags(values []string) []common.PurposeTag {
	result := make([]common.PurposeTag, len(values))
	for i, value := range values {
		result[i] = common.PurposeTag(value)
	}
	return result
}

func documentTypes(values []string) []common.DocumentType {
	result := make([]common.DocumentType, len(values))
	for i, value := range values {
		result[i] = common.DocumentType(value)
	}
	return result
}
