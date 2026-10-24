package refresh

import (
	"context"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/sessioncookie"
)

type Handler struct{ usecase Usecase }

func New(usecase Usecase) *Handler { return &Handler{usecase: usecase} }

func (h *Handler) Refresh(ctx context.Context, _ generated.RefreshSessionRequestObject) (generated.RefreshSessionResponseObject, error) {
	if h.usecase == nil {
		return nil, errors.New("refresh is not configured")
	}
	token, err := h.usecase.Execute(ctx, sessioncookie.Token(ctx))
	if err != nil {
		return nil, err
	}
	return generated.RefreshSession200JSONResponse{AccessToken: token.Token, TokenType: generated.Bearer, ExpiresIn: token.ExpiresIn}, nil
}
