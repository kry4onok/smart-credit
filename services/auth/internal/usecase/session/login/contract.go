package login

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

type Users interface {
	FindByEmail(context.Context, string) (model.User, error)
	LockByID(context.Context, uuid.UUID) (model.User, error)
}
type Sessions interface {
	Save(context.Context, model.RefreshSession) error
}
type Passwords interface {
	Verify(string, string) (bool, error)
}
type AccessTokens interface {
	Issue(model.Principal) (model.AccessToken, error)
}
type RefreshTokens interface {
	Generate() (model.RefreshSecret, error)
}
type Transactions interface {
	Within(context.Context, func(context.Context) error) error
}

type Params struct {
	Users             Users
	Sessions          Sessions
	Passwords         Passwords
	AccessTokens      AccessTokens
	RefreshTokens     RefreshTokens
	Transactions      Transactions
	DummyPasswordHash string
}
