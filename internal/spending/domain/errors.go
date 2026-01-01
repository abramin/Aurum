package domain

import "errors"

// Domain errors for the Spending context.
var (
	// ErrAuthorizationNotFound is returned when an authorization cannot be found.
	ErrAuthorizationNotFound = errors.New("authorization not found")

	// ErrCardAccountNotFound is returned when a card account cannot be found.
	ErrCardAccountNotFound = errors.New("card account not found")

	// ErrAlreadyCaptured is returned when attempting to capture an already captured authorization.
	ErrAlreadyCaptured = errors.New("authorization already captured")

	// ErrInvalidStateTransition is returned when a state transition is not allowed.
	ErrInvalidStateTransition = errors.New("invalid state transition")

	// ErrExceedsAuthorizedAmount is returned when capture amount exceeds authorized amount.
	ErrExceedsAuthorizedAmount = errors.New("capture amount exceeds authorized amount")

	// ErrSpendingLimitExceeded is returned when a transaction would exceed the spending limit.
	ErrSpendingLimitExceeded = errors.New("spending limit exceeded")

	// ErrCurrencyMismatch is returned when currencies don't match.
	ErrCurrencyMismatch = errors.New("currency mismatch")

	// ErrOptimisticLock is returned when an optimistic lock conflict occurs.
	ErrOptimisticLock = errors.New("optimistic lock conflict")

	// ErrIdempotencyKeyExists is returned when an idempotency key already exists.
	ErrIdempotencyKeyExists = errors.New("idempotency key already exists")

	// ErrCorruptData is returned when data loaded from persistence is invalid.
	ErrCorruptData = errors.New("corrupt data in database")

	// ErrEmptyTenantID is returned when a required tenant ID is empty.
	ErrEmptyTenantID = errors.New("tenant_id is required")
)
