package domain

import (
	vo "aurum/internal/common/value_objects"
)

// CardAccount represents a card account with spending limits.
// It tracks the rolling spend to enforce limits on new authorizations.
type CardAccount struct {
	id            CardAccountID
	tenantID      vo.TenantID
	spendingLimit vo.Money
	rollingSpend  vo.Money
}

// ErrSpendingLimitExceeded is returned when an authorization would exceed the spending limit.
type ErrSpendingLimitExceeded struct{}

func (e ErrSpendingLimitExceeded) Error() string {
	return "spending limit exceeded"
}

// ErrCurrencyMismatch is returned when currencies don't match.
type ErrCurrencyMismatch struct{}

func (e ErrCurrencyMismatch) Error() string {
	return "currency mismatch"
}

// NewCardAccount creates a new CardAccount with the given spending limit.
func NewCardAccount(id CardAccountID, tenantID vo.TenantID, limit vo.Money) *CardAccount {
	return &CardAccount{
		id:            id,
		tenantID:      tenantID,
		spendingLimit: limit,
		rollingSpend:  vo.Zero(limit.Currency),
	}
}

// ID returns the card account identifier.
func (c *CardAccount) ID() CardAccountID {
	return c.id
}

// TenantID returns the tenant identifier.
func (c *CardAccount) TenantID() vo.TenantID {
	return c.tenantID
}

// SpendingLimit returns the spending limit.
func (c *CardAccount) SpendingLimit() vo.Money {
	return c.spendingLimit
}

// RollingSpend returns the current rolling spend.
func (c *CardAccount) RollingSpend() vo.Money {
	return c.rollingSpend
}

// AvailableLimit returns the remaining available spending limit.
func (c *CardAccount) AvailableLimit() vo.Money {
	available, _ := c.spendingLimit.Subtract(c.rollingSpend)
	return available
}

// CanAuthorize checks if the given amount can be authorized within the spending limit.
// Deprecated: Use AuthorizeAmount for atomic check-and-mutate to avoid TOCTOU races.
func (c *CardAccount) CanAuthorize(amount vo.Money) error {
	if amount.Currency != c.spendingLimit.Currency {
		return ErrCurrencyMismatch{}
	}

	newSpend, err := c.rollingSpend.Add(amount)
	if err != nil {
		return err
	}

	if newSpend.GreaterThan(c.spendingLimit) {
		return ErrSpendingLimitExceeded{}
	}

	return nil
}

// RecordAuthorization records a new authorization against the rolling spend.
// Deprecated: Use AuthorizeAmount for atomic check-and-mutate to avoid TOCTOU races.
func (c *CardAccount) RecordAuthorization(amount vo.Money) error {
	if amount.Currency != c.spendingLimit.Currency {
		return ErrCurrencyMismatch{}
	}

	newSpend, err := c.rollingSpend.Add(amount)
	if err != nil {
		return err
	}

	c.rollingSpend = newSpend
	return nil
}

// AuthorizeAmount atomically validates the spending limit and records the authorization.
// This prevents TOCTOU races between checking and recording.
func (c *CardAccount) AuthorizeAmount(amount vo.Money) error {
	if amount.Currency != c.spendingLimit.Currency {
		return ErrCurrencyMismatch{}
	}

	newSpend, err := c.rollingSpend.Add(amount)
	if err != nil {
		return err
	}

	if newSpend.GreaterThan(c.spendingLimit) {
		return ErrSpendingLimitExceeded{}
	}

	c.rollingSpend = newSpend
	return nil
}

// RecordCapture records a capture. Since the authorization was already counted,
// capture does not increase the rolling spend (no double-counting).
func (c *CardAccount) RecordCapture(amount vo.Money) error {
	// Capture doesn't change rolling spend - already counted at authorization time
	return nil
}

// RecordReversal records a reversal, decreasing the rolling spend.
func (c *CardAccount) RecordReversal(amount vo.Money) error {
	if amount.Currency != c.spendingLimit.Currency {
		return ErrCurrencyMismatch{}
	}

	newSpend, err := c.rollingSpend.Subtract(amount)
	if err != nil {
		return err
	}

	c.rollingSpend = newSpend
	return nil
}
