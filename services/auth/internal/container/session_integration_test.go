package container

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/jwt"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/refreshtoken"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	refreshrepo "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/repository/refresh"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func sessionDatabase(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid AUTH_TEST_DATABASE_URL")
	}
	schema := "auth_session_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	pool, err := pgxpool.New(t.Context(), u.String())
	if err != nil {
		t.Fatal("create test database pool")
	}
	t.Cleanup(func() {
		defer pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	if _, err := pool.Exec(t.Context(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile("../../migrations/00001_auth_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, found := strings.Cut(string(sql), "-- +goose Down")
	if !found {
		t.Fatal("missing migration Up")
	}
	if _, err := pool.Exec(t.Context(), up); err != nil {
		t.Fatal(err)
	}
	return pool, u.String()
}

func sessionContainer(t *testing.T, dsn string) *Container {
	t.Helper()
	key := filepath.Join(t.TempDir(), "private.pem")
	if err := jwt.GenerateKey(key); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(key)
	cfg.Database.URL = dsn
	cfg.Database.MaxConnections = 4
	cfg.Database.ConnectTimeout = 5 * time.Second
	cfg.ReadinessTimeout = time.Second
	cfg.Cookie = config.CookieConfig{Name: "refresh", Path: "/api/auth", Secure: true}
	c, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

func addUser(t *testing.T, db *pgxpool.Pool, c *Container, role string, active bool) model.Principal {
	t.Helper()
	p := model.Principal{UserID: uuid.New(), Role: role}
	var org any
	if role == model.RoleClient {
		p.OrgID = uuid.New()
		org = p.OrgID
		if _, err := db.Exec(t.Context(), `INSERT INTO organizations (id, inn, name) VALUES ($1, '7707083893', 'Тест')`, org); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := c.Passwords.Hash(" пароль 🔐 ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), `INSERT INTO users (id, email, full_name, password_hash, role, org_id, is_active)
	VALUES ($1, $2, 'Тест', $3, $4, $5, $6)`, p.UserID, p.UserID.String()+"@example.test", hash, role, org, active); err != nil {
		t.Fatal(err)
	}
	return p
}

func sessionRequest(c *Container, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer expired-access-token")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.HTTPHandler.ServeHTTP(w, r)
	return w
}

func checkStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d, want=%d", w.Code, status)
	}
}

func accessResponse(t *testing.T, c *Container, w *httptest.ResponseRecorder) model.Principal {
	t.Helper()
	checkStatus(t, w, 200)
	var body generated.AccessTokenResponse
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.TokenType != generated.Bearer || body.ExpiresIn != 900 {
		t.Fatal("invalid token response")
	}
	p, err := c.Tokens.Verify(t.Context(), body.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func loginUser(t *testing.T, c *Container, p model.Principal) (*http.Cookie, *httptest.ResponseRecorder) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"email": strings.ToUpper(p.UserID.String() + "@example.test"), "password": " пароль 🔐 "})
	if err != nil {
		t.Fatal(err)
	}
	w := sessionRequest(c, "/auth/login", string(body), nil)
	if got := accessResponse(t, c, w); got != p {
		t.Fatal("login principal mismatch")
	}
	cookies := (&http.Response{Header: w.Header()}).Cookies()
	if len(cookies) != 1 {
		t.Fatal("login did not issue a refresh cookie")
	}
	return cookies[0], w
}

func TestSessionLifecycleIntegration(t *testing.T) {
	db, dsn := sessionDatabase(t)
	c := sessionContainer(t, dsn)
	for _, role := range []string{model.RoleClient, model.RoleAnalyst, model.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			p := addUser(t, db, c, role, true)
			cookie, _ := loginUser(t, c, p)
			hash := sha256.Sum256([]byte(cookie.Value))
			var stored []byte
			var created, expires time.Time
			if err := db.QueryRow(t.Context(), `SELECT token_hash, created_at, expires_at FROM refresh_tokens WHERE user_id=$1`, p.UserID).Scan(&stored, &created, &expires); err != nil {
				t.Fatal(err)
			}
			if len(stored) != 32 || string(stored) != string(hash[:]) || expires.Sub(created) != model.RefreshLifetime {
				t.Fatal("refresh token persisted incorrectly")
			}
			for range 2 {
				w := sessionRequest(c, "/auth/refresh", "", cookie)
				if got := accessResponse(t, c, w); got != p || w.Header().Get("Set-Cookie") != "" {
					t.Fatal("refresh changed session or principal")
				}
			}
			var after time.Time
			if err := db.QueryRow(t.Context(), `SELECT expires_at FROM refresh_tokens WHERE token_hash=$1`, hash[:]).Scan(&after); err != nil || !after.Equal(expires) {
				t.Fatal("refresh extended the original lifetime")
			}
			var otherCookie *http.Cookie
			if role == model.RoleClient {
				otherCookie, _ = loginUser(t, c, p)
				if otherCookie.Value == cookie.Value {
					t.Fatal("separate logins reused a refresh token")
				}
				if _, err := db.Exec(t.Context(), `UPDATE users SET can_submit=true WHERE id=$1`, p.UserID); err != nil {
					t.Fatal(err)
				}
				if !accessResponse(t, c, sessionRequest(c, "/auth/refresh", "", cookie)).CanSubmit {
					t.Fatal("refresh used stale permissions")
				}
				if _, err := db.Exec(t.Context(), `UPDATE users SET role='ADMIN', org_id=NULL, can_submit=false WHERE id=$1`, p.UserID); err != nil {
					t.Fatal(err)
				}
				updated := accessResponse(t, c, sessionRequest(c, "/auth/refresh", "", cookie))
				if updated.UserID != p.UserID || updated.Role != model.RoleAdmin || updated.OrgID != uuid.Nil || updated.CanSubmit {
					t.Fatal("refresh used a stale role or organization")
				}
			}
			checkStatus(t, sessionRequest(c, "/auth/logout", "", cookie), 204)
			var revoked time.Time
			if err := db.QueryRow(t.Context(), `SELECT revoked_at FROM refresh_tokens WHERE token_hash=$1`, hash[:]).Scan(&revoked); err != nil {
				t.Fatal("logout did not persist revocation")
			}
			checkStatus(t, sessionRequest(c, "/auth/logout", "", cookie), 204)
			var second time.Time
			if err := db.QueryRow(t.Context(), `SELECT revoked_at FROM refresh_tokens WHERE token_hash=$1`, hash[:]).Scan(&second); err != nil || !second.Equal(revoked) {
				t.Fatal("repeated logout changed revocation")
			}
			checkStatus(t, sessionRequest(c, "/auth/refresh", "", cookie), 401)
			if otherCookie != nil {
				checkStatus(t, sessionRequest(c, "/auth/refresh", "", otherCookie), 200)
				checkStatus(t, sessionRequest(c, "/auth/logout", "", otherCookie), 204)
			}
		})
	}
	checkStatus(t, sessionRequest(c, "/auth/refresh", "", nil), 401)
	checkStatus(t, sessionRequest(c, "/auth/logout", "", nil), 204)
}

func TestLoginRejectionsAndRefreshExpiryIntegration(t *testing.T) {
	db, dsn := sessionDatabase(t)
	c := sessionContainer(t, dsn)
	p := addUser(t, db, c, model.RoleAdmin, true)
	var expectedDetail string
	for _, tc := range []struct {
		email, password string
		inactive        bool
	}{
		{p.UserID.String() + "@example.test", "wrong", false},
		{"unknown@example.test", "wrong", false},
		{p.UserID.String() + "@example.test", " пароль 🔐 ", true},
	} {
		if tc.inactive {
			if _, err := db.Exec(t.Context(), `UPDATE users SET is_active=false WHERE id=$1`, p.UserID); err != nil {
				t.Fatal(err)
			}
		}
		data, err := json.Marshal(map[string]string{"email": tc.email, "password": tc.password})
		if err != nil {
			t.Fatal(err)
		}
		w := sessionRequest(c, "/auth/login", string(data), nil)
		checkStatus(t, w, 401)
		var problem common.Problem
		if json.Unmarshal(w.Body.Bytes(), &problem) != nil || problem.Code != "UNAUTHORIZED" || problem.Detail == nil || w.Header().Get("Set-Cookie") != "" {
			t.Fatal("invalid credentials response")
		}
		if expectedDetail == "" {
			expectedDetail = *problem.Detail
		} else if expectedDetail != *problem.Detail {
			t.Fatal("credential errors differ")
		}
	}
	var count int
	if err := db.QueryRow(t.Context(), `SELECT count(*) FROM refresh_tokens`).Scan(&count); err != nil || count != 0 {
		t.Fatal("rejected login created a session")
	}
	if _, err := db.Exec(t.Context(), `UPDATE users SET is_active=true WHERE id=$1`, p.UserID); err != nil {
		t.Fatal(err)
	}
	cookie, _ := loginUser(t, c, p)
	if _, err := db.Exec(t.Context(), `UPDATE users SET is_active=false WHERE id=$1`, p.UserID); err != nil {
		t.Fatal(err)
	}
	checkStatus(t, sessionRequest(c, "/auth/refresh", "", cookie), 401)
	if _, err := db.Exec(t.Context(), `UPDATE users SET is_active=true WHERE id=$1`, p.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), `UPDATE refresh_tokens SET created_at=now()-interval '8 days', expires_at=now()-interval '1 day' WHERE user_id=$1`, p.UserID); err != nil {
		t.Fatal(err)
	}
	checkStatus(t, sessionRequest(c, "/auth/refresh", "", cookie), 401)
	secret, err := refreshtoken.New().Generate()
	if err != nil {
		t.Fatal(err)
	}
	unknown := &http.Cookie{Name: "refresh", Value: secret.Token}
	checkStatus(t, sessionRequest(c, "/auth/refresh", "", unknown), 401)
	checkStatus(t, sessionRequest(c, "/auth/logout", "", unknown), 204)
}

func TestLoginCommitFailureDoesNotExposeTokensIntegration(t *testing.T) {
	db, dsn := sessionDatabase(t)
	c := sessionContainer(t, dsn)
	p := addUser(t, db, c, model.RoleAdmin, true)
	_, err := db.Exec(t.Context(), `CREATE FUNCTION fail_session_commit() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN RAISE EXCEPTION 'private-commit-failure'; END $$;
	CREATE CONSTRAINT TRIGGER reject_session AFTER INSERT ON refresh_tokens
	DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_session_commit()`)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]string{"email": p.UserID.String() + "@example.test", "password": " пароль 🔐 "})
	if err != nil {
		t.Fatal(err)
	}
	w := sessionRequest(c, "/auth/login", string(data), nil)
	checkStatus(t, w, 500)
	if w.Header().Get("Set-Cookie") != "" || strings.Contains(w.Body.String(), "access_token") || strings.Contains(w.Body.String(), "private-commit-failure") {
		t.Fatal("failed transaction exposed tokens or database details")
	}
	var count int
	if err := db.QueryRow(t.Context(), `SELECT count(*) FROM refresh_tokens`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed transaction persisted a session")
	}
}

func TestRefreshLocksSerializeLogoutIntegration(t *testing.T) {
	db, dsn := sessionDatabase(t)
	c := sessionContainer(t, dsn)
	p := addUser(t, db, c, model.RoleAdmin, true)
	cookie, _ := loginUser(t, c, p)
	hash := sha256.Sum256([]byte(cookie.Value))
	sessions, transactions := refreshrepo.New(db), transaction.New(db)
	locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() {
		done <- transactions.Within(ctx, func(ctx context.Context) error {
			if _, err := sessions.Lock(ctx, hash); err != nil {
				return err
			}
			close(locked)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	select {
	case <-locked:
	case err := <-done:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	err := sessions.Revoke(short, hash)
	stop()
	close(release)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("logout did not wait for refresh lock")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := sessions.Revoke(ctx, hash); err != nil {
		t.Fatal(err)
	}
	checkStatus(t, sessionRequest(c, "/auth/refresh", "", cookie), 401)
}
