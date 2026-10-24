package login

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type Usecase struct {
	params Params
	now    func() time.Time
}

func New(params Params) *Usecase { return &Usecase{params: params, now: time.Now} }

func (u *Usecase) Execute(ctx context.Context, params model.LoginParams) (model.LoginResult, error) {
	user, err := u.params.Users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(params.Email)))
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return model.LoginResult{}, err
	}
	found := err == nil
	hash := user.PasswordHash
	if !found {
		hash = u.params.DummyPasswordHash
	}
	match, verifyErr := u.params.Passwords.Verify(hash, params.Password)
	if verifyErr != nil {
		// A corrupt stored hash must not make this account distinguishable from
		// an unknown account by skipping the expensive password calculation.
		_, _ = u.params.Passwords.Verify(u.params.DummyPasswordHash, params.Password)
	}
	if !found || !user.Active || !match || verifyErr != nil {
		return model.LoginResult{}, domain.ErrUnauthorized
	}
	var result model.LoginResult
	err = u.params.Transactions.Within(ctx, func(ctx context.Context) error {
		current, err := u.params.Users.LockByID(ctx, user.UserID)
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.ErrUnauthorized
		}
		if err != nil {
			return err
		}
		if !current.Active || current.PasswordHash != user.PasswordHash {
			return domain.ErrUnauthorized
		}
		access, err := u.params.AccessTokens.Issue(current.Principal)
		if err != nil {
			return err
		}
		refresh, err := u.params.RefreshTokens.Generate()
		if err != nil {
			return err
		}
		now := u.now().UTC().Truncate(time.Microsecond)
		session := model.RefreshSession{Hash: refresh.Hash, UserID: current.UserID,
			CreatedAt: now, ExpiresAt: now.Add(model.RefreshLifetime)}
		if err := u.params.Sessions.Save(ctx, session); err != nil {
			return err
		}
		result = model.LoginResult{AccessToken: access, RefreshToken: refresh.Token, RefreshExpiresAt: session.ExpiresAt}
		return nil
	})
	if err != nil {
		return model.LoginResult{}, err
	}
	return result, nil
}
