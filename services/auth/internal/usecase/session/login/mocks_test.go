package login

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

type usersMock struct {
	user, current    model.User
	findErr, lockErr error
	email            string
}

func (m *usersMock) FindByEmail(_ context.Context, email string) (model.User, error) {
	m.email = email
	return m.user, m.findErr
}
func (m *usersMock) LockByID(context.Context, uuid.UUID) (model.User, error) {
	return m.current, m.lockErr
}

type passwordsMock struct {
	hashes []string
	match  bool
	err    error
}

func (m *passwordsMock) Verify(hash, _ string) (bool, error) {
	m.hashes = append(m.hashes, hash)
	return m.match, m.err
}

type sessionsMock struct {
	saved []model.RefreshSession
	err   error
}

func (m *sessionsMock) Save(_ context.Context, value model.RefreshSession) error {
	m.saved = append(m.saved, value)
	return m.err
}

type accessMock struct {
	principals []model.Principal
	err        error
}

func (m *accessMock) Issue(p model.Principal) (model.AccessToken, error) {
	m.principals = append(m.principals, p)
	return model.AccessToken{Token: "access", ExpiresIn: 900}, m.err
}

type refreshMock struct{ err error }

func (m refreshMock) Generate() (model.RefreshSecret, error) {
	return model.RefreshSecret{Token: "refresh", Hash: [32]byte{1}}, m.err
}

type transactionsMock struct {
	calls     int
	commitErr error
}

func (m *transactionsMock) Within(ctx context.Context, fn func(context.Context) error) error {
	m.calls++
	if err := fn(ctx); err != nil {
		return err
	}
	return m.commitErr
}
