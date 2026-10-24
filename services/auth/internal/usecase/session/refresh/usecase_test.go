package refresh

import (
	"errors"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

func TestRefreshSessionValidity(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name   string
		change func(*model.SessionState)
		valid  bool
	}{
		{"active", func(*model.SessionState) {}, true},
		{"inactive user", func(s *model.SessionState) { s.Active = false }, false},
		{"expired", func(s *model.SessionState) { s.ExpiresAt = now.Add(-time.Second) }, false},
		{"expiry boundary", func(s *model.SessionState) { s.ExpiresAt = now }, false},
		{"future creation", func(s *model.SessionState) { s.CreatedAt = now.Add(time.Second) }, false},
		{"revoked", func(s *model.SessionState) { s.RevokedAt = &now }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := model.SessionState{Principal: model.Principal{UserID: uuid.New(), Role: model.RoleAdmin}, Active: true, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
			tc.change(&state)
			sessions, access := &sessionsMock{state: state}, &accessMock{}
			u := New(Params{Sessions: sessions, AccessTokens: access, RefreshTokens: refreshMock{}, Transactions: transactionsMock{}})
			u.now = func() time.Time { return now }
			result, err := u.Execute(t.Context(), "refresh")
			if tc.valid {
				if err != nil || result.Token != "access" || access.principal != state.Principal {
					t.Fatal("valid session rejected or stale permissions issued")
				}
			} else if !errors.Is(err, domain.ErrUnauthorized) || result != (model.AccessToken{}) || access.calls != 0 {
				t.Fatal("invalid session issued access token")
			}
		})
	}
}

func TestRefreshFailuresReturnNoToken(t *testing.T) {
	failure := errors.New("dependency failure")
	for _, stage := range []string{"hash", "read", "sign", "commit"} {
		t.Run(stage, func(t *testing.T) {
			now := time.Now()
			sessions := &sessionsMock{state: model.SessionState{Active: true, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}}
			access := &accessMock{}
			params := Params{Sessions: sessions, AccessTokens: access, RefreshTokens: refreshMock{}, Transactions: transactionsMock{}}
			want := failure
			switch stage {
			case "hash":
				params.RefreshTokens = refreshMock{err: failure}
				want = domain.ErrUnauthorized
			case "read":
				sessions.err = failure
			case "sign":
				access.err = failure
			case "commit":
				params.Transactions = transactionsMock{err: failure}
			}
			result, err := New(params).Execute(t.Context(), "refresh")
			if !errors.Is(err, want) || result != (model.AccessToken{}) {
				t.Fatal("failed refresh leaked token or swallowed failure")
			}
			if stage == "hash" && sessions.calls != 0 {
				t.Fatal("malformed token reached database")
			}
		})
	}
}
