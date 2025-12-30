package types

import "github.com/google/uuid"

// TenantID represents a tenant identifier for multi-tenancy isolation.
type TenantID string

// CorrelationID tracks a request across service boundaries.
type CorrelationID string

// CausationID links an event to its cause (another event).
type CausationID string

// EventID uniquely identifies a domain event.
type EventID string

// NewEventID generates a new unique EventID.
func NewEventID() EventID {
	return EventID(uuid.NewString())
}

// NewCorrelationID generates a new unique CorrelationID.
func NewCorrelationID() CorrelationID {
	return CorrelationID(uuid.NewString())
}

// String returns the string representation of TenantID.
func (t TenantID) String() string {
	return string(t)
}

// String returns the string representation of CorrelationID.
func (c CorrelationID) String() string {
	return string(c)
}

// String returns the string representation of EventID.
func (e EventID) String() string {
	return string(e)
}

// IsEmpty checks if the TenantID is empty.
func (t TenantID) IsEmpty() bool {
	return t == ""
}

// IsEmpty checks if the CorrelationID is empty.
func (c CorrelationID) IsEmpty() bool {
	return c == ""
}
