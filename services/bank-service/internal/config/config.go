package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddress      = ":8080"
	defaultShutdownTimeout  = 10 * time.Second
	defaultDBConnectTimeout = 5 * time.Second
	defaultDBMaxConnections = 10
	defaultDBMinConnections = 1
)

type Config struct {
	HTTPAddress     string
	Database        DatabaseConfig
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	URL            string
	ConnectTimeout time.Duration
	MaxConnections int32
	MinConnections int32
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress: valueOrDefault("BANK_HTTP_ADDR", defaultHTTPAddress),
		Database: DatabaseConfig{
			URL:            strings.TrimSpace(os.Getenv("BANK_DATABASE_URL")),
			ConnectTimeout: defaultDBConnectTimeout,
			MaxConnections: defaultDBMaxConnections,
			MinConnections: defaultDBMinConnections,
		},
		ShutdownTimeout: defaultShutdownTimeout,
	}

	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("BANK_DATABASE_URL is required")
	}
	if err := validateHTTPAddress(cfg.HTTPAddress); err != nil {
		return Config{}, fmt.Errorf("BANK_HTTP_ADDR: %w", err)
	}
	if err := validateDatabaseURL(cfg.Database.URL); err != nil {
		return Config{}, fmt.Errorf("BANK_DATABASE_URL: %w", err)
	}

	var err error
	cfg.Database.ConnectTimeout, err = durationValue("BANK_DB_CONNECT_TIMEOUT", defaultDBConnectTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.Database.MaxConnections, err = int32Value("BANK_DB_MAX_CONNS", defaultDBMaxConnections, 1)
	if err != nil {
		return Config{}, err
	}
	cfg.Database.MinConnections, err = int32Value("BANK_DB_MIN_CONNS", defaultDBMinConnections, 0)
	if err != nil {
		return Config{}, err
	}
	if cfg.Database.MinConnections > cfg.Database.MaxConnections {
		return Config{}, fmt.Errorf("BANK_DB_MIN_CONNS must not exceed BANK_DB_MAX_CONNS")
	}
	cfg.ShutdownTimeout, err = durationValue("BANK_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func durationValue(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}

func int32Value(name string, fallback int32, minimum int64) (int32, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < minimum {
		return 0, fmt.Errorf("%s must be an integer greater than or equal to %d", name, minimum)
	}
	return int32(value), nil
}

func validateHTTPAddress(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("must have host:port form")
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func validateDatabaseURL(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("must be a valid URL")
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return fmt.Errorf("scheme must be postgres or postgresql")
	}
	if parsed.Host == "" {
		return fmt.Errorf("host is required")
	}
	if strings.Trim(parsed.Path, "/") == "" {
		return fmt.Errorf("database name is required")
	}
	return nil
}
