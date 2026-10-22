package authmiddleware

import (
	"net/http"

	"github.com/google/uuid"
)

// DemoAnalyst supplies a fixed identity only when explicitly configured for a
// local demo. It must not be enabled in a deployed environment.
func DemoAnalyst(next http.Handler, analystID uuid.UUID) http.Handler {
	if analystID == uuid.Nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(WithAnalystID(r.Context(), analystID)))
	})
}
