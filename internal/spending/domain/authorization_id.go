package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrEmptyAuthorizationID is returned when parsing an empty authorization ID.
var ErrEmptyAuthorizationID = errors.New("authorization_id cannot be empty")

// ErrInvalidAuthorizationID is returned when parsing an invalid UUID format.
var ErrInvalidAuthorizationID = errors.New("authorization_id: invalid uuid format")

// AuthorizationID uniquely identifies an authorization within a tenant.
// It is a struct wrapper to prevent accidental type confusion at compile time.
type AuthorizationID struct {
	value string
}

// ParseAuthorizationID creates an AuthorizationID from a string, validating UUID format.
func ParseAuthorizationID(s string) (AuthorizationID, error) {
	if s == "" {
		return AuthorizationID{}, ErrEmptyAuthorizationID
	}
	if _, err := uuid.Parse(s); err != nil {
		return AuthorizationID{}, fmt.Errorf("%w: %s", ErrInvalidAuthorizationID, s)
	}
	return AuthorizationID{value: s}, nil
}

// NewAuthorizationID generates a new unique AuthorizationID.
func NewAuthorizationID() AuthorizationID {
	return AuthorizationID{value: uuid.NewString()}
}

// String returns the string representation of AuthorizationID.
func (a AuthorizationID) String() string {
	return a.value
}

// IsEmpty checks if the AuthorizationID is empty.
func (a AuthorizationID) IsEmpty() bool {
	return a.value == ""
}
