package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/sessioncookie"
)

const (
	loginWindow   = time.Minute
	loginAttempts = 10
	maxLoginPeers = 4096
	maxLoginBytes = 64 << 10
	// loginQueueWait bounds how long a login waits for one of the Argon2id slots.
	loginQueueWait = time.Second
)

type attempts struct {
	until time.Time
	count int
}
type Sessions struct {
	cookieName string
	mu         sync.Mutex
	peers      map[string]attempts
	active     chan struct{}
	now        func() time.Time
}

func NewSessions(cfg config.CookieConfig) *Sessions {
	return &Sessions{cookieName: cfg.Name, peers: make(map[string]attempts), active: make(chan struct{}, 4), now: time.Now}
}

func (s *Sessions) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/login", "/auth/refresh", "/auth/logout", "/auth/me":
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
		}
		if r.Method == http.MethodPost && r.URL.Path == "/auth/login" {
			if !s.allow(loginPeer(r)) {
				rateLimited(w, r)
				return
			}
			// A short wait absorbs a burst of simultaneous logins instead of failing them.
			select {
			case s.active <- struct{}{}:
				defer func() { <-s.active }()
			case <-time.After(loginQueueWait):
				rateLimited(w, r)
				return
			case <-r.Context().Done():
				rateLimited(w, r)
				return
			}
			if !validLoginBody(w, r) {
				domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusBadRequest, "BAD_REQUEST", "Некорректный запрос."))
				return
			}
		}
		ctx := sessioncookie.WithRequest(r.Context(), r, s.cookieName)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loginPeer identifies the browser for the login limit. Behind the bundled nginx every
// request comes from the proxy's private address, so the proxy-set X-Real-IP is used;
// a client connecting from a public address cannot override its own address.
func loginPeer(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	proxy := net.ParseIP(host)
	if proxy == nil || !proxy.IsLoopback() && !proxy.IsPrivate() {
		return r.RemoteAddr
	}
	if real := net.ParseIP(r.Header.Get("X-Real-IP")); real != nil {
		return real.String()
	}
	return r.RemoteAddr
}

func (s *Sessions) allow(remote string) bool {
	peer, _, err := net.SplitHostPort(remote)
	if err != nil {
		peer = remote
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	entry, found := s.peers[peer]
	if !found || !now.Before(entry.until) {
		if len(s.peers) >= maxLoginPeers {
			for key, value := range s.peers {
				if !now.Before(value.until) {
					delete(s.peers, key)
				}
			}
			if !found && len(s.peers) >= maxLoginPeers {
				return false
			}
		}
		entry = attempts{until: now.Add(loginWindow)}
	}
	if entry.count >= loginAttempts {
		return false
	}
	entry.count++
	s.peers[peer] = entry
	return true
}

func validLoginBody(w http.ResponseWriter, r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBytes)
	data, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || len(fields) != 2 || fields["email"] == nil || fields["password"] == nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return true
}

func rateLimited(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "60")
	domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusTooManyRequests, "RATE_LIMITED", "Слишком много попыток входа. Повторите позже."))
}
