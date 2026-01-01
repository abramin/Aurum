package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/postgres/sqlc"
)

// IdempotencyStore persists idempotency keys for request de-duplication.
// It scopes lookups by tenant_id to maintain tenant isolation.
type IdempotencyStore struct {
	queries *sqlc.Queries
}

// NewIdempotencyStore binds sqlc queries to a database handle (pool or tx).
// Callers control transactional scope by passing a pgx.Tx when participating in a unit of work.
func NewIdempotencyStore(db sqlc.DBTX) *IdempotencyStore {
	return &IdempotencyStore{queries: sqlc.New(db)}
}

// Get retrieves an idempotency entry by tenant and key.
// Returns (nil, nil) when no entry exists; absence is not treated as an error.
// Errors: returns database errors on failure and domain.ErrCorruptData on invalid stored timestamps.
func (s *IdempotencyStore) Get(ctx context.Context, tenantID types.TenantID, key string) (*domain.IdempotencyEntry, error) {
	row, err := s.queries.GetIdempotencyEntry(ctx, sqlc.GetIdempotencyEntryParams{
		TenantID:       tenantID.String(),
		IdempotencyKey: key,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	createdAt, err := timestamptzToTime(row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid created_at: %v", domain.ErrCorruptData, err)
	}

	return &domain.IdempotencyEntry{
		TenantID:       types.TenantID(row.TenantID),
		IdempotencyKey: row.IdempotencyKey,
		ResourceID:     row.ResourceID,
		StatusCode:     int(row.StatusCode),
		ResponseBody:   row.ResponseBody,
		CreatedAt:      createdAt,
	}, nil
}

// Set stores an idempotency entry, overwriting any existing entry for the key.
// It upserts on (tenant_id, idempotency_key) and overwrites the stored response payload.
// Side effects: writes to spending.idempotency_keys.
// Errors: returns database errors on failure.
func (s *IdempotencyStore) Set(ctx context.Context, entry *domain.IdempotencyEntry) error {
	return s.queries.UpsertIdempotencyEntry(ctx, sqlc.UpsertIdempotencyEntryParams{
		TenantID:       entry.TenantID.String(),
		IdempotencyKey: entry.IdempotencyKey,
		ResourceID:     entry.ResourceID,
		StatusCode:     int32(entry.StatusCode),
		ResponseBody:   entry.ResponseBody,
		CreatedAt:      timeToTimestamptz(entry.CreatedAt),
	})
}

// SetIfAbsent atomically stores an entry if no entry exists for the tenant/key.
// It uses a single-statement CTE to attempt insert and return the existing row.
// Side effects: writes to spending.idempotency_keys when the insert wins.
// Returns (true, entry, nil) if inserted, or (false, existing, nil) if already present.
// Concurrency: safe under concurrent callers; only one insert should win.
func (s *IdempotencyStore) SetIfAbsent(ctx context.Context, entry *domain.IdempotencyEntry) (bool, *domain.IdempotencyEntry, error) {
	row, err := s.queries.InsertIdempotencyIfAbsent(ctx, sqlc.InsertIdempotencyIfAbsentParams{
		TenantID:       entry.TenantID.String(),
		IdempotencyKey: entry.IdempotencyKey,
		ResourceID:     entry.ResourceID,
		StatusCode:     int32(entry.StatusCode),
		ResponseBody:   entry.ResponseBody,
		CreatedAt:      timeToTimestamptz(entry.CreatedAt),
	})
	if err != nil {
		return false, nil, err
	}

	createdAt, err := timestamptzToTime(row.CreatedAt)
	if err != nil {
		return false, nil, fmt.Errorf("%w: invalid created_at: %v", domain.ErrCorruptData, err)
	}

	return row.Inserted, &domain.IdempotencyEntry{
		TenantID:       types.TenantID(row.TenantID),
		IdempotencyKey: row.IdempotencyKey,
		ResourceID:     row.ResourceID,
		StatusCode:     int(row.StatusCode),
		ResponseBody:   row.ResponseBody,
		CreatedAt:      createdAt,
	}, nil
}

// Verify interface implementation.
var _ domain.IdempotencyStore = (*IdempotencyStore)(nil)
