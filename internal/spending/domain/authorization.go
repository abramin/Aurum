package domain

import (
	vo "aurum/internal/common/value_objects"
)

type Authorization struct {
	id               AuthorizationID
	tenantID         vo.TenantID
	cardAccountID    CardAccountID
	authorizedAmount vo.Money
	capturedAmount   vo.Money
	state            AuthorizationState
}

type ErrInvalidStateTransition struct{}

// Error implements [error].
func (e ErrInvalidStateTransition) Error() string {
	return "invalid state transition"
}

type ErrExceedsAuthorizedAmount struct{}

// Error implements [error].
func (e ErrExceedsAuthorizedAmount) Error() string {
	return "amount exceeds authorized amount"
}

type ErrAlreadyCaptured struct{}

// Error implements [error].
func (e ErrAlreadyCaptured) Error() string {
	return "authorization already captured"
}

func NewAuthorization(tenantID vo.TenantID, cardAccountID CardAccountID, amount vo.Money) *Authorization {
	return &Authorization{
		id:               NewAuthorizationID(),
		tenantID:         tenantID,
		cardAccountID:    cardAccountID,
		authorizedAmount: amount,
		capturedAmount:   vo.Zero(amount.Currency),
		state:            AuthorizationStateAuthorized,
	}
}

// ID returns the authorization identifier.
func (a *Authorization) ID() AuthorizationID {
	return a.id
}

// TenantID returns the tenant identifier.
func (a *Authorization) TenantID() vo.TenantID {
	return a.tenantID
}

// CardAccountID returns the card account identifier.
func (a *Authorization) CardAccountID() CardAccountID {
	return a.cardAccountID
}

// AuthorizedAmount returns the authorized amount.
func (a *Authorization) AuthorizedAmount() vo.Money {
	return a.authorizedAmount
}

func (a *Authorization) Capture(amount vo.Money) error {
	if a.state == AuthorizationStateCaptured {
		return ErrAlreadyCaptured{}
	}
	if a.state != AuthorizationStateAuthorized {
		return ErrInvalidStateTransition{}
	}
	if amount.GreaterThan(a.authorizedAmount) {
		return ErrExceedsAuthorizedAmount{}
	}
	a.state = AuthorizationStateCaptured
	a.capturedAmount = amount
	return nil
}

func (a *Authorization) State() AuthorizationState {
	return a.state
}

func (a *Authorization) Reverse() error {
	if a.state != AuthorizationStateAuthorized {
		return ErrInvalidStateTransition{}
	}
	a.state = AuthorizationStateReversed
	return nil
}

func (a *Authorization) Expire() error {
	if a.state != AuthorizationStateAuthorized {
		return ErrInvalidStateTransition{}
	}
	a.state = AuthorizationStateExpired
	return nil
}

func (a *Authorization) CapturedAmount() vo.Money {
	return a.capturedAmount
}

// ReconstructAuthorization reconstructs an Authorization from persisted state.
// This is used by repository implementations to rehydrate entities from storage.
// It bypasses business validation since the data is assumed valid from the database.
func ReconstructAuthorization(
	id AuthorizationID,
	tenantID vo.TenantID,
	cardAccountID CardAccountID,
	authorizedAmount vo.Money,
	capturedAmount vo.Money,
	state AuthorizationState,
) *Authorization {
	return &Authorization{
		id:               id,
		tenantID:         tenantID,
		cardAccountID:    cardAccountID,
		authorizedAmount: authorizedAmount,
		capturedAmount:   capturedAmount,
		state:            state,
	}
}
