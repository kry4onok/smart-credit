package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func Write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func Problem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": code, "status": status, "code": code})
}
func Read(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}
func Version(r *http.Request) (int64, error) {
	s := r.Header.Get("If-Match")
	if len(s) < 3 || !strings.HasPrefix(s, `"`) || !strings.HasSuffix(s, `"`) {
		return 0, errors.New("IF_MATCH_REQUIRED")
	}
	v, e := strconv.ParseInt(s[1:len(s)-1], 10, 64)
	if e != nil || v < 1 {
		return 0, errors.New("INVALID_IF_MATCH")
	}
	return v, nil
}
