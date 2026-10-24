package middleware

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverClearsUncommittedResponseHeaders(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	for _, value := range []any{"private-value", map[string]string{"secret": "private-value"}, nil} {
		handler := Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Set-Cookie", "refresh=private-value")
			w.Header().Set("Content-Length", "500")
			w.Header().Set("Cache-Control", "public, max-age=300")
			panic(value)
		}))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != 500 || w.Header().Get("Set-Cookie") != "" || w.Header().Get("Content-Length") != "" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Trace-ID") == "" || strings.Contains(w.Body.String(), "private-value") {
			t.Fatal("panic kept unsafe response state")
		}
	}
}

func TestRecoverAbortsAlreadyCommittedResponse(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial response"))
		panic("private-value")
	}))
	w := httptest.NewRecorder()
	defer func() {
		if err, ok := recover().(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatal("committed response must abort the connection")
		}
		if w.Body.String() != "partial response" {
			t.Fatal("panic appended an invalid second response")
		}
	}()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
}
