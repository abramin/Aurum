package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"aurum/internal/spending/domain"
)

type DataStore struct {
	pool              *pgxpool.Pool
	authorizationRepo *AuthorizationRepository
	cardAccountRepo   *CardAccountRepository
	idempotencyStore  *IdempotencyStore
	outboxRepo        *OutboxRepository
}

// NewDataStore creates a new DataStore with the given connection pool.
func NewDataStore(pool *pgxpool.Pool) *DataStore {
	return &DataStore{
		pool:              pool,
		authorizationRepo: NewAuthorizationRepository(pool),
		cardAccountRepo:   NewCardAccountRepository(pool),
		idempotencyStore:  NewIdempotencyStore(pool),
		outboxRepo:        NewOutboxRepository(pool),
	}
}

// Authorizations returns the authorization repository.
func (ds *DataStore) Authorizations() domain.AuthorizationRepository {
	return ds.authorizationRepo
}

// CardAccounts returns the card account repository.
func (ds *DataStore) CardAccounts() domain.CardAccountRepository {
	return ds.cardAccountRepo
}

// IdempotencyStore returns the idempotency store.
func (ds *DataStore) IdempotencyStore() domain.IdempotencyStore {
	return ds.idempotencyStore
}

// Outbox returns the outbox repository.
func (ds *DataStore) Outbox() domain.OutboxRepository {
	return ds.outboxRepo
}

// withTx creates a new DataStore with transactional repositories.
// This is the key to the Atomic pattern - we create new repository instances
// that share the same transaction.
func (ds *DataStore) withTx(tx pgx.Tx) *DataStore {
	return &DataStore{
		pool:              ds.pool,
		authorizationRepo: NewAuthorizationRepository(tx),
		cardAccountRepo:   NewCardAccountRepository(tx),
		idempotencyStore:  NewIdempotencyStore(tx),
		outboxRepo:        NewOutboxRepository(tx),
	}
}

// Atomic executes the callback within a database transaction.
// If the callback returns nil, the transaction is committed.
// If the callback returns an error or panics, the transaction is rolled back.
//
// - The service is responsible for requesting an atomic operation with procedures defined in the callback
// - All concerns like commits and rollbacks are handled by the repository
func (ds *DataStore) Atomic(ctx context.Context, fn domain.AtomicCallback) (err error) {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Use defer to handle both errors and panics
	defer func() {
		if p := recover(); p != nil {
			// Rollback on panic
			_ = tx.Rollback(ctx)
			panic(p) // Re-throw the panic
		}
		if err != nil {
			// Rollback on error
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				err = fmt.Errorf("tx error: %v, rollback error: %v", err, rbErr)
			}
		} else {
			// Commit on success
			err = tx.Commit(ctx)
			if err != nil {
				err = fmt.Errorf("commit transaction: %w", err)
			}
		}
	}()

	// Create transactional DataStore and execute callback
	txDataStore := ds.withTx(tx)
	err = fn(txDataStore)
	return
}

// Verify interface implementations.
var (
	_ domain.AtomicExecutor = (*DataStore)(nil)
	_ domain.Repositories   = (*DataStore)(nil)
)
