package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration.
type Config struct {
	// Database
	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://aurum:aurum@localhost:5432/aurum?sslmode=disable"`

	// Connection pool settings (tune for expected concurrency)
	DBMaxConns        int `env:"DB_MAX_CONNS" envDefault:"25"`
	DBMinConns        int `env:"DB_MIN_CONNS" envDefault:"5"`
	DBMaxConnLifetime int `env:"DB_MAX_CONN_LIFETIME_MINS" envDefault:"30"` // minutes
	DBMaxConnIdleTime int `env:"DB_MAX_CONN_IDLE_MINS" envDefault:"5"`      // minutes

	// Kafka
	KafkaBrokers string `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`

	// HTTP Server
	Port int `env:"PORT" envDefault:"8080"`

	// Logging
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"` // "json" or "text"

	// Environment
	Environment string `env:"ENVIRONMENT" envDefault:"development"`
}

// Load loads configuration from environment variables.
// It first attempts to load from .env file if present.
func Load() (*Config, error) {
	// Load .env file if it exists (won't override existing env vars)
	if err := LoadEnvFileIfExists(".env"); err != nil {
		return nil, fmt.Errorf("loading .env file: %w", err)
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing environment: %w", err)
	}

	return cfg, nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}
