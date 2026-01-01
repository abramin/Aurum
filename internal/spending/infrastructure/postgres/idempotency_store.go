package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

// IdempotencyStore implements domain.IdempotencyStore using PostgreSQL.
type IdempotencyStore struct {
	db Executor
}

// NewIdempotencyStore creates a new IdempotencyStore.
func NewIdempotencyStore(db Executor) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

// Get retrieves an idempotency entry by key.
func (s *IdempotencyStore) Get(ctx context.Context, tenantID types.TenantID, key string) (*domain.IdempotencyEntry, error) {
	var (
		tenant       string
		idempKey     string
		resourceID   string
		statusCode   int
		responseBody []byte
		createdAt    time.Time
	)

	err := s.db.QueryRow(ctx, `
		SELECT tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
		FROM spending.idempotency_keys
		WHERE tenant_id = $1 AND idempotency_key = $2`,
		tenantID.String(), key,
	).Scan(&tenant, &idempKey, &resourceID, &statusCode, &responseBody, &createdAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // Not found is not an error for idempotency checks
	}
	if err != nil {
		return nil, err
	}

	return &domain.IdempotencyEntry{
		TenantID:       types.TenantID(tenant),
		IdempotencyKey: idempKey,
		ResourceID:     resourceID,
		StatusCode:     statusCode,
		ResponseBody:   responseBody,
		CreatedAt:      createdAt,
	}, nil
}

// Set stores an idempotency entry.
func (s *IdempotencyStore) Set(ctx context.Context, entry *domain.IdempotencyEntry) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO spending.idempotency_keys (
			tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, idempotency_key) DO UPDATE SET
			resource_id = EXCLUDED.resource_id,
			status_code = EXCLUDED.status_code,
			response_body = EXCLUDED.response_body`,
		entry.TenantID.String(),
		entry.IdempotencyKey,
		entry.ResourceID,
		entry.StatusCode,
		entry.ResponseBody,
		entry.CreatedAt,
	)
	return err
}

// SetIfAbsent atomically stores an entry if no entry exists.
// Uses INSERT ... ON CONFLICT DO NOTHING to achieve atomicity.
func (s *IdempotencyStore) SetIfAbsent(ctx context.Context, entry *domain.IdempotencyEntry) (bool, *domain.IdempotencyEntry, error) {
	// Use INSERT ... ON CONFLICT DO NOTHING with RETURNING to detect insert vs skip
	var (
		tenant       string
		idempKey     string
		resourceID   string
		statusCode   int
		responseBody []byte
		createdAt    time.Time
	)

	err := s.db.QueryRow(ctx, `
		INSERT INTO spending.idempotency_keys (
			tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
		RETURNING tenant_id, idempotency_key, resource_id, status_code, response_body, created_at`,
		entry.TenantID.String(),
		entry.IdempotencyKey,
		entry.ResourceID,
		entry.StatusCode,
		entry.ResponseBody,
		entry.CreatedAt,
	).Scan(&tenant, &idempKey, &resourceID, &statusCode, &responseBody, &createdAt)

	if errors.Is(err, pgx.ErrNoRows) {
		// Row already existed - fetch it
		existing, err := s.Get(ctx, entry.TenantID, entry.IdempotencyKey)
		if err != nil {
			return false, nil, err
		}
		return false, existing, nil
	}
	if err != nil {
		return false, nil, err
	}

	// Row was inserted
	return true, &domain.IdempotencyEntry{
		TenantID:       types.TenantID(tenant),
		IdempotencyKey: idempKey,
		ResourceID:     resourceID,
		StatusCode:     statusCode,
		ResponseBody:   responseBody,
		CreatedAt:      createdAt,
	}, nil
}

// Verify interface implementation.
var _ domain.IdempotencyStore = (*IdempotencyStore)(nil)
