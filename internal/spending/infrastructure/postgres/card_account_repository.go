package postgres

import (
	"context"
	"errors"
	"fmt"
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
// It uses an UPSERT with optimistic locking:
//   - Inserts when version == 1
//   - Updates only if the stored version matches (version - 1)
// Returns ErrOptimisticLock when a concurrent update wins the version check.
func (r *CardAccountRepository) Save(ctx context.Context, account *domain.CardAccount) error {
	// Use UPSERT pattern: INSERT ... ON CONFLICT DO UPDATE with version check
	// For new records (version=1): inserts successfully
	// For existing records: updates only if version matches (optimistic lock)
	tag, err := r.db.Exec(ctx, `
		INSERT INTO spending.card_accounts (
			id, tenant_id,
			spending_limit_amount, spending_limit_currency,
			rolling_spend_amount, rolling_spend_currency,
			version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			rolling_spend_amount = EXCLUDED.rolling_spend_amount,
			rolling_spend_currency = EXCLUDED.rolling_spend_currency,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at
		WHERE spending.card_accounts.version = EXCLUDED.version - 1`,
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
	if err != nil {
		return err
	}

	// For updates, if version didn't match, no rows affected
	// For inserts, version=1 so we expect 1 row
	// Check: version > 1 means update, and 0 rows = conflict
	if account.Version() > 1 && tag.RowsAffected() == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

// FindByID retrieves a card account by ID.
// It queries by tenant and ID, maps missing rows to ErrCardAccountNotFound,
// and reconstructs the aggregate from stored values.
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
// It loads the first matching row for the tenant and reconstructs the aggregate.
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

	parsedID, err := domain.ParseCardAccountID(accountID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid card_account_id %q: %v", domain.ErrCorruptData, accountID, err)
	}

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
