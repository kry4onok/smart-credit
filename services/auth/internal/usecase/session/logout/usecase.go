package logout

import "context"

type Usecase struct{ params Params }

func New(params Params) *Usecase { return &Usecase{params: params} }

func (u *Usecase) Execute(ctx context.Context, rawToken string) error {
	hash, err := u.params.RefreshTokens.Hash(rawToken)
	if err != nil {
		return nil
	}
	return u.params.Sessions.Revoke(ctx, hash)
}
