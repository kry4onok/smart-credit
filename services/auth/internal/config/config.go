package config

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	HTTP             HTTPConfig
	Database         DatabaseConfig
	JWT              JWTConfig
	Cookie           CookieConfig
	ReadinessTimeout time.Duration `env:"AUTH_READINESS_TIMEOUT" env-default:"1s" validate:"gt=0,ltcsfield=HTTP.WriteTimeout"`
	ShutdownTimeout  time.Duration `env:"AUTH_SHUTDOWN_TIMEOUT" env-default:"10s" validate:"gt=0"`
}

type HTTPConfig struct {
	Address           string        `env:"AUTH_HTTP_ADDR" env-default:":8082" validate:"required"`
	ReadHeaderTimeout time.Duration `env:"AUTH_HTTP_READ_HEADER_TIMEOUT" env-default:"5s" validate:"gt=0"`
	ReadTimeout       time.Duration `env:"AUTH_HTTP_READ_TIMEOUT" env-default:"30s" validate:"gt=0"`
	WriteTimeout      time.Duration `env:"AUTH_HTTP_WRITE_TIMEOUT" env-default:"30s" validate:"gt=0"`
	IdleTimeout       time.Duration `env:"AUTH_HTTP_IDLE_TIMEOUT" env-default:"60s" validate:"gt=0"`
	MaxHeaderBytes    int           `env:"AUTH_HTTP_MAX_HEADER_BYTES" env-default:"1048576" validate:"gt=0"`
}

type DatabaseConfig struct {
	URL               string        `env:"AUTH_DATABASE_URL" env-required:"true" validate:"required"`
	ConnectTimeout    time.Duration `env:"AUTH_DB_CONNECT_TIMEOUT" env-default:"5s" validate:"gt=0"`
	MaxConnections    int32         `env:"AUTH_DB_MAX_CONNS" env-default:"10" validate:"gt=0"`
	MinConnections    int32         `env:"AUTH_DB_MIN_CONNS" env-default:"1" validate:"gte=0,ltefield=MaxConnections"`
	MaxConnLifetime   time.Duration `env:"AUTH_DB_MAX_CONN_LIFETIME" env-default:"30m" validate:"gt=0"`
	MaxConnIdleTime   time.Duration `env:"AUTH_DB_MAX_CONN_IDLE_TIME" env-default:"5m" validate:"gt=0"`
	HealthCheckPeriod time.Duration `env:"AUTH_DB_HEALTH_CHECK_PERIOD" env-default:"1m" validate:"gt=0"`
}

type JWTConfig struct {
	Issuer         string        `env:"AUTH_JWT_ISSUER" env-default:"smart-credit-auth" validate:"required"`
	Audience       string        `env:"AUTH_JWT_AUDIENCE" env-default:"smart-credit" validate:"required"`
	AccessTTL      time.Duration `env:"AUTH_JWT_ACCESS_TTL" env-default:"15m" validate:"gt=0"`
	KeyID          string        `env:"AUTH_JWT_KEY_ID" env-default:"auth-v1" validate:"required"`
	PrivateKeyPath string        `env:"AUTH_JWT_PRIVATE_KEY_PATH" env-default:".data/auth/private.pem" validate:"required"`
}

type CookieConfig struct {
	Name   string `env:"AUTH_REFRESH_COOKIE_NAME" env-default:"smart_credit_refresh" validate:"required"`
	Path   string `env:"AUTH_REFRESH_COOKIE_PATH" env-default:"/api/auth" validate:"required"`
	Secure bool   `env:"AUTH_REFRESH_COOKIE_SECURE" env-default:"false"`
}

func Load() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, fmt.Errorf("read environment: %w", err)
	}
	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("AUTH_DATABASE_URL is required")
	}
	if err := validator.New(validator.WithRequiredStructEnabled()).Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	_, port, err := net.SplitHostPort(cfg.HTTP.Address)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("AUTH_HTTP_ADDR must have host:port form with a valid port")
	}
	parsed, err := url.Parse(cfg.Database.URL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" || parsed.Path == "" || parsed.Path == "/" {
		return Config{}, fmt.Errorf("AUTH_DATABASE_URL must be a postgres URL with host and database name")
	}
	for _, field := range []struct{ name, value string }{
		{"AUTH_JWT_ISSUER", cfg.JWT.Issuer}, {"AUTH_JWT_AUDIENCE", cfg.JWT.Audience},
		{"AUTH_JWT_KEY_ID", cfg.JWT.KeyID}, {"AUTH_JWT_PRIVATE_KEY_PATH", cfg.JWT.PrivateKeyPath},
	} {
		if strings.TrimSpace(field.value) == "" {
			return Config{}, fmt.Errorf("%s must not be blank", field.name)
		}
	}
	if cfg.JWT.AccessTTL < time.Second {
		return Config{}, fmt.Errorf("AUTH_JWT_ACCESS_TTL must be at least one second")
	}
	if !strings.HasPrefix(cfg.Cookie.Path, "/") {
		return Config{}, fmt.Errorf("AUTH_REFRESH_COOKIE_PATH must start with /")
	}
	cookie := http.Cookie{Name: cfg.Cookie.Name, Path: cfg.Cookie.Path, Secure: cfg.Cookie.Secure}
	if err := cookie.Valid(); err != nil {
		return Config{}, fmt.Errorf("invalid refresh cookie configuration: %w", err)
	}
	if strings.HasPrefix(cfg.Cookie.Name, "__Secure-") && !cfg.Cookie.Secure {
		return Config{}, fmt.Errorf("AUTH_REFRESH_COOKIE_SECURE is required for __Secure- cookies")
	}
	if strings.HasPrefix(cfg.Cookie.Name, "__Host-") && (!cfg.Cookie.Secure || cfg.Cookie.Path != "/") {
		return Config{}, fmt.Errorf("__Host- cookies require AUTH_REFRESH_COOKIE_SECURE and path /")
	}
	return cfg, nil
}
