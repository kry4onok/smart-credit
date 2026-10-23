package middleware

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type verifierMock struct {
	principal model.Principal
	err       error
}

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, m.err
}
