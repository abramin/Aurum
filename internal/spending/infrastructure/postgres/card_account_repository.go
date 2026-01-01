package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

// CardAccountRepository implements domain.CardAccountRepository using PostgreSQL.
type CardAccountRepository struct {
	db Executor
}

// NewCardAccountRepository creates a new CardAccountRepository.
func NewCardAccountRepository(db Executor) *CardAccountRepository {
	return &CardAccountRepository{db: db}
}

// Save persists a card account to the database.
// Uses optimistic locking via version column to prevent concurrent modification conflicts.
func (r *CardAccountRepository) Save(ctx context.Context, account *domain.CardAccount) error {
	// Check if account already exists
	var existingVersion int
	err := r.db.QueryRow(ctx,
		`SELECT version FROM spending.card_accounts WHERE id = $1 AND tenant_id = $2`,
		account.ID().String(), account.TenantID().String(),
	).Scan(&existingVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		// Insert new account
		_, err = r.db.Exec(ctx, `
			INSERT INTO spending.card_accounts (
				id, tenant_id,
				spending_limit_amount, spending_limit_currency,
				rolling_spend_amount, rolling_spend_currency,
				version, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			account.ID().String(),
			account.TenantID().String(),
			account.SpendingLimit().Amount,
			account.SpendingLimit().Currency,
			account.RollingSpend().Amount,
			account.RollingSpend().Currency,
			account.Version(),
			account.CreatedAt(),
			account.UpdatedAt(),
		)
		return err
	}
	if err != nil {
		return err
	}

	// Update existing account with optimistic locking
	tag, err := r.db.Exec(ctx, `
		UPDATE spending.card_accounts
		SET rolling_spend_amount = $1,
			rolling_spend_currency = $2,
			version = $3,
			updated_at = $4
		WHERE id = $5 AND tenant_id = $6 AND version = $7`,
		account.RollingSpend().Amount,
		account.RollingSpend().Currency,
		account.Version(),
		account.UpdatedAt(),
		account.ID().String(),
		account.TenantID().String(),
		existingVersion,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

// FindByID retrieves a card account by ID.
func (r *CardAccountRepository) FindByID(ctx context.Context, tenantID types.TenantID, id domain.CardAccountID) (*domain.CardAccount, error) {
	return r.findOne(ctx, `
		SELECT id, tenant_id,
			   spending_limit_amount, spending_limit_currency,
			   rolling_spend_amount, rolling_spend_currency,
			   version, created_at, updated_at
		FROM spending.card_accounts
		WHERE id = $1 AND tenant_id = $2`,
		id.String(), tenantID.String(),
	)
}

// FindByTenantID retrieves the card account for a tenant.
func (r *CardAccountRepository) FindByTenantID(ctx context.Context, tenantID types.TenantID) (*domain.CardAccount, error) {
	return r.findOne(ctx, `
		SELECT id, tenant_id,
			   spending_limit_amount, spending_limit_currency,
			   rolling_spend_amount, rolling_spend_currency,
			   version, created_at, updated_at
		FROM spending.card_accounts
		WHERE tenant_id = $1
		LIMIT 1`,
		tenantID.String(),
	)
}

func (r *CardAccountRepository) findOne(ctx context.Context, query string, args ...any) (*domain.CardAccount, error) {
	var (
		accountID     string
		tenant        string
		limitAmount   decimal.Decimal
		limitCurrency string
		spendAmount   decimal.Decimal
		spendCurrency string
		version       int
		createdAt     time.Time
		updatedAt     time.Time
	)

	err := r.db.QueryRow(ctx, query, args...).Scan(
		&accountID, &tenant,
		&limitAmount, &limitCurrency,
		&spendAmount, &spendCurrency,
		&version, &createdAt, &updatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCardAccountNotFound
	}
	if err != nil {
		return nil, err
	}

	parsedID, _ := domain.ParseCardAccountID(accountID)

	return domain.ReconstructCardAccount(
		parsedID,
		types.TenantID(tenant),
		types.NewMoney(limitAmount, limitCurrency),
		types.NewMoney(spendAmount, spendCurrency),
		version,
		createdAt,
		updatedAt,
	), nil
}

// Verify interface implementation.
var _ domain.CardAccountRepository = (*CardAccountRepository)(nil)
