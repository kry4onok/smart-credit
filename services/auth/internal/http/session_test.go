package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

func cookieConfig() config.CookieConfig {
	return config.CookieConfig{Name: "refresh", Path: "/api/auth", Secure: true}
}
func responseCookies(w *httptest.ResponseRecorder) []*http.Cookie {
	return (&http.Response{Header: w.Header()}).Cookies()
}

func TestLoginResponseAndCookie(t *testing.T) {
	expires := time.Now().Add(model.RefreshLifetime).Truncate(time.Second)
	handler := New(Params{Cookie: cookieConfig(), Login: loginFunc(func(_ context.Context, p model.LoginParams) (model.LoginResult, error) {
		if p.Password != " password " {
			t.Fatal("password was normalized")
		}
		return model.LoginResult{AccessToken: model.AccessToken{Token: "access", ExpiresIn: 900}, RefreshToken: "private-refresh", RefreshExpiresAt: expires}, nil
	})})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"user@example.test","password":" password "}`))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	handler.ServeHTTP(w, r)
	var body map[string]any
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || len(body) != 3 || body["access_token"] != "access" || body["token_type"] != "Bearer" || body["expires_in"] != float64(900) || w.Code != 200 {
		t.Fatal("login response contract mismatch")
	}
	cookies := responseCookies(w)
	if len(cookies) != 1 {
		t.Fatal("missing refresh cookie")
	}
	cookie := cookies[0]
	if cookie.Name != "refresh" || cookie.Value != "private-refresh" || cookie.Path != "/api/auth" || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 604800 || !cookie.Expires.Equal(expires) || cookie.Domain != "" {
		t.Fatal("unsafe or incorrect refresh cookie")
	}
	if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "private-refresh") {
		t.Fatal("session data could be cached or refresh token leaked in body")
	}
}

func TestInvalidLoginRequests(t *testing.T) {
	for _, tc := range []struct{ body, contentType string }{
		{`{`, "application/json"}, {`null`, "application/json"}, {`{}`, "application/json"},
		{`{"email":"user@example.test"}`, "application/json"},
		{`{"email":"bad","password":"pw"}`, "application/json"},
		{`{"email":null,"password":"pw"}`, "application/json"},
		{`{"email":"user@example.test","password":""}`, "application/json"},
		{`{"email":"user@example.test","password":null}`, "application/json"},
		{`{"email":"user@example.test","password":123}`, "application/json"},
		{`{"email":"user@example.test","password":"pw","role":"ADMIN"}`, "application/json"},
		{`{"Email":"user@example.test","password":"pw"}`, "application/json"},
		{`{"email":"user@example.test","password":"pw"} {}`, "application/json"},
		{`{"email":"user@example.test","password":"pw"}`, "text/plain"},
		{`{"email":"user@example.test","password":"pw"}`, ""},
		{`{"email":"user@example.test","password":"` + strings.Repeat("x", 65536) + `"}`, "application/json"},
	} {
		t.Run("invalid request", func(t *testing.T) {
			handler := New(Params{Cookie: cookieConfig(), Login: loginFunc(func(context.Context, model.LoginParams) (model.LoginResult, error) {
				t.Fatal("invalid input reached usecase")
				return model.LoginResult{}, nil
			})})
			r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			assertProblem(t, w, 400, "BAD_REQUEST")
		})
	}
}

func TestSessionErrorsAreSanitized(t *testing.T) {
	for _, err := range []error{domain.ErrUnauthorized, errors.New("private-password-and-token")} {
		for _, route := range []string{"login", "refresh", "logout"} {
			if route == "logout" && errors.Is(err, domain.ErrUnauthorized) {
				continue
			}
			handler := New(Params{Cookie: cookieConfig(), Login: loginFunc(func(context.Context, model.LoginParams) (model.LoginResult, error) { return model.LoginResult{}, err }),
				Refresh: refreshFunc(func(context.Context, string) (model.AccessToken, error) { return model.AccessToken{}, err }),
				Logout:  logoutFunc(func(context.Context, string) error { return err })})
			r := httptest.NewRequest(http.MethodPost, "/auth/"+route, strings.NewReader(`{"email":"user@example.test","password":"pw"}`))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if errors.Is(err, domain.ErrUnauthorized) {
				assertProblem(t, w, 401, "UNAUTHORIZED")
			} else {
				assertProblem(t, w, 500, "INTERNAL_ERROR")
			}
			if w.Header().Get("Set-Cookie") != "" || strings.Contains(w.Body.String(), "private-password-and-token") {
				t.Fatal("failure leaked secrets or changed cookie")
			}
		}
	}
}

func TestRefreshReadsOnlyConfiguredCookieAndDoesNotRotate(t *testing.T) {
	handler := New(Params{Cookie: cookieConfig(), Refresh: refreshFunc(func(_ context.Context, token string) (model.AccessToken, error) {
		if token != "correct" {
			return model.AccessToken{}, domain.ErrUnauthorized
		}
		return model.AccessToken{Token: "new-access", ExpiresIn: 900}, nil
	})})
	for _, cookie := range []string{"refresh=correct", "other=correct", "", "refresh=correct; refresh=other"} {
		r := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		r.Header.Set("Authorization", "Bearer expired-access")
		r.Header.Set("Cookie", cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if cookie == "refresh=correct" {
			if w.Code != 200 || !strings.Contains(w.Body.String(), `"access_token":"new-access"`) || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("refresh rotated cookie or rejected valid session")
			}
		} else {
			assertProblem(t, w, 401, "UNAUTHORIZED")
		}
	}
}

func TestLogoutClearsCookieAndNeedsNoAccessToken(t *testing.T) {
	handler := New(Params{Cookie: cookieConfig(), Logout: logoutFunc(func(context.Context, string) error { return nil })})
	for _, raw := range []string{"", "refresh=old", "refresh=malformed"} {
		r := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		r.Header.Set("Cookie", raw)
		r.Header.Set("Authorization", "Bearer expired-access")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		cookies := responseCookies(w)
		if w.Code != 204 || w.Body.Len() != 0 || len(cookies) != 1 {
			t.Fatal("logout must be idempotent 204 with an empty body")
		}
		c := cookies[0]
		if c.Name != "refresh" || c.Value != "" || c.Path != "/api/auth" || c.MaxAge != -1 || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || !c.Expires.Before(time.Now()) {
			t.Fatal("cookie was not correctly cleared")
		}
	}
}

func TestLoginRateLimitUsesPeerAddress(t *testing.T) {
	calls := 0
	handler := New(Params{Login: loginFunc(func(context.Context, model.LoginParams) (model.LoginResult, error) {
		calls++
		return model.LoginResult{}, domain.ErrUnauthorized
	})})
	for i := range 11 {
		r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"user@example.test","password":"pw"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-For", time.Now().String())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if i < 10 {
			assertProblem(t, w, 401, "UNAUTHORIZED")
		} else {
			assertProblem(t, w, 429, "RATE_LIMITED")
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("missing Retry-After")
			}
		}
	}
	if calls != 10 {
		t.Fatal("rate limit did not stop password verification")
	}
}

func assertProblem(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var problem common.Problem
	if json.Unmarshal(w.Body.Bytes(), &problem) != nil || w.Code != status || problem.Status != status || problem.Code != code ||
		w.Header().Get("Content-Type") != "application/problem+json" || problem.TraceId == "" || problem.TraceId != w.Header().Get("X-Trace-ID") {
		t.Fatalf("unexpected problem response: status=%d code=%s", w.Code, problem.Code)
	}
}
