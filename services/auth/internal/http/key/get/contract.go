package get

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type Usecase interface {
	Get(context.Context) ([]model.PublicKey, error)
}
