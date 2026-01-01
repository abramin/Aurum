package domain

import (
	"time"

	"aurum/internal/common/types"
)

// CardAccount represents a card account with spending limits (aggregate root).
// Invariants:
//   - Rolling spend cannot exceed spending limit
//   - Currencies must match between limit and spend
type CardAccount struct {
	id            CardAccountID
	tenantID      types.TenantID
	spendingLimit types.Money
	rollingSpend  types.Money
	version       int
	createdAt     time.Time
	updatedAt     time.Time
}

// NewCardAccount creates a new card account with the given spending limit.
// The now parameter makes the function pure and testable.
// Returns error if tenant ID is empty.
func NewCardAccount(tenantID types.TenantID, spendingLimit types.Money, now time.Time) (*CardAccount, error) {
	if tenantID.IsEmpty() {
		return nil, ErrEmptyTenantID
	}
	return &CardAccount{
		id:            NewCardAccountID(),
		tenantID:      tenantID,
		spendingLimit: spendingLimit,
		rollingSpend:  types.Zero(spendingLimit.Currency),
		version:       1,
		createdAt:     now,
		updatedAt:     now,
	}, nil
}

// ReconstructCardAccount reconstructs a CardAccount from persistence.
// This bypasses validation - only use for loading from database.
func ReconstructCardAccount(
	id CardAccountID,
	tenantID types.TenantID,
	spendingLimit types.Money,
	rollingSpend types.Money,
	version int,
	createdAt time.Time,
	updatedAt time.Time,
) *CardAccount {
	return &CardAccount{
		id:            id,
		tenantID:      tenantID,
		spendingLimit: spendingLimit,
		rollingSpend:  rollingSpend,
		version:       version,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
	}
}

// AuthorizeAmount validates and records an authorization amount.
// The now parameter makes the function pure and testable.
// Returns error if the authorization would exceed the spending limit.
func (c *CardAccount) AuthorizeAmount(amount types.Money, now time.Time) error {
	if amount.Currency != c.spendingLimit.Currency {
		return ErrCurrencyMismatch
	}

	newSpend, err := c.rollingSpend.Add(amount)
	if err != nil {
		return err
	}

	if newSpend.GreaterThan(c.spendingLimit) {
		return ErrSpendingLimitExceeded
	}

	c.rollingSpend = newSpend
	c.version++
	c.updatedAt = now
	return nil
}

// ReleaseAmount releases a previously authorized amount (e.g., on reversal).
// The now parameter makes the function pure and testable.
func (c *CardAccount) ReleaseAmount(amount types.Money, now time.Time) error {
	if amount.Currency != c.rollingSpend.Currency {
		return ErrCurrencyMismatch
	}

	newSpend, err := c.rollingSpend.Subtract(amount)
	if err != nil {
		return err
	}

	c.rollingSpend = newSpend
	c.version++
	c.updatedAt = now
	return nil
}

// AvailableLimit returns the remaining spending limit.
func (c *CardAccount) AvailableLimit() types.Money {
	available, _ := c.spendingLimit.Subtract(c.rollingSpend)
	return available
}

// Getters

func (c *CardAccount) ID() CardAccountID         { return c.id }
func (c *CardAccount) TenantID() types.TenantID  { return c.tenantID }
func (c *CardAccount) SpendingLimit() types.Money { return c.spendingLimit }
func (c *CardAccount) RollingSpend() types.Money { return c.rollingSpend }
func (c *CardAccount) Version() int              { return c.version }
func (c *CardAccount) CreatedAt() time.Time      { return c.createdAt }
func (c *CardAccount) UpdatedAt() time.Time      { return c.updatedAt }
