package list

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type Repository interface {
	List(context.Context, *string) (model.ProductCatalog, error)
}
