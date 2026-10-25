package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) ListBankProducts(ctx context.Context, request generated.ListBankProductsRequestObject) (generated.ListBankProductsResponseObject, error) {
	return s.listProducts.Handle(ctx, request)
}
