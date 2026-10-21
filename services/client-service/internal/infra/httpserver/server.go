package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/config"
)

type Server struct {
	server *http.Server
}

func New(cfg config.HTTPConfig, handler http.Handler) *Server {
	return &Server{server: &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}}
}

func (s *Server) Run() error {
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return errors.Join(fmt.Errorf("shutdown HTTP: %w", err), s.server.Close())
	}
	return nil
}
