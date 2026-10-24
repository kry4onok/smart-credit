package refresh

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type sessionsMock struct {
	state model.SessionState
	err   error
	calls int
}

func (m *sessionsMock) Lock(context.Context, [32]byte) (model.SessionState, error) {
	m.calls++
	return m.state, m.err
}

type accessMock struct {
	principal model.Principal
	err       error
	calls     int
}

func (m *accessMock) Issue(p model.Principal) (model.AccessToken, error) {
	m.principal = p
	m.calls++
	return model.AccessToken{Token: "access", ExpiresIn: 900}, m.err
}

type refreshMock struct{ err error }

func (m refreshMock) Hash(string) ([32]byte, error) { return [32]byte{1}, m.err }

type transactionsMock struct{ err error }

func (m transactionsMock) Within(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	return m.err
}
