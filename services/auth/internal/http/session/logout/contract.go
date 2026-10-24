package logout

import "context"

type Usecase interface {
	Execute(context.Context, string) error
}
type Cookies interface{ Clear() string }
type Params struct {
	Usecase Usecase
	Cookies Cookies
}
