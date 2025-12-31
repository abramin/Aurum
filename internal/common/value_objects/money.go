package valueobjects

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// Currency represents a validated currency code.
type Currency string

// Supported currency codes
const (
	CurrencyEUR Currency = "EUR"
	CurrencyUSD Currency = "USD"
	CurrencyGBP Currency = "GBP"
)

// ErrInvalidCurrency is returned when parsing an unsupported currency code.
var ErrInvalidCurrency = errors.New("invalid or unsupported currency code")

// ErrNonPositiveAmount is returned when a positive amount is required but not provided.
var ErrNonPositiveAmount = errors.New("amount must be positive")

// validCurrencies is a set of supported currency codes.
var validCurrencies = map[Currency]bool{
	CurrencyEUR: true,
	CurrencyUSD: true,
	CurrencyGBP: true,
}

// ParseCurrency validates and parses a currency code string.
func ParseCurrency(s string) (Currency, error) {
	c := Currency(s)
	if !validCurrencies[c] {
		return "", fmt.Errorf("%w: %s", ErrInvalidCurrency, s)
	}
	return c, nil
}

// String returns the string representation of Currency.
func (c Currency) String() string {
	return string(c)
}

// Money represents a monetary amount with currency.
// Uses decimal.Decimal for precise financial calculations.
type Money struct {
	Amount   decimal.Decimal `json:"value"`
	Currency Currency        `json:"currency"`
}

// New creates a new Money instance with a validated currency.
func New(amount decimal.Decimal, currency Currency) Money {
	return Money{
		Amount:   amount,
		Currency: currency,
	}
}

// NewFromString creates Money from a string amount and currency.
// Validates both the decimal format and currency code.
func NewFromString(amount string, currency string) (Money, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, fmt.Errorf("invalid amount %q: %w", amount, err)
	}
	c, err := ParseCurrency(currency)
	if err != nil {
		return Money{}, err
	}
	return New(d, c), nil
}

// NewFromInt creates Money from an integer (whole units) with a validated currency.
func NewFromInt(amount int64, currency string) Money {
	// For convenience, this panics on invalid currency since it's typically used
	// with constants. Use NewFromString for user input.
	c, err := ParseCurrency(currency)
	if err != nil {
		panic(err)
	}
	return New(decimal.NewFromInt(amount), c)
}

// Zero returns a zero Money in the given currency.
func Zero(currency Currency) Money {
	return New(decimal.Zero, currency)
}

// NewPositiveMoney creates Money that must be > 0 (for authorization amounts, etc).
func NewPositiveMoney(amount decimal.Decimal, currency Currency) (Money, error) {
	if !amount.IsPositive() {
		return Money{}, ErrNonPositiveAmount
	}
	return New(amount, currency), nil
}

// NewPositiveFromString creates Money from a string, validating it is positive.
// Use for authorization amounts and other operations requiring positive values.
func NewPositiveFromString(amount string, currency string) (Money, error) {
	m, err := NewFromString(amount, currency)
	if err != nil {
		return Money{}, err
	}
	if !m.Amount.IsPositive() {
		return Money{}, ErrNonPositiveAmount
	}
	return m, nil
}

// Add adds two Money values. Returns error if currencies don't match.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errors.New("cannot add money with different currencies")
	}
	return New(m.Amount.Add(other.Amount), m.Currency), nil
}

// Subtract subtracts other from m. Returns error if currencies don't match.
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errors.New("cannot subtract money with different currencies")
	}
	return New(m.Amount.Sub(other.Amount), m.Currency), nil
}

// Negate returns the negated value (useful for ledger entries).
func (m Money) Negate() Money {
	return New(m.Amount.Neg(), m.Currency)
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
