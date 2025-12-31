package domain

import (
	"context"

	vo "aurum/internal/common/value_objects"
)

// AuthorizationRepository defines the interface for authorization persistence.
type AuthorizationRepository interface {
	Save(ctx context.Context, auth *Authorization) error
	FindByID(ctx context.Context, tenantID vo.TenantID, id AuthorizationID) (*Authorization, error)
}

// CardAccountRepository defines the interface for card account persistence.
type CardAccountRepository interface {
	Save(ctx context.Context, account *CardAccount) error
	FindByID(ctx context.Context, tenantID vo.TenantID, id CardAccountID) (*CardAccount, error)
	FindByTenantID(ctx context.Context, tenantID vo.TenantID) (*CardAccount, error)
}

// IdempotencyStore defines the interface for idempotency key storage.
type IdempotencyStore interface {
	// Get retrieves a stored response for the given idempotency key.
	// Returns nil if no entry exists.
	Get(ctx context.Context, tenantID vo.TenantID, key string) (*IdempotencyEntry, error)
	// Set stores a response for the given idempotency key.
	Set(ctx context.Context, entry *IdempotencyEntry) error
	// SetIfAbsent atomically stores a response if no entry exists for the key.
	// Returns true if the entry was created, false if it already existed.
	SetIfAbsent(ctx context.Context, entry *IdempotencyEntry) (created bool, existing *IdempotencyEntry, err error)
}

// IdempotencyEntry represents a stored idempotent response.
type IdempotencyEntry struct {
	TenantID       vo.TenantID
	IdempotencyKey string
	ResourceID     string
	StatusCode     int
	ResponseBody   []byte
}

// UnitOfWork defines an interface for transactional operations across repositories.
// It ensures all-or-nothing semantics for multi-aggregate operations.
type UnitOfWork interface {
	// Begin starts a new transaction and returns a Transaction handle.
	Begin(ctx context.Context) (Transaction, error)
}

// Transaction represents an in-flight transaction with staged changes.
type Transaction interface {
	// AuthorizationRepo returns a transactional authorization repository.
	AuthorizationRepo() AuthorizationRepository
	// CardAccountRepo returns a transactional card account repository.
	CardAccountRepo() CardAccountRepository
	// Commit atomically applies all staged changes.
	Commit() error
	// Rollback discards all staged changes.
	Rollback() error
}
