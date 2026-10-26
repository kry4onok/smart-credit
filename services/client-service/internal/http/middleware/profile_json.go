package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
)

// ProfileJSON preserves the contract distinction between an omitted value and null.
// It also rejects client-supplied verification flags before generated decoding can ignore them.
func ProfileJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var target any
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/client/company":
			target = &common.CompanyProfile{}
		case r.Method == http.MethodPatch && len(parts) == 3 && parts[0] == "client" && parts[1] == "applications":
			target = &generated.ApplicationFormPatch{}
		case r.Method == http.MethodPost && len(parts) == 4 && parts[0] == "client" && parts[1] == "applications" && parts[3] == "info":
			target = &generated.ProvideInfoRequest{}
		default:
			next.ServeHTTP(w, r)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024))
		_ = r.Body.Close()
		if err != nil {
			domainerror.Write(w, domainerror.Problem(r.Context(), 400, "BAD_REQUEST", "Некорректное или слишком большое тело запроса."))
			return
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(target); err != nil {
			domainerror.Write(w, domainerror.Problem(r.Context(), 400, "BAD_REQUEST", "Некорректные или неизвестные поля запроса."))
			return
		}
		var extra any
		if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			domainerror.Write(w, domainerror.Problem(r.Context(), 400, "BAD_REQUEST", "Ожидается один JSON-объект."))
			return
		}
		var tree any
		if err = json.Unmarshal(raw, &tree); err != nil || invalidPresence(tree) {
			domainerror.Write(w, domainerror.Problem(r.Context(), 422, "INVALID_PROFILE", "Пропустите неизвестное значение вместо null; для суммы укажите amount_kopecks и currency."))
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		next.ServeHTTP(w, r)
	})
}
func invalidPresence(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case map[string]any:
		for key, item := range v {
			if key == "revenue_12m" || key == "profit_12m" || key == "obligations" || key == "amount" {
				m, ok := item.(map[string]any)
				if !ok {
					return true
				}
				if _, ok := m["amount_kopecks"]; !ok {
					return true
				}
				if _, ok := m["currency"]; !ok {
					return true
				}
			}
			if invalidPresence(item) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if invalidPresence(item) {
				return true
			}
		}
	}
	return false
}
