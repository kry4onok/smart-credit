package refresh

import (
	"context"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type Usecase struct {
	params Params
	now    func() time.Time
}

func New(params Params) *Usecase { return &Usecase{params: params, now: time.Now} }

func (u *Usecase) Execute(ctx context.Context, rawToken string) (model.AccessToken, error) {
	hash, err := u.params.RefreshTokens.Hash(rawToken)
	if err != nil {
		return model.AccessToken{}, domain.ErrUnauthorized
	}
	var result model.AccessToken
	err = u.params.Transactions.Within(ctx, func(ctx context.Context) error {
		session, err := u.params.Sessions.Lock(ctx, hash)
		if err != nil {
			return err
		}
		now := u.now()
		if !session.Active || session.RevokedAt != nil || !session.ExpiresAt.After(now) || session.CreatedAt.After(now) {
			return domain.ErrUnauthorized
		}
		result, err = u.params.AccessTokens.Issue(session.Principal)
		return err
	})
	if err != nil {
		return model.AccessToken{}, err
	}
	return result, nil
}
