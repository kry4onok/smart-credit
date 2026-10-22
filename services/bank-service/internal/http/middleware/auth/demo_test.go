package authmiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestDemoAnalystIsOptIn(t *testing.T) {
	id := uuid.New()
	check := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := AnalystID(r.Context())
		if got == id && ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})

	withoutDemo := httptest.NewRecorder()
	DemoAnalyst(check, uuid.Nil).ServeHTTP(withoutDemo, httptest.NewRequest(http.MethodGet, "/", nil))
	if withoutDemo.Code != http.StatusForbidden {
		t.Fatalf("without demo identity: status %d, want 403", withoutDemo.Code)
	}

	withDemo := httptest.NewRecorder()
	DemoAnalyst(check, id).ServeHTTP(withDemo, httptest.NewRequest(http.MethodGet, "/", nil))
	if withDemo.Code != http.StatusNoContent {
		t.Fatalf("with demo identity: status %d, want 204", withDemo.Code)
	}
}
