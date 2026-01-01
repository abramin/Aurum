package postgres

import (
	"context"
	"time"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

// OutboxRepository implements domain.OutboxRepository using PostgreSQL.
// This implements the outbox pattern for reliable event publishing.
//
// Events are written to the outbox within the same transaction as domain changes,
// then published asynchronously by a separate process (outbox publisher).
type OutboxRepository struct {
	db Executor
}

// NewOutboxRepository creates a new OutboxRepository.
func NewOutboxRepository(db Executor) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// Append adds an event to the outbox.
func (r *OutboxRepository) Append(ctx context.Context, entry *domain.OutboxEntry) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO spending.outbox (
			event_id, event_type, tenant_id,
			correlation_id, causation_id,
			payload, occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.ID.String(),
		entry.EventType,
		entry.TenantID.String(),
		string(entry.CorrelationID),
		string(entry.CausationID),
		entry.Payload,
		entry.OccurredAt,
	)
	return err
}

// FetchUnpublished retrieves unpublished events for publishing.
// Orders by occurred_at to maintain event ordering.
func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT event_id, event_type, tenant_id,
			   correlation_id, causation_id,
			   payload, occurred_at, published_at
		FROM spending.outbox
		WHERE published_at IS NULL
		ORDER BY occurred_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*domain.OutboxEntry
	for rows.Next() {
		var (
			eventID       string
			eventType     string
			tenant        string
			correlationID string
			causationID   string
			payload       []byte
			occurredAt    time.Time
			publishedAt   *time.Time
		)

		if err := rows.Scan(
			&eventID, &eventType, &tenant,
			&correlationID, &causationID,
			&payload, &occurredAt, &publishedAt,
		); err != nil {
			return nil, err
		}

		entries = append(entries, &domain.OutboxEntry{
			ID:            types.EventID(eventID),
			EventType:     eventType,
			TenantID:      types.TenantID(tenant),
			CorrelationID: types.CorrelationID(correlationID),
			CausationID:   types.CausationID(causationID),
			Payload:       payload,
			OccurredAt:    occurredAt,
			PublishedAt:   publishedAt,
		})
	}

	return entries, rows.Err()
}

// MarkPublished marks events as published.
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []types.EventID) error {
	if len(ids) == 0 {
		return nil
	}

	// Convert IDs to strings for the query
	stringIDs := make([]string, len(ids))
	for i, id := range ids {
		stringIDs[i] = id.String()
	}

	_, err := r.db.Exec(ctx, `
		UPDATE spending.outbox
		SET published_at = $1
		WHERE event_id = ANY($2)`,
		time.Now(),
		stringIDs,
	)
	return err
}

// Verify interface implementation.
var _ domain.OutboxRepository = (*OutboxRepository)(nil)
