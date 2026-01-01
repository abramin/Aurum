package postgres_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}

	err = pool.Client.Ping()
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "17-alpine",
		Env: []string{
			"POSTGRES_USER=aurum",
			"POSTGRES_PASSWORD=aurum",
			"POSTGRES_DB=aurum",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	hostPort := resource.GetHostPort("5432/tcp")
	databaseURL := fmt.Sprintf("postgres://aurum:aurum@%s/aurum?sslmode=disable", hostPort)

	// Set a hard deadline for container startup
	resource.Expire(120)

	pool.MaxWait = 60 * time.Second
	if err := pool.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var poolErr error
		testPool, poolErr = pgxpool.New(ctx, databaseURL)
		if poolErr != nil {
			return poolErr
		}

		return testPool.Ping(ctx)
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	// Run migrations
	if err := runMigrations(context.Background(), testPool); err != nil {
		log.Fatalf("Could not run migrations: %s", err)
	}

	code := m.Run()

	testPool.Close()

	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		// 000001_init_schemas
		`CREATE SCHEMA IF NOT EXISTS spending;`,

		// 000002_spending_tables
		`CREATE TABLE spending.card_accounts (
			id UUID PRIMARY KEY,
			tenant_id VARCHAR(255) NOT NULL,
			spending_limit_amount DECIMAL(19, 4) NOT NULL,
			spending_limit_currency VARCHAR(3) NOT NULL,
			rolling_spend_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
			rolling_spend_currency VARCHAR(3) NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_spending_limit_positive CHECK (spending_limit_amount >= 0),
			CONSTRAINT chk_rolling_spend_non_negative CHECK (rolling_spend_amount >= 0),
			CONSTRAINT chk_currency_match CHECK (spending_limit_currency = rolling_spend_currency)
		);`,
		`CREATE INDEX idx_card_accounts_tenant ON spending.card_accounts(tenant_id);`,
		`CREATE TABLE spending.authorizations (
			id UUID PRIMARY KEY,
			tenant_id VARCHAR(255) NOT NULL,
			card_account_id UUID NOT NULL REFERENCES spending.card_accounts(id),
			authorized_amount DECIMAL(19, 4) NOT NULL,
			authorized_currency VARCHAR(3) NOT NULL,
			captured_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
			captured_currency VARCHAR(3) NOT NULL,
			merchant_ref VARCHAR(255),
			reference VARCHAR(255),
			state VARCHAR(20) NOT NULL DEFAULT 'authorized',
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_authorized_amount_positive CHECK (authorized_amount > 0),
			CONSTRAINT chk_captured_amount_non_negative CHECK (captured_amount >= 0),
			CONSTRAINT chk_state_valid CHECK (state IN ('authorized', 'captured', 'reversed', 'expired')),
			CONSTRAINT chk_auth_currency_match CHECK (authorized_currency = captured_currency)
		);`,
		`CREATE INDEX idx_authorizations_tenant ON spending.authorizations(tenant_id);`,
		`CREATE INDEX idx_authorizations_card_account ON spending.authorizations(card_account_id);`,
		`CREATE INDEX idx_authorizations_state ON spending.authorizations(state);`,
		`CREATE TABLE spending.idempotency_keys (
			tenant_id VARCHAR(255) NOT NULL,
			idempotency_key VARCHAR(255) NOT NULL,
			resource_id VARCHAR(255) NOT NULL,
			status_code INTEGER NOT NULL,
			response_body BYTEA,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (tenant_id, idempotency_key)
		);`,
		`CREATE TABLE spending.outbox (
			event_id VARCHAR(255) PRIMARY KEY,
			event_type VARCHAR(100) NOT NULL,
			tenant_id VARCHAR(255) NOT NULL,
			correlation_id VARCHAR(255),
			causation_id VARCHAR(255),
			payload JSONB NOT NULL,
			occurred_at TIMESTAMPTZ NOT NULL,
			published_at TIMESTAMPTZ,
			CONSTRAINT chk_event_type_not_empty CHECK (event_type <> '')
		);`,
		`CREATE INDEX idx_outbox_unpublished ON spending.outbox(occurred_at) WHERE published_at IS NULL;`,
		`CREATE INDEX idx_outbox_tenant ON spending.outbox(tenant_id);`,

		// 000003_performance_indexes
		`DROP INDEX IF EXISTS spending.idx_card_accounts_tenant;`,
		`CREATE UNIQUE INDEX idx_card_accounts_tenant_unique ON spending.card_accounts(tenant_id);`,
		`CREATE INDEX IF NOT EXISTS idx_authorizations_tenant_id ON spending.authorizations(tenant_id, id);`,
	}

	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration failed: %s: %w", sql[:min(50, len(sql))], err)
		}
	}

	return nil
}

func truncateTables(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		TRUNCATE spending.outbox, spending.authorizations, spending.idempotency_keys, spending.card_accounts CASCADE
	`)
	return err
}

func getTestPool() *pgxpool.Pool {
	return testPool
}
