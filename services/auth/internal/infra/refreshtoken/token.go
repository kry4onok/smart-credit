package refreshtoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type Tokens struct{}

func New() *Tokens { return &Tokens{} }

func (*Tokens) Generate() (model.RefreshSecret, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return model.RefreshSecret{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(random[:])
	return model.RefreshSecret{Token: token, Hash: sha256.Sum256([]byte(token))}, nil
}

func (*Tokens) Hash(token string) ([32]byte, error) {
	if len(token) != 43 {
		return [32]byte{}, errors.New("invalid refresh token")
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(data) != 32 {
		return [32]byte{}, errors.New("invalid refresh token")
	}
	return sha256.Sum256([]byte(token)), nil
}
