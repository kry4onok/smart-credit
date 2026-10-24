package get

import (
	"context"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
)

type Handler struct{ usecase Usecase }

func New(usecase Usecase) *Handler { return &Handler{usecase: usecase} }

func (h *Handler) Get(ctx context.Context, _ generated.GetJwksRequestObject) (generated.GetJwksResponseObject, error) {
	if h.usecase == nil {
		return nil, errors.New("JWKS usecase is not configured")
	}
	keys, err := h.usecase.Get(ctx)
	if err != nil {
		return nil, err
	}
	var body generated.Jwks
	for _, key := range keys {
		body.Keys = append(body.Keys, struct {
			Alg generated.JwksKeysAlg `json:"alg"`
			E   string                `json:"e"`
			Kid string                `json:"kid"`
			Kty generated.JwksKeysKty `json:"kty"`
			N   string                `json:"n"`
			Use generated.JwksKeysUse `json:"use"`
		}{Alg: generated.RS256, E: key.E, Kid: key.KeyID, Kty: generated.RSA, N: key.N, Use: generated.Sig})
	}
	cacheControl := "public, max-age=300"
	return generated.GetJwks200JSONResponse{Body: body,
		Headers: generated.GetJwks200ResponseHeaders{CacheControl: &cacheControl}}, nil
}
