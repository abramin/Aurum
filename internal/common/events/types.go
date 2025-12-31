package events

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrEmptyID is returned when parsing an empty string as an ID.
var ErrEmptyID = errors.New("id cannot be empty")

// ErrInvalidUUID is returned when parsing an invalid UUID format.
var ErrInvalidUUID = errors.New("invalid uuid format")

// EventID uniquely identifies a domain event.
// It is a struct wrapper to prevent accidental type confusion at compile time.
type EventID struct {
	value string
}

// ParseEventID creates an EventID from a string, validating UUID format.
func ParseEventID(s string) (EventID, error) {
	if s == "" {
		return EventID{}, fmt.Errorf("event_id: %w", ErrEmptyID)
	}
	if _, err := uuid.Parse(s); err != nil {
		return EventID{}, fmt.Errorf("event_id: %w", ErrInvalidUUID)
	}
	return EventID{value: s}, nil
}

// NewEventID generates a new unique EventID.
func NewEventID() EventID {
	return EventID{value: uuid.NewString()}
}

// String returns the string representation of EventID.
func (e EventID) String() string {
	return e.value
}

// IsEmpty checks if the EventID is empty.
func (e EventID) IsEmpty() bool {
	return e.value == ""
}

// CausationID links an event to its cause (another event).
// It is a struct wrapper to prevent accidental type confusion at compile time.
type CausationID struct {
	value string
}

// ParseCausationID creates a CausationID from a string, validating UUID format.
func ParseCausationID(s string) (CausationID, error) {
	if s == "" {
		return CausationID{}, fmt.Errorf("causation_id: %w", ErrEmptyID)
	}
	if _, err := uuid.Parse(s); err != nil {
		return CausationID{}, fmt.Errorf("causation_id: %w", ErrInvalidUUID)
	}
	return CausationID{value: s}, nil
}

// String returns the string representation of CausationID.
func (c CausationID) String() string {
	return c.value
}

// IsEmpty checks if the CausationID is empty.
func (c CausationID) IsEmpty() bool {
	return c.value == ""
}
