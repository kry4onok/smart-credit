package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/password"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This opt-in test only recreates services in an explicitly named smoke project.
// Fixture identities and companies are removed; no demo seed is installed.
func TestDockerAuthClientIntegration(t *testing.T) {
	baseURL := os.Getenv("AUTH_DOCKER_TEST_URL")
	if baseURL == "" {
		t.Skip("set AUTH_DOCKER_TEST_* to run the isolated Compose smoke test")
	}
	project := os.Getenv("AUTH_DOCKER_TEST_PROJECT")
	if !strings.HasPrefix(project, "smart-credit-auth-smoke-") {
		t.Fatal("Docker smoke test requires its own smart-credit-auth-smoke-* project")
	}
	envFile, override := os.Getenv("AUTH_DOCKER_TEST_ENV"), os.Getenv("AUTH_DOCKER_TEST_OVERRIDE")
	if envFile == "" || override == "" {
		t.Fatal("set AUTH_DOCKER_TEST_ENV and AUTH_DOCKER_TEST_OVERRIDE")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	open := func(name string) *pgxpool.Pool {
		t.Helper()
		dsn := os.Getenv(name)
		if dsn == "" {
			t.Fatalf("set %s", name)
		}
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal("invalid smoke database configuration")
		}
		t.Cleanup(pool.Close)
		return pool
	}
	authDB, clientDB := open("AUTH_DOCKER_TEST_DATABASE_URL"), open("CLIENT_DOCKER_TEST_DATABASE_URL")
	secret := uuid.NewString() + " пароль "
	hash, err := password.New().Hash(secret)
	if err != nil {
		t.Fatal(err)
	}
	type fixture struct {
		id, org    uuid.UUID
		role, name string
	}
	var fixtures []fixture
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		for _, f := range fixtures {
			if _, err := authDB.Exec(cleanup, `DELETE FROM refresh_tokens WHERE user_id=$1`, f.id); err != nil {
				t.Error(err)
			}
			if _, err := authDB.Exec(cleanup, `DELETE FROM users WHERE id=$1`, f.id); err != nil {
				t.Error(err)
			}
			if f.org != uuid.Nil {
				if _, err := clientDB.Exec(cleanup, `DELETE FROM companies WHERE org_id=$1`, f.org); err != nil {
					t.Error(err)
				}
				if _, err := authDB.Exec(cleanup, `DELETE FROM organizations WHERE id=$1`, f.org); err != nil {
					t.Error(err)
				}
			}
		}
	})
	for i, role := range []string{model.RoleClient, model.RoleClient, model.RoleAnalyst, model.RoleAdmin} {
		f := fixture{id: uuid.New(), role: role, name: "docker-smoke-" + uuid.NewString()}
		if role == model.RoleClient {
			f.org = uuid.New()
		}
		fixtures = append(fixtures, f)
		var org any
		if f.org != uuid.Nil {
			org = f.org
			inn := fmt.Sprintf("%010d", 1000000000+time.Now().UnixNano()%8000000000+int64(i))
			if _, err := authDB.Exec(ctx, `INSERT INTO organizations (id, inn, name) VALUES ($1,$2,$3)`, f.org, inn, f.name); err != nil {
				t.Fatal(err)
			}
			if _, err := clientDB.Exec(ctx, `INSERT INTO companies (org_id, inn, name, okved, registered_at, revenue_12m_kopecks, profit_12m_kopecks, obligations_kopecks)
			VALUES ($1,$2,$3,'62.01','2020-01-01',0,0,0)`, f.org, inn, f.name); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := authDB.Exec(ctx, `INSERT INTO users (id,email,full_name,password_hash,role,org_id,can_submit)
		VALUES ($1,$2,'Docker smoke',$3,$4,$5,$6)`, f.id, f.id.String()+"@example.test", hash, role, org, role == model.RoleClient); err != nil {
			t.Fatal(err)
		}
	}
	request := func(client *http.Client, method, path, token string, data []byte, want int) ([]byte, http.Header) {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if data != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal("HTTP smoke request failed:", err)
		}
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s: status=%d, want=%d", method, strings.Split(path, "?")[0], response.StatusCode, want)
		}
		return body, response.Header
	}
	plain := &http.Client{Timeout: 5 * time.Second}
	request(plain, http.MethodGet, "/internal/auth/readyz", "", nil, 204)
	request(plain, http.MethodGet, "/internal/client/readyz", "", nil, 204)
	request(plain, http.MethodGet, "/api/client/company", "", nil, 401)
	request(plain, http.MethodGet, "/api/client/company", "malformed", nil, 401)
	var firstClient *http.Client
	var firstAccess string
	for i, f := range fixtures {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
		data, err := json.Marshal(map[string]string{"email": f.id.String() + "@example.test", "password": secret})
		if err != nil {
			t.Fatal(err)
		}
		body, headers := request(client, http.MethodPost, "/api/auth/login", "", data, 200)
		var access generated.AccessTokenResponse
		if json.Unmarshal(body, &access) != nil || access.AccessToken == "" {
			t.Fatal("login did not issue an access token")
		}
		cookies := (&http.Response{Header: headers}).Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Path != "/api/auth" || cookies[0].SameSite != http.SameSiteStrictMode {
			t.Fatal("incorrect refresh cookie through gateway")
		}
		body, _ = request(client, http.MethodGet, "/api/auth/me", access.AccessToken, nil, 200)
		var user generated.CurrentUser
		if json.Unmarshal(body, &user) != nil || user.Id != f.id || string(user.Role) != f.role {
			t.Fatal("gateway returned the wrong current user")
		}
		if f.role == model.RoleClient {
			body, _ = request(client, http.MethodGet, "/api/client/company?org_id="+fixtures[1-i].org.String(), access.AccessToken, nil, 200)
			var company struct {
				Profile struct {
					Name string `json:"name"`
				} `json:"profile"`
			}
			if json.Unmarshal(body, &company) != nil || company.Profile.Name != f.name {
				t.Fatal("client did not enforce the organization from the JWT")
			}
		} else {
			request(client, http.MethodGet, "/api/client/company", access.AccessToken, nil, 403)
		}
		if i == 0 {
			firstClient, firstAccess = client, access.AccessToken
		} else {
			request(client, http.MethodPost, "/api/auth/logout", "", nil, 204)
		}
	}
	before, _ := request(plain, http.MethodGet, "/api/.well-known/jwks.json", "", nil, 200)
	composeFile, err := filepath.Abs("../../../../deploy/docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range []string{"auth", "client-service"} {
		command := exec.CommandContext(ctx, "docker", "compose", "--env-file", envFile, "--project-name", project,
			"-f", composeFile, "-f", override, "up", "-d", "--no-deps", "--force-recreate", "--wait", service)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("recreate %s: %v\n%s", service, err, output)
		}
		// nginx refreshes Docker DNS asynchronously after a new container IP.
		deadline := time.Now().Add(35 * time.Second)
		for {
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/internal/"+strings.TrimSuffix(service, "-service")+"/readyz", nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := plain.Do(r)
			ready := false
			if err == nil {
				ready = response.StatusCode == 204
				_ = response.Body.Close()
			}
			if ready {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("gateway did not recover after container recreation")
			}
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
	}
	after, _ := request(plain, http.MethodGet, "/api/.well-known/jwks.json", "", nil, 200)
	if !bytes.Equal(before, after) {
		t.Fatal("auth signing key changed after container recreation")
	}
	request(firstClient, http.MethodGet, "/api/auth/me", firstAccess, nil, 200)
	request(firstClient, http.MethodGet, "/api/client/company", firstAccess, nil, 200)
	_, headers := request(firstClient, http.MethodPost, "/api/auth/refresh", "", nil, 200)
	if headers.Get("Set-Cookie") != "" {
		t.Fatal("refresh rotated its cookie after restart")
	}
	request(firstClient, http.MethodPost, "/api/auth/logout", "", nil, 204)
	request(firstClient, http.MethodPost, "/api/auth/refresh", "", nil, 401)
}
