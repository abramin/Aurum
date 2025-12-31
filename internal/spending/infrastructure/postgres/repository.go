package postgres

import (
	"context"
	"errors"

	"aurum/internal/spending/domain"
	sqlc "aurum/internal/spending/infrastructure/postgres/sqlc"

	vo "aurum/internal/common/value_objects"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ErrNotFound is returned when an entity is not found.
var ErrNotFound = errors.New("entity not found")

// ErrOptimisticLock is returned when an optimistic lock conflict occurs.
var ErrOptimisticLock = errors.New("optimistic lock conflict")

// AuthorizationRepository implements domain.AuthorizationRepository using PostgreSQL.
type AuthorizationRepository struct {
	queries *sqlc.Queries
}

// NewAuthorizationRepository creates a new PostgreSQL authorization repository.
func NewAuthorizationRepository(db sqlc.DBTX) *AuthorizationRepository {
	return &AuthorizationRepository{
		queries: sqlc.New(db),
	}
}

// Save persists an authorization to the database.
func (r *AuthorizationRepository) Save(ctx context.Context, auth *domain.Authorization) error {
	authUUID := mustParseUUID(auth.ID().String())
	cardAccountUUID := mustParseUUID(auth.CardAccountID().String())

	// Try to get existing authorization to determine if this is an insert or update
	existing, err := r.queries.GetAuthorization(ctx, sqlc.GetAuthorizationParams{
		TenantID: auth.TenantID().String(),
		ID:       authUUID,
	})

	if errors.Is(err, pgx.ErrNoRows) {
		// Insert new authorization
		return r.queries.InsertAuthorization(ctx, sqlc.InsertAuthorizationParams{
			ID:                 authUUID,
			TenantID:           auth.TenantID().String(),
			CardAccountID:      cardAccountUUID,
			AuthorizedAmount:   decimalToNumeric(auth.AuthorizedAmount().Amount),
			AuthorizedCurrency: string(auth.AuthorizedAmount().Currency),
			CapturedAmount:     decimalToNumeric(auth.CapturedAmount().Amount),
			CapturedCurrency:   string(auth.CapturedAmount().Currency),
			State:              string(auth.State()),
		})
	}

	if err != nil {
		return err
	}

	// Update existing authorization with optimistic locking
	rowsAffected, err := r.queries.UpdateAuthorization(ctx, sqlc.UpdateAuthorizationParams{
		TenantID:       auth.TenantID().String(),
		ID:             authUUID,
		CapturedAmount: decimalToNumeric(auth.CapturedAmount().Amount),
		State:          string(auth.State()),
		Version:        existing.Version,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// FindByID retrieves an authorization by tenant and ID.
func (r *AuthorizationRepository) FindByID(ctx context.Context, tenantID vo.TenantID, id domain.AuthorizationID) (*domain.Authorization, error) {
	row, err := r.queries.GetAuthorization(ctx, sqlc.GetAuthorizationParams{
		TenantID: tenantID.String(),
		ID:       mustParseUUID(id.String()),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return rowToAuthorization(row)
}

// CardAccountRepository implements domain.CardAccountRepository using PostgreSQL.
type CardAccountRepository struct {
	queries *sqlc.Queries
}

// NewCardAccountRepository creates a new PostgreSQL card account repository.
func NewCardAccountRepository(db sqlc.DBTX) *CardAccountRepository {
	return &CardAccountRepository{
		queries: sqlc.New(db),
	}
}

// Save persists a card account to the database.
func (r *CardAccountRepository) Save(ctx context.Context, account *domain.CardAccount) error {
	accountUUID := mustParseUUID(account.ID().String())

	// Try to get existing account to determine if this is an insert or update
	existing, err := r.queries.GetCardAccount(ctx, sqlc.GetCardAccountParams{
		TenantID: account.TenantID().String(),
		ID:       accountUUID,
	})

	if errors.Is(err, pgx.ErrNoRows) {
		// Insert new card account
		return r.queries.InsertCardAccount(ctx, sqlc.InsertCardAccountParams{
			ID:                    accountUUID,
			TenantID:              account.TenantID().String(),
			SpendingLimitAmount:   decimalToNumeric(account.SpendingLimit().Amount),
			SpendingLimitCurrency: string(account.SpendingLimit().Currency),
			RollingSpendAmount:    decimalToNumeric(account.RollingSpend().Amount),
			RollingSpendCurrency:  string(account.RollingSpend().Currency),
		})
	}

	if err != nil {
		return err
	}

	// Update existing account with optimistic locking
	rowsAffected, err := r.queries.UpdateCardAccount(ctx, sqlc.UpdateCardAccountParams{
		TenantID:           account.TenantID().String(),
		ID:                 accountUUID,
		RollingSpendAmount: decimalToNumeric(account.RollingSpend().Amount),
		Version:            existing.Version,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// FindByID retrieves a card account by tenant and ID.
func (r *CardAccountRepository) FindByID(ctx context.Context, tenantID vo.TenantID, id domain.CardAccountID) (*domain.CardAccount, error) {
	row, err := r.queries.GetCardAccount(ctx, sqlc.GetCardAccountParams{
		TenantID: tenantID.String(),
		ID:       mustParseUUID(id.String()),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return rowToCardAccount(row)
}

// FindByTenantID retrieves a card account by tenant ID.
func (r *CardAccountRepository) FindByTenantID(ctx context.Context, tenantID vo.TenantID) (*domain.CardAccount, error) {
	row, err := r.queries.GetCardAccountByTenant(ctx, tenantID.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return rowToCardAccount(sqlc.GetCardAccountRow(row))
}

// IdempotencyStore implements domain.IdempotencyStore using PostgreSQL.
type IdempotencyStore struct {
	queries *sqlc.Queries
}

// NewIdempotencyStore creates a new PostgreSQL idempotency store.
func NewIdempotencyStore(db sqlc.DBTX) *IdempotencyStore {
	return &IdempotencyStore{
		queries: sqlc.New(db),
	}
}

// Get retrieves an idempotency entry by tenant and key.
func (s *IdempotencyStore) Get(ctx context.Context, tenantID vo.TenantID, key string) (*domain.IdempotencyEntry, error) {
	row, err := s.queries.GetIdempotencyKey(ctx, sqlc.GetIdempotencyKeyParams{
		TenantID:       tenantID.String(),
		IdempotencyKey: key,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &domain.IdempotencyEntry{
		TenantID:       tenantID,
		IdempotencyKey: row.IdempotencyKey,
		ResourceID:     row.ResourceID,
		StatusCode:     int(row.StatusCode),
		ResponseBody:   row.ResponseBody,
	}, nil
}

// Set stores an idempotency entry.
func (s *IdempotencyStore) Set(ctx context.Context, entry *domain.IdempotencyEntry) error {
	return s.queries.InsertIdempotencyKey(ctx, sqlc.InsertIdempotencyKeyParams{
		TenantID:       entry.TenantID.String(),
		IdempotencyKey: entry.IdempotencyKey,
		ResourceID:     entry.ResourceID,
		StatusCode:     int32(entry.StatusCode),
		ResponseBody:   entry.ResponseBody,
	})
}

// SetIfAbsent atomically stores an entry if no entry exists.
// Returns true if created, false if already existed.
func (s *IdempotencyStore) SetIfAbsent(ctx context.Context, entry *domain.IdempotencyEntry) (bool, *domain.IdempotencyEntry, error) {
	// Try to insert; ON CONFLICT DO NOTHING means no row returned if exists
	row, err := s.queries.InsertIdempotencyKeyIfAbsent(ctx, sqlc.InsertIdempotencyKeyIfAbsentParams{
		TenantID:       entry.TenantID.String(),
		IdempotencyKey: entry.IdempotencyKey,
		ResourceID:     entry.ResourceID,
		StatusCode:     int32(entry.StatusCode),
		ResponseBody:   entry.ResponseBody,
	})

	if errors.Is(err, pgx.ErrNoRows) {
		// Row already existed, fetch it
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
		TenantID:       entry.TenantID,
		IdempotencyKey: row.IdempotencyKey,
		ResourceID:     row.ResourceID,
		StatusCode:     int(row.StatusCode),
		ResponseBody:   row.ResponseBody,
	}, nil
}

// UnitOfWork implements domain.UnitOfWork using PostgreSQL transactions.
type UnitOfWork struct {
	pool *pgxpool.Pool
}

// NewUnitOfWork creates a new PostgreSQL unit of work.
func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{pool: pool}
}

// Begin starts a new transaction.
func (u *UnitOfWork) Begin(ctx context.Context) (domain.Transaction, error) {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &Transaction{tx: tx}, nil
}

// Transaction implements domain.Transaction using a PostgreSQL transaction.
type Transaction struct {
	tx pgx.Tx
}

// AuthorizationRepo returns a transactional authorization repository.
func (t *Transaction) AuthorizationRepo() domain.AuthorizationRepository {
	return NewAuthorizationRepository(t.tx)
}

// CardAccountRepo returns a transactional card account repository.
func (t *Transaction) CardAccountRepo() domain.CardAccountRepository {
	return NewCardAccountRepository(t.tx)
}

// Commit commits the transaction.
func (t *Transaction) Commit() error {
	return t.tx.Commit(context.Background())
}

// Rollback rolls back the transaction.
func (t *Transaction) Rollback() error {
	return t.tx.Rollback(context.Background())
}

// Helper functions for type conversion

func mustParseUUID(s string) uuid.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func decimalToNumeric(d decimal.Decimal) pgtype.Numeric {
	// Convert decimal.Decimal to pgtype.Numeric
	var num pgtype.Numeric
	_ = num.Scan(d.String())
	return num
}

func numericToDecimal(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid {
		return decimal.Zero
	}
	// Convert pgtype.Numeric to decimal.Decimal
	d, _ := decimal.NewFromString(numericToString(n))
	return d
}

func numericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0"
	}
	// pgtype.Numeric stores Int and Exp
	// value = Int * 10^Exp
	intVal := n.Int
	exp := n.Exp

	if intVal == nil {
		return "0"
	}

	str := intVal.String()
	if exp == 0 {
		return str
	}
	if exp > 0 {
		// Add trailing zeros
		for i := int32(0); i < exp; i++ {
			str += "0"
		}
		return str
	}
	// exp < 0: insert decimal point
	absExp := int(-exp)
	if len(str) <= absExp {
		// Need leading zeros after decimal
		zeros := absExp - len(str) + 1
		prefix := "0."
		for i := 0; i < zeros-1; i++ {
			prefix += "0"
		}
		return prefix + str
	}
	// Insert decimal point
	insertPos := len(str) - absExp
	return str[:insertPos] + "." + str[insertPos:]
}

func rowToAuthorization(row sqlc.GetAuthorizationRow) (*domain.Authorization, error) {
	authID, err := domain.ParseAuthorizationID(row.ID.String())
	if err != nil {
		return nil, err
	}

	cardAccountID, err := domain.ParseCardAccountID(row.CardAccountID.String())
	if err != nil {
		return nil, err
	}

	tenantID, err := vo.ParseTenantID(row.TenantID)
	if err != nil {
		return nil, err
	}

	authCurrency, err := vo.ParseCurrency(row.AuthorizedCurrency)
	if err != nil {
		return nil, err
	}

	capCurrency, err := vo.ParseCurrency(row.CapturedCurrency)
	if err != nil {
		return nil, err
	}

	return domain.ReconstructAuthorization(
		authID,
		tenantID,
		cardAccountID,
		vo.New(numericToDecimal(row.AuthorizedAmount), authCurrency),
		vo.New(numericToDecimal(row.CapturedAmount), capCurrency),
		domain.AuthorizationState(row.State),
	), nil
}

func rowToCardAccount(row sqlc.GetCardAccountRow) (*domain.CardAccount, error) {
	id, err := domain.ParseCardAccountID(row.ID.String())
	if err != nil {
		return nil, err
	}

	tenantID, err := vo.ParseTenantID(row.TenantID)
	if err != nil {
		return nil, err
	}

	limitCurrency, err := vo.ParseCurrency(row.SpendingLimitCurrency)
	if err != nil {
		return nil, err
	}

	spendCurrency, err := vo.ParseCurrency(row.RollingSpendCurrency)
	if err != nil {
		return nil, err
	}

	return domain.ReconstructCardAccount(
		id,
		tenantID,
		vo.New(numericToDecimal(row.SpendingLimitAmount), limitCurrency),
		vo.New(numericToDecimal(row.RollingSpendAmount), spendCurrency),
	), nil
}
