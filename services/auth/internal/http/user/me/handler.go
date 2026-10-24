package me

import (
	"context"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type Handler struct{ usecase Usecase }

func New(usecase Usecase) *Handler { return &Handler{usecase: usecase} }

func (h *Handler) Me(ctx context.Context, _ generated.GetCurrentUserRequestObject) (generated.GetCurrentUserResponseObject, error) {
	principal, ok := middleware.Principal(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	if h.usecase == nil {
		return nil, errors.New("current user usecase is not configured")
	}
	user, err := h.usecase.Execute(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	response := generated.GetCurrentUser200JSONResponse{Id: user.ID, Email: openapi_types.Email(user.Email),
		FullName: user.FullName, Role: common.Role(user.Role), CanSubmit: user.CanSubmit}
	if user.Organization != nil {
		response.Organization = &struct {
			Id   common.Id `json:"id"`
			Inn  string    `json:"inn"`
			Name string    `json:"name"`
		}{Id: user.Organization.ID, Inn: user.Organization.INN, Name: user.Organization.Name}
	}
	return response, nil
}
