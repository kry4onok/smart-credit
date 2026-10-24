package login

import (
	"errors"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

type fixture struct {
	users        *usersMock
	passwords    *passwordsMock
	sessions     *sessionsMock
	access       *accessMock
	transactions *transactionsMock
	params       Params
}

func setup() fixture {
	user := model.User{Principal: model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: model.RoleClient}, PasswordHash: "stored", Active: true}
	f := fixture{users: &usersMock{user: user, current: user}, passwords: &passwordsMock{match: true}, sessions: &sessionsMock{}, access: &accessMock{}, transactions: &transactionsMock{}}
	f.params = Params{Users: f.users, Passwords: f.passwords, Sessions: f.sessions, AccessTokens: f.access,
		RefreshTokens: refreshMock{}, Transactions: f.transactions, DummyPasswordHash: "dummy"}
	return f
}

func TestLoginNormalizesEmailAndStoresOnlyHash(t *testing.T) {
	f := setup()
	f.users.current.CanSubmit = true
	u := New(f.params)
	now := time.Date(2026, 10, 24, 12, 0, 0, 123456789, time.UTC)
	u.now = func() time.Time { return now }
	result, err := u.Execute(t.Context(), model.LoginParams{Email: " User@Example.test ", Password: " password "})
	if err != nil {
		t.Fatal(err)
	}
	if f.users.email != "user@example.test" || result.AccessToken.Token != "access" || result.RefreshToken != "refresh" || len(f.sessions.saved) != 1 {
		t.Fatal("login result mismatch")
	}
	session := f.sessions.saved[0]
	if session.Hash != ([32]byte{1}) || session.UserID != f.users.user.UserID || session.ExpiresAt.Sub(session.CreatedAt) != model.RefreshLifetime ||
		!session.CreatedAt.Equal(now.Truncate(time.Microsecond)) || !session.ExpiresAt.Equal(result.RefreshExpiresAt) || f.transactions.calls != 1 {
		t.Fatal("session persistence mismatch")
	}
	if len(f.access.principals) != 1 || !f.access.principals[0].CanSubmit {
		t.Fatal("login did not use current permissions")
	}
}

func TestInvalidCredentialsAreIndistinguishable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*fixture)
		dummy  bool
	}{
		{"unknown user", func(f *fixture) { f.users.findErr = domain.ErrUserNotFound }, true},
		{"wrong password", func(f *fixture) { f.passwords.match = false }, false},
		{"inactive", func(f *fixture) { f.users.user.Active = false }, false},
		{"corrupt hash", func(f *fixture) { f.passwords.err = errors.New("corrupt hash") }, true},
		{"disabled during verification", func(f *fixture) { f.users.current.Active = false }, false},
		{"password changed during verification", func(f *fixture) { f.users.current.PasswordHash = "changed" }, false},
		{"deleted during verification", func(f *fixture) { f.users.lockErr = domain.ErrUserNotFound }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup()
			tc.change(&f)
			result, err := New(f.params).Execute(t.Context(), model.LoginParams{Email: "user@example.test", Password: "password"})
			if !errors.Is(err, domain.ErrUnauthorized) || result != (model.LoginResult{}) || len(f.access.principals) != 0 || len(f.sessions.saved) != 0 {
				t.Fatal("invalid credentials must never issue a session")
			}
			if len(f.passwords.hashes) == 0 {
				t.Fatal("password work was skipped")
			}
			if tc.dummy && f.passwords.hashes[len(f.passwords.hashes)-1] != "dummy" {
				t.Fatal("dummy password verification was skipped")
			}
		})
	}
}

func TestLoginInfrastructureFailuresReturnNoTokens(t *testing.T) {
	failure := errors.New("dependency failure")
	for _, tc := range []struct {
		name   string
		change func(*fixture)
	}{
		{"read", func(f *fixture) { f.users.findErr = failure }},
		{"lock", func(f *fixture) { f.users.lockErr = failure }},
		{"sign", func(f *fixture) { f.access.err = failure }},
		{"random", func(f *fixture) { f.params.RefreshTokens = refreshMock{err: failure} }},
		{"save", func(f *fixture) { f.sessions.err = failure }},
		{"commit", func(f *fixture) { f.transactions.commitErr = failure }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup()
			tc.change(&f)
			result, err := New(f.params).Execute(t.Context(), model.LoginParams{Email: "user@example.test", Password: "password"})
			if !errors.Is(err, failure) || result != (model.LoginResult{}) {
				t.Fatal("failed login leaked tokens or swallowed dependency failure")
			}
		})
	}
}
