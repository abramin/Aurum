package valueobjects

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrEmptyID is returned when parsing an empty string as an ID.
var ErrEmptyID = errors.New("id cannot be empty")

// ErrInvalidUUID is returned when parsing an invalid UUID format.
var ErrInvalidUUID = errors.New("invalid uuid format")

// TenantID represents a tenant identifier for multi-tenancy isolation.
// It is a struct wrapper to prevent accidental type confusion at compile time.
type TenantID struct {
	value string
}

// ParseTenantID creates a TenantID from a string, validating it is non-empty.
func ParseTenantID(s string) (TenantID, error) {
	if s == "" {
		return TenantID{}, fmt.Errorf("tenant_id: %w", ErrEmptyID)
	}
	return TenantID{value: s}, nil
}

// MustParseTenantID creates a TenantID from a string, panicking on invalid input.
// Use only in tests or initialization code where panicking is acceptable.
func MustParseTenantID(s string) TenantID {
	t, err := ParseTenantID(s)
	if err != nil {
		panic(err)
	}
	return t
}

// String returns the string representation of TenantID.
func (t TenantID) String() string {
	return t.value
}

// IsEmpty checks if the TenantID is empty.
func (t TenantID) IsEmpty() bool {
	return t.value == ""
}

// CorrelationID tracks a request across service boundaries.
// It is a struct wrapper to prevent accidental type confusion at compile time.
type CorrelationID struct {
	value string
}

// ParseCorrelationID creates a CorrelationID from a string, validating it is non-empty.
func ParseCorrelationID(s string) (CorrelationID, error) {
	if s == "" {
		return CorrelationID{}, fmt.Errorf("correlation_id: %w", ErrEmptyID)
	}
	return CorrelationID{value: s}, nil
}

// NewCorrelationID generates a new unique CorrelationID.
func NewCorrelationID() CorrelationID {
	return CorrelationID{value: uuid.NewString()}
}

// String returns the string representation of CorrelationID.
func (c CorrelationID) String() string {
	return c.value
}

// IsEmpty checks if the CorrelationID is empty.
func (c CorrelationID) IsEmpty() bool {
	return c.value == ""
}
