package logout

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/refreshtoken"
)

type sessionsMock struct {
	calls int
	hash  [32]byte
	err   error
}

func (m *sessionsMock) Revoke(_ context.Context, hash [32]byte) error {
	m.calls++
	m.hash = hash
	return m.err
}

func TestLogout(t *testing.T) {
	tokens := refreshtoken.New()
	secret, err := tokens.Generate()
	if err != nil {
		t.Fatal(err)
	}
	sessions := &sessionsMock{}
	u := New(Params{Sessions: sessions, RefreshTokens: tokens})
	for _, raw := range []string{"", "malformed"} {
		if err := u.Execute(t.Context(), raw); err != nil || sessions.calls != 0 {
			t.Fatal("empty or invalid token must be an idempotent no-op")
		}
	}
	if err := u.Execute(t.Context(), secret.Token); err != nil || sessions.calls != 1 || sessions.hash != secret.Hash {
		t.Fatal("logout did not revoke the hashed token")
	}
	failure := errors.New("database unavailable")
	sessions.err = failure
	if err := u.Execute(t.Context(), secret.Token); !errors.Is(err, failure) {
		t.Fatal("database failure was silently accepted")
	}
}
