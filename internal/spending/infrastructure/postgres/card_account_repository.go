package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/postgres/sqlc"
)

// CardAccountRepository persists CardAccount aggregates using PostgreSQL.
// It scopes reads by tenant_id and writes the tenant_id from the aggregate to enforce tenant isolation.
type CardAccountRepository struct {
	queries *sqlc.Queries
}

// NewCardAccountRepository binds sqlc queries to a database handle (pool or tx).
// Callers control transactional scope by passing a pgx.Tx when participating in a unit of work.
func NewCardAccountRepository(db sqlc.DBTX) *CardAccountRepository {
	return &CardAccountRepository{queries: sqlc.New(db)}
}

// Save persists a CardAccount aggregate to the database.
// It uses an UPSERT with optimistic locking:
//   - Inserts when version == 1
//   - Updates only if the stored version matches (version - 1)
//
// Side effects: writes to spending.card_accounts.
// Errors: returns domain.ErrOptimisticLock on version conflict; returns database errors on failure.
// Concurrency: uses optimistic locking via the version check.
func (r *CardAccountRepository) Save(ctx context.Context, account *domain.CardAccount) error {
	rows, err := r.queries.UpsertCardAccount(ctx, sqlc.UpsertCardAccountParams{
		ID:                    uuid.UUID(account.ID()),
		TenantID:              account.TenantID().String(),
		SpendingLimitAmount:   decimalToNumeric(account.SpendingLimit().Amount),
		SpendingLimitCurrency: account.SpendingLimit().Currency,
		RollingSpendAmount:    decimalToNumeric(account.RollingSpend().Amount),
		RollingSpendCurrency:  account.RollingSpend().Currency,
		Version:               int32(account.Version()),
		CreatedAt:             timeToTimestamptz(account.CreatedAt()),
		UpdatedAt:             timeToTimestamptz(account.UpdatedAt()),
	})
	if err != nil {
		return err
	}

	// For updates, if version didn't match, no rows affected.
	// For inserts, version=1 so we expect 1 row.
	// Check: version > 1 means update, and 0 rows = conflict.
	if account.Version() > 1 && rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

// FindByID retrieves a CardAccount aggregate by ID.
// It queries by tenant and ID, maps missing rows to ErrCardAccountNotFound,
// and reconstructs the aggregate from stored values.
// Errors: returns domain.ErrCardAccountNotFound when missing and domain.ErrCorruptData
// when stored values violate invariants or cannot be decoded.
func (r *CardAccountRepository) FindByID(ctx context.Context, tenantID types.TenantID, id domain.CardAccountID) (*domain.CardAccount, error) {
	row, err := r.queries.GetCardAccountByID(ctx, sqlc.GetCardAccountByIDParams{
		ID:       uuid.UUID(id),
		TenantID: tenantID.String(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCardAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return mapCardAccount(row)
}

// FindByTenantID retrieves the CardAccount aggregate for a tenant.
// It loads the first matching row for the tenant and reconstructs the aggregate.
// Assumes the schema enforces at most one account per tenant.
func (r *CardAccountRepository) FindByTenantID(ctx context.Context, tenantID types.TenantID) (*domain.CardAccount, error) {
	row, err := r.queries.GetCardAccountByTenantID(ctx, tenantID.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCardAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return mapCardAccount(row)
}

func mapCardAccount(row sqlc.SpendingCardAccount) (*domain.CardAccount, error) {
	limitAmount, err := numericToDecimal(row.SpendingLimitAmount)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid spending_limit_amount: %v", domain.ErrCorruptData, err)
	}
	rollingAmount, err := numericToDecimal(row.RollingSpendAmount)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid rolling_spend_amount: %v", domain.ErrCorruptData, err)
	}
	createdAt, err := timestamptzToTime(row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid created_at: %v", domain.ErrCorruptData, err)
	}
	updatedAt, err := timestamptzToTime(row.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid updated_at: %v", domain.ErrCorruptData, err)
	}

	return domain.ReconstructCardAccount(
		domain.CardAccountID(row.ID),
		types.TenantID(row.TenantID),
		types.NewMoney(limitAmount, row.SpendingLimitCurrency),
		types.NewMoney(rollingAmount, row.RollingSpendCurrency),
		int(row.Version),
		createdAt,
		updatedAt,
	), nil
}

// Verify interface implementation.
var _ domain.CardAccountRepository = (*CardAccountRepository)(nil)
