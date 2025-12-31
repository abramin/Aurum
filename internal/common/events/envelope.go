package events

import (
	"encoding/json"
	"time"

	vo "aurum/internal/common/value_objects"
)

// EventEnvelope wraps all domain events with standard metadata.
// This is the canonical event structure for all bounded contexts.
type EventEnvelope struct {
	EventID       EventID         `json:"event_id"`
	EventType     string          `json:"event_type"`
	OccurredAt    time.Time       `json:"occurred_at"`
	TenantID      vo.TenantID     `json:"tenant_id"`
	CorrelationID vo.CorrelationID `json:"correlation_id"`
	CausationID   *EventID        `json:"causation_id,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// NewEventEnvelope creates a new event envelope with generated ID and timestamp.
func NewEventEnvelope(
	eventType string,
	tenantID vo.TenantID,
	correlationID vo.CorrelationID,
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

// eventEnvelopeJSON is an internal type for JSON marshaling/unmarshaling.
type eventEnvelopeJSON struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	OccurredAt    time.Time       `json:"occurred_at"`
	TenantID      string          `json:"tenant_id"`
	CorrelationID string          `json:"correlation_id"`
	CausationID   *string         `json:"causation_id,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// MarshalJSON implements json.Marshaler for EventEnvelope.
func (e EventEnvelope) MarshalJSON() ([]byte, error) {
	j := eventEnvelopeJSON{
		EventID:       e.EventID.String(),
		EventType:     e.EventType,
		OccurredAt:    e.OccurredAt,
		TenantID:      e.TenantID.String(),
		CorrelationID: e.CorrelationID.String(),
		Payload:       e.Payload,
	}
	if e.CausationID != nil {
		s := e.CausationID.String()
		j.CausationID = &s
	}
	return json.Marshal(j)
}

// UnmarshalJSON implements json.Unmarshaler for EventEnvelope.
func (e *EventEnvelope) UnmarshalJSON(data []byte) error {
	var j eventEnvelopeJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	eventID, err := ParseEventID(j.EventID)
	if err != nil {
		return err
	}

	tenantID, err := vo.ParseTenantID(j.TenantID)
	if err != nil {
		return err
	}

	correlationID, err := vo.ParseCorrelationID(j.CorrelationID)
	if err != nil {
		return err
	}

	e.EventID = eventID
	e.EventType = j.EventType
	e.OccurredAt = j.OccurredAt
	e.TenantID = tenantID
	e.CorrelationID = correlationID
	e.Payload = j.Payload

	if j.CausationID != nil {
		causationID, err := ParseEventID(*j.CausationID)
		if err != nil {
			return err
		}
		e.CausationID = &causationID
	}

	return nil
}
