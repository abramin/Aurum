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

// AuthorizationRepository persists Authorization aggregates using PostgreSQL.
// It scopes reads by tenant_id and writes the tenant_id from the aggregate to enforce tenant isolation.
type AuthorizationRepository struct {
	queries *sqlc.Queries
}

// NewAuthorizationRepository binds sqlc queries to a database handle (pool or tx).
// Callers control transactional scope by passing a pgx.Tx when participating in a unit of work.
func NewAuthorizationRepository(db sqlc.DBTX) *AuthorizationRepository {
	return &AuthorizationRepository{queries: sqlc.New(db)}
}

// Save persists an Authorization aggregate to the database.
// It uses an UPSERT with optimistic locking:
//   - Inserts when version == 1
//   - Updates only if the stored version matches (version - 1)
//
// Side effects: writes to spending.authorizations.
// Errors: returns domain.ErrOptimisticLock on version conflict; returns database errors on failure.
// Concurrency: uses optimistic locking via the version check.
func (r *AuthorizationRepository) Save(ctx context.Context, auth *domain.Authorization) error {
	rows, err := r.queries.UpsertAuthorization(ctx, sqlc.UpsertAuthorizationParams{
		ID:                 uuid.UUID(auth.ID()),
		TenantID:           auth.TenantID().String(),
		CardAccountID:      uuid.UUID(auth.CardAccountID()),
		AuthorizedAmount:   decimalToNumeric(auth.AuthorizedAmount().Amount),
		AuthorizedCurrency: auth.AuthorizedAmount().Currency,
		CapturedAmount:     decimalToNumeric(auth.CapturedAmount().Amount),
		CapturedCurrency:   auth.CapturedAmount().Currency,
		MerchantRef:        textFromString(auth.MerchantRef()),
		Reference:          textFromString(auth.Reference()),
		State:              string(auth.State()),
		Version:            int32(auth.Version()),
		CreatedAt:          timeToTimestamptz(auth.CreatedAt()),
		UpdatedAt:          timeToTimestamptz(auth.UpdatedAt()),
	})
	if err != nil {
		return err
	}

	// For updates, if version didn't match, no rows affected.
	// For inserts, version=1 so we expect 1 row.
	// Check: version > 1 means update, and 0 rows = conflict.
	if auth.Version() > 1 && rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

// FindByID retrieves an Authorization aggregate by ID.
// It queries by tenant and ID, maps missing rows to ErrAuthorizationNotFound,
// validates stored IDs/state, and reconstructs the aggregate from stored values.
// Errors: returns domain.ErrAuthorizationNotFound when missing and domain.ErrCorruptData
// when stored values violate invariants or cannot be decoded.
func (r *AuthorizationRepository) FindByID(ctx context.Context, tenantID types.TenantID, id domain.AuthorizationID) (*domain.Authorization, error) {
	row, err := r.queries.GetAuthorizationByID(ctx, sqlc.GetAuthorizationByIDParams{
		ID:       uuid.UUID(id),
		TenantID: tenantID.String(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAuthorizationNotFound
	}
	if err != nil {
		return nil, err
	}

	authorizedAmount, err := numericToDecimal(row.AuthorizedAmount)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid authorized_amount: %v", domain.ErrCorruptData, err)
	}
	capturedAmount, err := numericToDecimal(row.CapturedAmount)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid captured_amount: %v", domain.ErrCorruptData, err)
	}
	createdAt, err := timestamptzToTime(row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid created_at: %v", domain.ErrCorruptData, err)
	}
	updatedAt, err := timestamptzToTime(row.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid updated_at: %v", domain.ErrCorruptData, err)
	}

	return domain.ReconstructAuthorization(
		domain.AuthorizationID(row.ID),
		types.TenantID(row.TenantID),
		domain.CardAccountID(row.CardAccountID),
		types.NewMoney(authorizedAmount, row.AuthorizedCurrency),
		types.NewMoney(capturedAmount, row.CapturedCurrency),
		row.MerchantRef,
		row.Reference,
		domain.AuthorizationState(row.State),
		int(row.Version),
		createdAt,
		updatedAt,
	)
}

// Verify interface implementation.
var _ domain.AuthorizationRepository = (*AuthorizationRepository)(nil)
