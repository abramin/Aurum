package domain

import (
	"time"

	"aurum/internal/common/types"
)

// AuthorizationState represents the state of an authorization.
type AuthorizationState string

const (
	AuthorizationStateAuthorized AuthorizationState = "authorized"
	AuthorizationStateCaptured   AuthorizationState = "captured"
	AuthorizationStateReversed   AuthorizationState = "reversed"
	AuthorizationStateExpired    AuthorizationState = "expired"
)

// IsValid returns true if the state is a known valid state.
func (s AuthorizationState) IsValid() bool {
	switch s {
	case AuthorizationStateAuthorized, AuthorizationStateCaptured,
		AuthorizationStateReversed, AuthorizationStateExpired:
		return true
	default:
		return false
	}
}

// Authorization represents a spend authorization (aggregate root).
// Invariants:
//   - Cannot capture more than authorized amount
//   - Cannot capture twice
//   - Capture requires Authorized state
type Authorization struct {
	id               AuthorizationID
	tenantID         types.TenantID
	cardAccountID    CardAccountID
	authorizedAmount types.Money
	capturedAmount   types.Money
	merchantRef      string
	reference        string
	state            AuthorizationState
	version          int
	createdAt        time.Time
	updatedAt        time.Time
}

// NewAuthorization creates a new authorization in the Authorized state.
// Returns error if tenant ID is empty.
func NewAuthorization(
	tenantID types.TenantID,
	cardAccountID CardAccountID,
	authorizedAmount types.Money,
	merchantRef string,
	reference string,
) (*Authorization, error) {
	if tenantID.IsEmpty() {
		return nil, ErrEmptyTenantID
	}
	now := time.Now()
	return &Authorization{
		id:               NewAuthorizationID(),
		tenantID:         tenantID,
		cardAccountID:    cardAccountID,
		authorizedAmount: authorizedAmount,
		capturedAmount:   types.Zero(authorizedAmount.Currency),
		merchantRef:      merchantRef,
		reference:        reference,
		state:            AuthorizationStateAuthorized,
		version:          1,
		createdAt:        now,
		updatedAt:        now,
	}, nil
}

// ReconstructAuthorization reconstructs an Authorization from persistence.
// Returns error if data violates domain invariants (indicates corrupt data).
func ReconstructAuthorization(
	id AuthorizationID,
	tenantID types.TenantID,
	cardAccountID CardAccountID,
	authorizedAmount types.Money,
	capturedAmount types.Money,
	merchantRef string,
	reference string,
	state AuthorizationState,
	version int,
	createdAt time.Time,
	updatedAt time.Time,
) (*Authorization, error) {
	if !state.IsValid() {
		return nil, ErrCorruptData
	}
	if capturedAmount.GreaterThan(authorizedAmount) {
		return nil, ErrCorruptData
	}
	return &Authorization{
		id:               id,
		tenantID:         tenantID,
		cardAccountID:    cardAccountID,
		authorizedAmount: authorizedAmount,
		capturedAmount:   capturedAmount,
		merchantRef:      merchantRef,
		reference:        reference,
		state:            state,
		version:          version,
		createdAt:        createdAt,
		updatedAt:        updatedAt,
	}, nil
}

// Capture captures the authorization with the given amount.
// Returns error if:
//   - Already captured
//   - Not in Authorized state
//   - Amount exceeds authorized amount
func (a *Authorization) Capture(amount types.Money) error {
	if a.state == AuthorizationStateCaptured {
		return ErrAlreadyCaptured
	}
	if a.state != AuthorizationStateAuthorized {
		return ErrInvalidStateTransition
	}
	if amount.Currency != a.authorizedAmount.Currency {
		return ErrCurrencyMismatch
	}
	if amount.GreaterThan(a.authorizedAmount) {
		return ErrExceedsAuthorizedAmount
	}

	a.capturedAmount = amount
	a.state = AuthorizationStateCaptured
	a.version++
	a.updatedAt = time.Now()
	return nil
}

// Reverse reverses the authorization.
func (a *Authorization) Reverse() error {
	if a.state != AuthorizationStateAuthorized {
		return ErrInvalidStateTransition
	}

	a.state = AuthorizationStateReversed
	a.version++
	a.updatedAt = time.Now()
	return nil
}

// Expire expires the authorization.
func (a *Authorization) Expire() error {
	if a.state != AuthorizationStateAuthorized {
		return ErrInvalidStateTransition
	}

	a.state = AuthorizationStateExpired
	a.version++
	a.updatedAt = time.Now()
	return nil
}

// Getters

func (a *Authorization) ID() AuthorizationID       { return a.id }
func (a *Authorization) TenantID() types.TenantID  { return a.tenantID }
func (a *Authorization) CardAccountID() CardAccountID { return a.cardAccountID }
func (a *Authorization) AuthorizedAmount() types.Money { return a.authorizedAmount }
func (a *Authorization) CapturedAmount() types.Money { return a.capturedAmount }
func (a *Authorization) MerchantRef() string       { return a.merchantRef }
func (a *Authorization) Reference() string         { return a.reference }
func (a *Authorization) State() AuthorizationState { return a.state }
func (a *Authorization) Version() int              { return a.version }
func (a *Authorization) CreatedAt() time.Time      { return a.createdAt }
func (a *Authorization) UpdatedAt() time.Time      { return a.updatedAt }
