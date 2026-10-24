package login

import (
	"context"
	"errors"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	params   Params
	validate *validator.Validate
}

func New(params Params) *Handler { return &Handler{params: params, validate: validator.New()} }

func (h *Handler) Login(ctx context.Context, request generated.LoginRequestObject) (generated.LoginResponseObject, error) {
	if request.Body == nil || request.Body.Password == "" || h.validate.Var(strings.TrimSpace(string(request.Body.Email)), "required,email") != nil {
		return nil, domain.ErrInvalidRequest
	}
	if h.params.Usecase == nil || h.params.Cookies == nil {
		return nil, errors.New("login is not configured")
	}
	result, err := h.params.Usecase.Execute(ctx, model.LoginParams{Email: string(request.Body.Email), Password: request.Body.Password})
	if err != nil {
		return nil, err
	}
	cookie := h.params.Cookies.Set(result.RefreshToken, result.RefreshExpiresAt)
	return generated.Login200JSONResponse{Body: generated.AccessTokenResponse{AccessToken: result.AccessToken.Token,
		TokenType: generated.Bearer, ExpiresIn: result.AccessToken.ExpiresIn}, Headers: generated.Login200ResponseHeaders{SetCookie: &cookie}}, nil
}
