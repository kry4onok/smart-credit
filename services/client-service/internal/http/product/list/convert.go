package list

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	producthttp "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func toResponse(catalog model.ProductCatalog) generated.ListClientProducts200JSONResponse {
	items := make([]common.Product, 0, len(catalog.Items))
	for _, product := range catalog.Items {
		items = append(items, producthttp.ToAPI(product))
	}
	return generated.ListClientProducts200JSONResponse{CatalogVersion: catalog.Version, Items: items}
}
