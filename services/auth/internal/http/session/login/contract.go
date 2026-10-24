package login

import (
	"context"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type Usecase interface {
	Execute(context.Context, model.LoginParams) (model.LoginResult, error)
}
type Cookies interface {
	Set(string, time.Time) string
}
type Params struct {
	Usecase Usecase
	Cookies Cookies
}
