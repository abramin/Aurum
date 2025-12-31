package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrEmptyCardAccountID is returned when parsing an empty card account ID.
var ErrEmptyCardAccountID = errors.New("card_account_id cannot be empty")

// ErrInvalidCardAccountID is returned when parsing an invalid UUID format.
var ErrInvalidCardAccountID = errors.New("card_account_id: invalid uuid format")

// CardAccountID uniquely identifies a card account within a tenant.
// It is a struct wrapper to prevent accidental type confusion at compile time.
type CardAccountID struct {
	value string
}

// ParseCardAccountID creates a CardAccountID from a string, validating UUID format.
func ParseCardAccountID(s string) (CardAccountID, error) {
	if s == "" {
		return CardAccountID{}, ErrEmptyCardAccountID
	}
	if _, err := uuid.Parse(s); err != nil {
		return CardAccountID{}, fmt.Errorf("%w: %s", ErrInvalidCardAccountID, s)
	}
	return CardAccountID{value: s}, nil
}

// NewCardAccountID generates a new unique CardAccountID.
func NewCardAccountID() CardAccountID {
	return CardAccountID{value: uuid.NewString()}
}

// String returns the string representation of CardAccountID.
func (c CardAccountID) String() string {
	return c.value
}

// IsEmpty checks if the CardAccountID is empty.
func (c CardAccountID) IsEmpty() bool {
	return c.value == ""
}
