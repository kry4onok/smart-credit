package get

import (
	"context"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type Usecase struct{ keys KeyProvider }

func New(keys KeyProvider) *Usecase { return &Usecase{keys: keys} }

func (u *Usecase) Get(context.Context) ([]model.PublicKey, error) {
	if u.keys == nil {
		return nil, errors.New("signing key is not configured")
	}
	return []model.PublicKey{u.keys.PublicKey()}, nil
}
