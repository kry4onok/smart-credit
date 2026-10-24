package logout

import (
	"context"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/sessioncookie"
)

type Handler struct{ params Params }

func New(params Params) *Handler { return &Handler{params: params} }

func (h *Handler) Logout(ctx context.Context, _ generated.LogoutRequestObject) (generated.LogoutResponseObject, error) {
	if h.params.Usecase == nil || h.params.Cookies == nil {
		return nil, errors.New("logout is not configured")
	}
	if err := h.params.Usecase.Execute(ctx, sessioncookie.Token(ctx)); err != nil {
		return nil, err
	}
	cookie := h.params.Cookies.Clear()
	return generated.Logout204Response{Headers: generated.Logout204ResponseHeaders{SetCookie: &cookie}}, nil
}
