package domain

import (
	"encoding/json"
	"time"

	"aurum/internal/common/types"
)

// Event types for the Spending context.
const (
	EventTypeSpendAuthorized = "spend.authorized"
	EventTypeSpendCaptured   = "spend.captured"
	EventTypeSpendReversed   = "spend.reversed"
	EventTypeSpendExpired    = "spend.expired"
)

// SpendAuthorizedEvent is emitted when a spend is authorized.
type SpendAuthorizedEvent struct {
	AuthorizationID string      `json:"authorization_id"`
	TenantID        string      `json:"tenant_id"`
	CardAccountID   string      `json:"card_account_id"`
	Amount          types.Money `json:"amount"`
	MerchantRef     string      `json:"merchant_ref"`
	Reference       string      `json:"reference"`
	OccurredAt      time.Time   `json:"occurred_at"`
}

// SpendCapturedEvent is emitted when a spend is captured.
type SpendCapturedEvent struct {
	AuthorizationID string      `json:"authorization_id"`
	TenantID        string      `json:"tenant_id"`
	CardAccountID   string      `json:"card_account_id"`
	CapturedAmount  types.Money `json:"captured_amount"`
	OccurredAt      time.Time   `json:"occurred_at"`
}

// SpendReversedEvent is emitted when a spend is reversed.
type SpendReversedEvent struct {
	AuthorizationID string      `json:"authorization_id"`
	TenantID        string      `json:"tenant_id"`
	CardAccountID   string      `json:"card_account_id"`
	Amount          types.Money `json:"amount"`
	OccurredAt      time.Time   `json:"occurred_at"`
}

// NewSpendAuthorizedOutboxEntry creates an outbox entry for SpendAuthorized event.
func NewSpendAuthorizedOutboxEntry(
	auth *Authorization,
	correlationID types.CorrelationID,
) (*OutboxEntry, error) {
	event := SpendAuthorizedEvent{
		AuthorizationID: auth.ID().String(),
		TenantID:        auth.TenantID().String(),
		CardAccountID:   auth.CardAccountID().String(),
		Amount:          auth.AuthorizedAmount(),
		MerchantRef:     auth.MerchantRef(),
		Reference:       auth.Reference(),
		OccurredAt:      time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	return &OutboxEntry{
		ID:            types.NewEventID(),
		EventType:     EventTypeSpendAuthorized,
		TenantID:      auth.TenantID(),
		CorrelationID: correlationID,
		Payload:       payload,
		OccurredAt:    event.OccurredAt,
	}, nil
}

// NewSpendCapturedOutboxEntry creates an outbox entry for SpendCaptured event.
func NewSpendCapturedOutboxEntry(
	auth *Authorization,
	correlationID types.CorrelationID,
) (*OutboxEntry, error) {
	event := SpendCapturedEvent{
		AuthorizationID: auth.ID().String(),
		TenantID:        auth.TenantID().String(),
		CardAccountID:   auth.CardAccountID().String(),
		CapturedAmount:  auth.CapturedAmount(),
		OccurredAt:      time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	return &OutboxEntry{
		ID:            types.NewEventID(),
		EventType:     EventTypeSpendCaptured,
		TenantID:      auth.TenantID(),
		CorrelationID: correlationID,
		Payload:       payload,
		OccurredAt:    event.OccurredAt,
	}, nil
}

// NewSpendReversedOutboxEntry creates an outbox entry for SpendReversed event.
func NewSpendReversedOutboxEntry(
	auth *Authorization,
	correlationID types.CorrelationID,
) (*OutboxEntry, error) {
	event := SpendReversedEvent{
		AuthorizationID: auth.ID().String(),
		TenantID:        auth.TenantID().String(),
		CardAccountID:   auth.CardAccountID().String(),
		Amount:          auth.AuthorizedAmount(),
		OccurredAt:      time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	return &OutboxEntry{
		ID:            types.NewEventID(),
		EventType:     EventTypeSpendReversed,
		TenantID:      auth.TenantID(),
		CorrelationID: correlationID,
		Payload:       payload,
		OccurredAt:    event.OccurredAt,
	}, nil
}
