package logout

import "context"

type Sessions interface {
	Revoke(context.Context, [32]byte) error
}
type RefreshTokens interface {
	Hash(string) ([32]byte, error)
}
type Params struct {
	Sessions      Sessions
	RefreshTokens RefreshTokens
}
