package get

import "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"

type KeyProvider interface {
	PublicKey() model.PublicKey
}
