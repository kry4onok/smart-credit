package refresh

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type Usecase interface {
	Execute(context.Context, string) (model.AccessToken, error)
}
