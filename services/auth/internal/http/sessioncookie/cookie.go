package sessioncookie

import (
	"context"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
)

type contextKey struct{}

func WithRequest(ctx context.Context, r *http.Request, name string) context.Context {
	var token string
	if cookies := r.CookiesNamed(name); len(cookies) == 1 {
		token = cookies[0].Value
	}
	return context.WithValue(ctx, contextKey{}, token)
}

func Token(ctx context.Context) string {
	token, _ := ctx.Value(contextKey{}).(string)
	return token
}

type Cookies struct{ cfg config.CookieConfig }

func New(cfg config.CookieConfig) *Cookies { return &Cookies{cfg: cfg} }

func (c *Cookies) Set(token string, expiresAt time.Time) string {
	cookie := c.base()
	cookie.Value = token
	cookie.MaxAge = int(model.RefreshLifetime / time.Second)
	cookie.Expires = expiresAt.UTC()
	return cookie.String()
}

func (c *Cookies) Clear() string {
	cookie := c.base()
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0).UTC()
	return cookie.String()
}

func (c *Cookies) base() http.Cookie {
	return http.Cookie{Name: c.cfg.Name, Path: c.cfg.Path, HttpOnly: true,
		Secure: c.cfg.Secure, SameSite: http.SameSiteStrictMode}
}
