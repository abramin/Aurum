package types

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// Common currency codes
const (
	// CurrencyEUR is the ISO 4217 code for Euro.
	CurrencyEUR = "EUR"
	// CurrencyUSD is the ISO 4217 code for US Dollar.
	CurrencyUSD = "USD"
	// CurrencyGBP is the ISO 4217 code for British Pound.
	CurrencyGBP = "GBP"
)

// Money represents a monetary amount with currency.
// Uses decimal.Decimal for precise financial calculations.
type Money struct {
	Amount   decimal.Decimal `json:"value"`
	Currency string          `json:"currency"`
}

// NewMoney creates a new Money instance.
func NewMoney(amount decimal.Decimal, currency string) Money {
	return Money{
		Amount:   amount,
		Currency: currency,
	}
}

// NewMoneyFromString creates Money from a string amount.
func NewMoneyFromString(amount, currency string) (Money, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, fmt.Errorf("invalid amount %q: %w", amount, err)
	}
	return NewMoney(d, currency), nil
}

// NewMoneyFromInt creates Money from an integer (whole units).
func NewMoneyFromInt(amount int64, currency string) Money {
	return NewMoney(decimal.NewFromInt(amount), currency)
}

// Zero returns a zero Money in the given currency.
func Zero(currency string) Money {
	return NewMoney(decimal.Zero, currency)
}

// Add adds two Money values. Returns error if currencies don't match.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errors.New("cannot add money with different currencies")
	}
	return NewMoney(m.Amount.Add(other.Amount), m.Currency), nil
}

// Subtract subtracts other from m. Returns error if currencies don't match.
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errors.New("cannot subtract money with different currencies")
	}
	return NewMoney(m.Amount.Sub(other.Amount), m.Currency), nil
}

// Negate returns the negated value (useful for ledger entries).
func (m Money) Negate() Money {
	return NewMoney(m.Amount.Neg(), m.Currency)
}

// IsPositive returns true if amount > 0.
func (m Money) IsPositive() bool {
	return m.Amount.IsPositive()
}

// IsNegative returns true if amount < 0.
func (m Money) IsNegative() bool {
	return m.Amount.IsNegative()
}

// IsZero returns true if amount == 0.
func (m Money) IsZero() bool {
	return m.Amount.IsZero()
}

// GreaterThan returns true if m > other.
func (m Money) GreaterThan(other Money) bool {
	return m.Currency == other.Currency && m.Amount.GreaterThan(other.Amount)
}

// LessThanOrEqual returns true if m <= other.
func (m Money) LessThanOrEqual(other Money) bool {
	return m.Currency == other.Currency && m.Amount.LessThanOrEqual(other.Amount)
}

// Equal returns true if both amount and currency match.
func (m Money) Equal(other Money) bool {
	return m.Currency == other.Currency && m.Amount.Equal(other.Amount)
}

// String returns a human-readable representation.
func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.Amount.StringFixed(2), m.Currency)
}
