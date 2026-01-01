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

// AuthorizationRepository implements domain.AuthorizationRepository using PostgreSQL.
type AuthorizationRepository struct {
	db Executor
}

// NewAuthorizationRepository creates a new AuthorizationRepository.
func NewAuthorizationRepository(db Executor) *AuthorizationRepository {
	return &AuthorizationRepository{db: db}
}

// Save persists an authorization to the database.
// Uses optimistic locking via version column to prevent concurrent modification conflicts.
func (r *AuthorizationRepository) Save(ctx context.Context, auth *domain.Authorization) error {
	// Check if authorization already exists
	var existingVersion int
	err := r.db.QueryRow(ctx,
		`SELECT version FROM spending.authorizations WHERE id = $1 AND tenant_id = $2`,
		auth.ID().String(), auth.TenantID().String(),
	).Scan(&existingVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		// Insert new authorization
		_, err = r.db.Exec(ctx, `
			INSERT INTO spending.authorizations (
				id, tenant_id, card_account_id,
				authorized_amount, authorized_currency,
				captured_amount, captured_currency,
				merchant_ref, reference, state, version,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			auth.ID().String(),
			auth.TenantID().String(),
			auth.CardAccountID().String(),
			auth.AuthorizedAmount().Amount,
			auth.AuthorizedAmount().Currency,
			auth.CapturedAmount().Amount,
			auth.CapturedAmount().Currency,
			auth.MerchantRef(),
			auth.Reference(),
			string(auth.State()),
			auth.Version(),
			auth.CreatedAt(),
			auth.UpdatedAt(),
		)
		return err
	}
	if err != nil {
		return err
	}

	// Update existing authorization with optimistic locking
	tag, err := r.db.Exec(ctx, `
		UPDATE spending.authorizations
		SET captured_amount = $1,
			captured_currency = $2,
			state = $3,
			version = $4,
			updated_at = $5
		WHERE id = $6 AND tenant_id = $7 AND version = $8`,
		auth.CapturedAmount().Amount,
		auth.CapturedAmount().Currency,
		string(auth.State()),
		auth.Version(),
		auth.UpdatedAt(),
		auth.ID().String(),
		auth.TenantID().String(),
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

// FindByID retrieves an authorization by ID.
func (r *AuthorizationRepository) FindByID(ctx context.Context, tenantID types.TenantID, id domain.AuthorizationID) (*domain.Authorization, error) {
	var (
		authID           string
		tenant           string
		cardAccountID    string
		authorizedAmount decimal.Decimal
		authorizedCurr   string
		capturedAmount   decimal.Decimal
		capturedCurr     string
		merchantRef      string
		reference        string
		state            string
		version          int
		createdAt        time.Time
		updatedAt        time.Time
	)

	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, card_account_id,
			   authorized_amount, authorized_currency,
			   captured_amount, captured_currency,
			   merchant_ref, reference, state, version,
			   created_at, updated_at
		FROM spending.authorizations
		WHERE id = $1 AND tenant_id = $2`,
		id.String(), tenantID.String(),
	).Scan(
		&authID, &tenant, &cardAccountID,
		&authorizedAmount, &authorizedCurr,
		&capturedAmount, &capturedCurr,
		&merchantRef, &reference, &state, &version,
		&createdAt, &updatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAuthorizationNotFound
	}
	if err != nil {
		return nil, err
	}

	parsedID, err := domain.ParseAuthorizationID(authID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid authorization_id %q: %v", domain.ErrCorruptData, authID, err)
	}
	parsedCardAccountID, err := domain.ParseCardAccountID(cardAccountID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid card_account_id %q: %v", domain.ErrCorruptData, cardAccountID, err)
	}

	return domain.ReconstructAuthorization(
		parsedID,
		types.TenantID(tenant),
		parsedCardAccountID,
		types.NewMoney(authorizedAmount, authorizedCurr),
		types.NewMoney(capturedAmount, capturedCurr),
		merchantRef,
		reference,
		domain.AuthorizationState(state),
		version,
		createdAt,
		updatedAt,
	)
}

// Verify interface implementation.
var _ domain.AuthorizationRepository = (*AuthorizationRepository)(nil)
