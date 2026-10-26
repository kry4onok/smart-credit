package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
)

func TestLoginLimitExpiresAndIgnoresSourcePort(t *testing.T) {
	s := NewSessions(config.CookieConfig{})
	now := time.Now()
	s.now = func() time.Time { return now }
	for range loginAttempts {
		if !s.allow("127.0.0.1:1000") {
			t.Fatal("limit reached too early")
		}
	}
	if s.allow("127.0.0.1:2000") {
		t.Fatal("changing source port bypassed the limit")
	}
	now = now.Add(loginWindow)
	if !s.allow("127.0.0.1:2000") {
		t.Fatal("expired limit did not reset")
	}
}

func TestLoginConcurrentWorkIsBounded(t *testing.T) {
	s := NewSessions(config.CookieConfig{})
	entered, release, done := make(chan struct{}, 4), make(chan struct{}), make(chan int, 4)
	handler := s.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-release:
			w.WriteHeader(204)
		case <-r.Context().Done():
			w.WriteHeader(500)
		}
	}))
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"user@example.test","password":"pw"}`))
		r.Header.Set("Content-Type", "application/json")
		return r.WithContext(t.Context())
	}
	defer close(release)
	for range 4 {
		go func() { w := httptest.NewRecorder(); handler.ServeHTTP(w, request()); done <- w.Code }()
	}
	for range 4 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("login requests did not enter the handler")
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	if w.Code != http.StatusTooManyRequests {
		t.Fatal("more than four login computations were allowed")
	}
	// Drain the workers before this test's context is cancelled.
	for range 4 {
		release <- struct{}{}
	}
	for range 4 {
		if status := <-done; status != 204 {
			t.Fatal("accepted login did not complete")
		}
	}
}

func TestLoginPeerUsesProxyAddressOnlyFromPrivateNetwork(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	r.RemoteAddr = "172.18.0.5:41000"
	r.Header.Set("X-Real-IP", "192.168.65.1")
	if got := loginPeer(r); got != "192.168.65.1" {
		t.Fatalf("behind nginx peer = %q", got)
	}
	r.RemoteAddr = "203.0.113.7:5000"
	if got := loginPeer(r); got != "203.0.113.7:5000" {
		t.Fatalf("public client overrode its address: %q", got)
	}
	r.RemoteAddr = "172.18.0.5:41000"
	r.Header.Set("X-Real-IP", "not-an-ip")
	if got := loginPeer(r); got != "172.18.0.5:41000" {
		t.Fatalf("invalid header was trusted: %q", got)
	}
}
