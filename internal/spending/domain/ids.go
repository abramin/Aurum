package domain

import "github.com/google/uuid"

// AuthorizationID uniquely identifies an authorization.
type AuthorizationID uuid.UUID

// NewAuthorizationID generates a new AuthorizationID.
func NewAuthorizationID() AuthorizationID {
	return AuthorizationID(uuid.New())
}

// ParseAuthorizationID parses a string into an AuthorizationID.
func ParseAuthorizationID(s string) (AuthorizationID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return AuthorizationID{}, err
	}
	return AuthorizationID(id), nil
}

// String returns the string representation.
func (id AuthorizationID) String() string {
	return uuid.UUID(id).String()
}

// IsZero returns true if the ID is the zero value.
func (id AuthorizationID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

// CardAccountID uniquely identifies a card account.
type CardAccountID uuid.UUID

// NewCardAccountID generates a new CardAccountID.
func NewCardAccountID() CardAccountID {
	return CardAccountID(uuid.New())
}

// ParseCardAccountID parses a string into a CardAccountID.
func ParseCardAccountID(s string) (CardAccountID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return CardAccountID{}, err
	}
	return CardAccountID(id), nil
}

// String returns the string representation.
func (id CardAccountID) String() string {
	return uuid.UUID(id).String()
}

// IsZero returns true if the ID is the zero value.
func (id CardAccountID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}
