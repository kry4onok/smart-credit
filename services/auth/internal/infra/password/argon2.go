// Package password hashes passwords in the Argon2id PHC format stored in users.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memory  = 64 * 1024 // KiB; RFC 9106's second recommended Argon2id profile.
	passes  = 3
	threads = 4
	saltLen = 16
	keyLen  = 32
)

var ErrInvalidHash = errors.New("invalid Argon2id hash")

type Hasher struct{}

func New() *Hasher { return &Hasher{} }

func (*Hasher) Hash(password string) (string, error) {
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, passes, memory, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, passes, threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify bounds the PHC work factors before allocating Argon2 memory. A malformed
// database value must not cause unbounded work or an argon2.IDKey panic.
func (*Hasher) Verify(hash, password string) (bool, error) {
	if len(hash) > 256 {
		return false, ErrInvalidHash
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, ErrInvalidHash
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false, ErrInvalidHash
	}
	m, mErr := parameter(params[0], "m=")
	t, tErr := parameter(params[1], "t=")
	p, pErr := parameter(params[2], "p=")
	if mErr != nil || tErr != nil || pErr != nil || p < 1 || p > threads || t < 1 || t > passes || m < 8*p || m > memory {
		return false, ErrInvalidHash
	}
	salt, saltErr := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	key, keyErr := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if saltErr != nil || keyErr != nil || len(salt) != saltLen || len(key) != keyLen ||
		len(parts[4]) != 22 || len(parts[5]) != 43 {
		return false, ErrInvalidHash
	}
	actual := argon2.IDKey([]byte(password), salt, t, m, uint8(p), keyLen)
	return subtle.ConstantTimeCompare(actual, key) == 1, nil
}

func parameter(value, prefix string) (uint32, error) {
	if !strings.HasPrefix(value, prefix) {
		return 0, ErrInvalidHash
	}
	raw := strings.TrimPrefix(value, prefix)
	if raw == "" || raw[0] < '1' || raw[0] > '9' {
		return 0, ErrInvalidHash
	}
	n, err := strconv.ParseUint(raw, 10, 32)
	return uint32(n), err
}
