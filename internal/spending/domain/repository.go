package domain

import (
	"context"
	"time"

	"aurum/internal/common/types"
)

// AuthorizationRepository defines the interface for authorization persistence.
type AuthorizationRepository interface {
	Save(ctx context.Context, auth *Authorization) error
	FindByID(ctx context.Context, tenantID types.TenantID, id AuthorizationID) (*Authorization, error)
}

// CardAccountRepository defines the interface for card account persistence.
type CardAccountRepository interface {
	Save(ctx context.Context, account *CardAccount) error
	FindByID(ctx context.Context, tenantID types.TenantID, id CardAccountID) (*CardAccount, error)
	FindByTenantID(ctx context.Context, tenantID types.TenantID) (*CardAccount, error)
}

// IdempotencyEntry represents a stored idempotency record.
type IdempotencyEntry struct {
	TenantID       types.TenantID
	IdempotencyKey string
	ResourceID     string
	StatusCode     int
	ResponseBody   []byte
	CreatedAt      time.Time
}

// IdempotencyStore defines the interface for idempotency key storage.
type IdempotencyStore interface {
	Get(ctx context.Context, tenantID types.TenantID, key string) (*IdempotencyEntry, error)
	Set(ctx context.Context, entry *IdempotencyEntry) error
	// SetIfAbsent atomically stores an entry if no entry exists.
	// Returns (true, entry, nil) if created, (false, existing, nil) if already exists.
	SetIfAbsent(ctx context.Context, entry *IdempotencyEntry) (created bool, existing *IdempotencyEntry, err error)
}

// Repositories provides access to all repositories within a transaction.
// This is used with the Atomic pattern to ensure all operations share the same transaction.
type Repositories interface {
	Authorizations() AuthorizationRepository
	CardAccounts() CardAccountRepository
	IdempotencyStore() IdempotencyStore
	Outbox() OutboxRepository
}

// AtomicCallback is the function signature for atomic operations.
// Any error returned will cause the transaction to be rolled back.
type AtomicCallback func(repos Repositories) error

// AtomicExecutor provides atomic transaction execution following the Qonto pattern.
// See: https://medium.com/qonto-way/transactions-in-go-hexagonal-architecture-f12c7a817a61
//
// The service is responsible for requesting an atomic operation with a set of
// procedures defined in the callback. All other concerns like commits and rollbacks
// are left for the repository to implement.
//
// Example usage:
//
//	err := executor.Atomic(ctx, func(repos Repositories) error {
//	    account, err := repos.CardAccounts().FindByTenantID(ctx, tenantID)
//	    if err != nil {
//	        return err
//	    }
//	    if err := account.AuthorizeAmount(amount); err != nil {
//	        return err
//	    }
//	    return repos.CardAccounts().Save(ctx, account)
//	})
type AtomicExecutor interface {
	// Atomic executes the callback within a database transaction.
	// If the callback returns nil, the transaction is committed.
	// If the callback returns an error, the transaction is rolled back.
	Atomic(ctx context.Context, fn AtomicCallback) error
}

// OutboxEntry represents a domain event waiting to be published.
type OutboxEntry struct {
	ID            types.EventID
	EventType     string
	TenantID      types.TenantID
	CorrelationID types.CorrelationID
	CausationID   types.CausationID
	Payload       []byte
	OccurredAt    time.Time
	PublishedAt   *time.Time
}

// OutboxRepository defines the interface for the outbox pattern.
// Events are written to the outbox within the same transaction as the domain changes,
// then published asynchronously by a separate process.
type OutboxRepository interface {
	// Append adds an event to the outbox.
	Append(ctx context.Context, entry *OutboxEntry) error
	// FetchUnpublished retrieves unpublished events for publishing.
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxEntry, error)
	// MarkPublished marks events as published.
	MarkPublished(ctx context.Context, ids []types.EventID) error
}
