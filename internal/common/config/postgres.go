package config

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool creates a Postgres connection pool with the configured settings.
// Side effects: establishes network connections and pings the database.
// Use this function when wiring up Postgres-backed datastores.
func (c *Config) NewPostgresPool(ctx context.Context) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(c.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	// Apply pool settings from config
	poolConfig.MaxConns = int32(c.DBMaxConns)
	poolConfig.MinConns = int32(c.DBMinConns)
	poolConfig.MaxConnLifetime = time.Duration(c.DBMaxConnLifetime) * time.Minute
	poolConfig.MaxConnIdleTime = time.Duration(c.DBMaxConnIdleTime) * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}
