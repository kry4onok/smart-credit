package refresh

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type Sessions interface {
	Lock(context.Context, [32]byte) (model.SessionState, error)
}
type AccessTokens interface {
	Issue(model.Principal) (model.AccessToken, error)
}
type RefreshTokens interface {
	Hash(string) ([32]byte, error)
}
type Transactions interface {
	Within(context.Context, func(context.Context) error) error
}
type Params struct {
	Sessions      Sessions
	AccessTokens  AccessTokens
	RefreshTokens RefreshTokens
	Transactions  Transactions
}
