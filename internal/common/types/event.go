package types

import (
	"encoding/json"
	"time"
)

// EventEnvelope wraps all domain events with standard metadata.
// This is the canonical event structure for all bounded contexts.
type EventEnvelope struct {
	EventID       EventID         `json:"event_id"`
	EventType     string          `json:"event_type"`
	OccurredAt    time.Time       `json:"occurred_at"`
	TenantID      TenantID        `json:"tenant_id"`
	CorrelationID CorrelationID   `json:"correlation_id"`
	CausationID   *EventID        `json:"causation_id,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// NewEventEnvelope creates a new event envelope with generated ID and timestamp.
func NewEventEnvelope(
	eventType string,
	tenantID TenantID,
	correlationID CorrelationID,
	payload any,
) (EventEnvelope, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return EventEnvelope{}, err
	}

	return EventEnvelope{
		EventID:       NewEventID(),
		EventType:     eventType,
		OccurredAt:    time.Now().UTC(),
		TenantID:      tenantID,
		CorrelationID: correlationID,
		Payload:       payloadBytes,
	}, nil
}

// WithCausation returns a copy of the envelope with causation ID set.
func (e EventEnvelope) WithCausation(causationID EventID) EventEnvelope {
	e.CausationID = &causationID
	return e
}

// UnmarshalPayload decodes the payload into the target struct.
func (e EventEnvelope) UnmarshalPayload(target any) error {
	return json.Unmarshal(e.Payload, target)
}

// MarshalJSON implements json.Marshaler for EventEnvelope.
func (e EventEnvelope) MarshalJSON() ([]byte, error) {
	type Alias EventEnvelope
	return json.Marshal(Alias(e))
}
