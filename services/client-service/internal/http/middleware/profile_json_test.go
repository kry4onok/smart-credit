package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileJSONPresenceAndUnknownFields(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"inn":"990000000115","name":"ИП","okved":"47.11","registered_at":"2020-01-01","profile_schema_version":2}`, 204},
		{`{"inn":"990000000115","name":"ИП","okved":"47.11","registered_at":"2020-01-01","profit_12m":{"amount_kopecks":0,"currency":"RUB"}}`, 204},
		{`{"profit_12m":null}`, 422},
		{`{"profit_12m":{"currency":"RUB"}}`, 422},
		{`{"verified":true}`, 400},
		{`{"registry_checked":true}`, 400},
		{`{} {}`, 400},
	} {
		t.Run(tc.body, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/client/company", strings.NewReader(tc.body))
			ProfileJSON(next).ServeHTTP(recorder, request)
			if recorder.Code != tc.want {
				t.Fatalf("status=%d want=%d %s", recorder.Code, tc.want, recorder.Body.String())
			}
		})
	}
}
