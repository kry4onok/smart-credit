package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	HTTP             HTTPConfig
	Database         DatabaseConfig
	DemoAnalystID    string        `env:"BANK_DEMO_ANALYST_ID"`
	ScoringTarget    string        `env:"BANK_SCORING_GRPC_ADDR"`
	BureauTarget     string        `env:"BANK_BUREAU_GRPC_ADDR"`
	AssistantTarget  string        `env:"BANK_ASSISTANT_GRPC_ADDR"`
	KafkaBrokers     string        `env:"BANK_KAFKA_BROKERS" env-default:"127.0.0.1:9092"`
	KafkaGroup       string        `env:"BANK_KAFKA_CONSUMER_GROUP" env-default:"bank-service"`
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
	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("BANK_DATABASE_URL is required")
	}
	if cfg.DemoAnalystID != "" {
		id, err := uuid.Parse(cfg.DemoAnalystID)
		if err != nil || id == uuid.Nil {
			return Config{}, fmt.Errorf("BANK_DEMO_ANALYST_ID must be a nonzero UUID")
		}
	}
	if strings.TrimSpace(cfg.KafkaGroup) == "" {
		return Config{}, fmt.Errorf("BANK_KAFKA_CONSUMER_GROUP is required")
	}
	for _, broker := range strings.Split(cfg.KafkaBrokers, ",") {
		if strings.TrimSpace(broker) == "" {
			return Config{}, fmt.Errorf("BANK_KAFKA_BROKERS must be a comma-separated list of addresses")
		}
		_, port, err := net.SplitHostPort(strings.TrimSpace(broker))
		if err != nil {
			return Config{}, fmt.Errorf("BANK_KAFKA_BROKERS must contain host:port addresses: %w", err)
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return Config{}, fmt.Errorf("BANK_KAFKA_BROKERS must contain valid ports")
		}
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
