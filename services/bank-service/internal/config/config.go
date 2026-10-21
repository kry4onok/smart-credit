package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	HTTP             HTTPConfig
	Database         DatabaseConfig
	ReadinessTimeout time.Duration `env:"BANK_READINESS_TIMEOUT" env-required:"true" validate:"gt=0,ltcsfield=HTTP.WriteTimeout"`
	ShutdownTimeout  time.Duration `env:"BANK_SHUTDOWN_TIMEOUT" env-required:"true" validate:"gt=0"`
}

type HTTPConfig struct {
	Address           string        `env:"BANK_HTTP_ADDR" env-required:"true" validate:"required"`
	ReadHeaderTimeout time.Duration `env:"BANK_HTTP_READ_HEADER_TIMEOUT" env-required:"true" validate:"gt=0"`
	ReadTimeout       time.Duration `env:"BANK_HTTP_READ_TIMEOUT" env-required:"true" validate:"gt=0"`
	WriteTimeout      time.Duration `env:"BANK_HTTP_WRITE_TIMEOUT" env-required:"true" validate:"gt=0"`
	IdleTimeout       time.Duration `env:"BANK_HTTP_IDLE_TIMEOUT" env-required:"true" validate:"gt=0"`
	MaxHeaderBytes    int           `env:"BANK_HTTP_MAX_HEADER_BYTES" env-required:"true" validate:"gt=0"`
}

type DatabaseConfig struct {
	URL               string        `env:"BANK_DATABASE_URL" env-required:"true" validate:"required,url"`
	ConnectTimeout    time.Duration `env:"BANK_DB_CONNECT_TIMEOUT" env-required:"true" validate:"gt=0"`
	MaxConnections    int32         `env:"BANK_DB_MAX_CONNS" env-required:"true" validate:"gt=0"`
	MinConnections    int32         `env:"BANK_DB_MIN_CONNS" env-required:"true" validate:"gte=0,ltefield=MaxConnections"`
	MaxConnLifetime   time.Duration `env:"BANK_DB_MAX_CONN_LIFETIME" env-required:"true" validate:"gt=0"`
	MaxConnIdleTime   time.Duration `env:"BANK_DB_MAX_CONN_IDLE_TIME" env-required:"true" validate:"gt=0"`
	HealthCheckPeriod time.Duration `env:"BANK_DB_HEALTH_CHECK_PERIOD" env-required:"true" validate:"gt=0"`
}

func Load() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, fmt.Errorf("read environment: %w", err)
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	_, port, err := net.SplitHostPort(cfg.HTTP.Address)
	if err != nil {
		return Config{}, fmt.Errorf("BANK_HTTP_ADDR must have host:port form with a valid port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || validate.Var(portNumber, "required,port") != nil {
		return Config{}, fmt.Errorf("BANK_HTTP_ADDR must have host:port form with a valid port")
	}

	parsed, err := url.Parse(cfg.Database.URL)
	if err != nil ||
		(parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") ||
		parsed.Hostname() == "" ||
		parsed.Path == "" ||
		parsed.Path == "/" {
		return Config{}, fmt.Errorf("BANK_DATABASE_URL must be a postgres URL with host and database name")
	}

	return cfg, nil
}
