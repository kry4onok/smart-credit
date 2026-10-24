package container

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

func currentUserRequest(c *Container, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/auth/me?user_id="+uuid.NewString()+"&org_id="+uuid.NewString(), nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	c.HTTPHandler.ServeHTTP(w, r)
	return w
}

func currentUserResponse(t *testing.T, w *httptest.ResponseRecorder) generated.CurrentUser {
	t.Helper()
	checkStatus(t, w, 200)
	var user generated.CurrentUser
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"password", "is_active", "token", "revoked_at"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("current user response disclosed internal fields")
		}
	}
	return user
}

func TestCurrentUserIntegration(t *testing.T) {
	db, dsn := sessionDatabase(t)
	c := sessionContainer(t, dsn)
	for _, role := range []string{model.RoleClient, model.RoleAnalyst, model.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			p := addUser(t, db, c, role, true)
			cookie, login := loginUser(t, c, p)
			var access generated.AccessTokenResponse
			if err := json.Unmarshal(login.Body.Bytes(), &access); err != nil {
				t.Fatal(err)
			}
			w := currentUserRequest(c, access.AccessToken)
			user := currentUserResponse(t, w)
			if user.Id != p.UserID || string(user.Email) != p.UserID.String()+"@example.test" || user.FullName != "Тест" || string(user.Role) != role || user.CanSubmit {
				t.Fatal("current user does not match the database")
			}
			if role == model.RoleClient {
				if user.Organization == nil || user.Organization.Id != p.OrgID || user.Organization.Inn != "7707083893" || user.Organization.Name != "Тест" {
					t.Fatal("wrong client organization")
				}
				if _, err := db.Exec(t.Context(), `UPDATE users SET can_submit=true, full_name='Новое имя' WHERE id=$1`, p.UserID); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(t.Context(), `UPDATE organizations SET name='Новая компания' WHERE id=$1`, p.OrgID); err != nil {
					t.Fatal(err)
				}
				updated := currentUserResponse(t, currentUserRequest(c, access.AccessToken))
				if !updated.CanSubmit || updated.FullName != "Новое имя" || updated.Organization.Name != "Новая компания" {
					t.Fatal("current user returned stale profile or permissions")
				}
				if _, err := db.Exec(t.Context(), `UPDATE users SET role='ADMIN', org_id=NULL, can_submit=false WHERE id=$1`, p.UserID); err != nil {
					t.Fatal(err)
				}
				updated = currentUserResponse(t, currentUserRequest(c, access.AccessToken))
				if string(updated.Role) != model.RoleAdmin || updated.Organization != nil || updated.CanSubmit {
					t.Fatal("current user trusted stale JWT role or organization")
				}
			} else if user.Organization != nil {
				t.Fatal("staff profile contains an organization")
			}
			checkStatus(t, sessionRequest(c, "/auth/logout", "", cookie), 204)
			// Logout revokes refresh, while an issued access JWT keeps its own TTL.
			checkStatus(t, currentUserRequest(c, access.AccessToken), 200)
			if _, err := db.Exec(t.Context(), `UPDATE users SET is_active=false WHERE id=$1`, p.UserID); err != nil {
				t.Fatal(err)
			}
			checkStatus(t, currentUserRequest(c, access.AccessToken), 401)
			if _, err := db.Exec(t.Context(), `DELETE FROM refresh_tokens WHERE user_id=$1`, p.UserID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(t.Context(), `DELETE FROM users WHERE id=$1`, p.UserID); err != nil {
				t.Fatal(err)
			}
			checkStatus(t, currentUserRequest(c, access.AccessToken), 401)
		})
	}
}
