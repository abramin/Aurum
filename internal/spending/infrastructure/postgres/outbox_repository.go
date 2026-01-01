package postgres

import (
	"context"
	"fmt"
	"time"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/postgres/sqlc"
)

// OutboxRepository implements domain.OutboxRepository using PostgreSQL.
// This implements the outbox pattern for reliable event publishing.
//
// Events are written to the outbox within the same transaction as domain changes,
// then published asynchronously by a separate process (outbox publisher).
type OutboxRepository struct {
	queries *sqlc.Queries
}

// NewOutboxRepository creates a new OutboxRepository.
func NewOutboxRepository(db sqlc.DBTX) *OutboxRepository {
	return &OutboxRepository{queries: sqlc.New(db)}
}

// Append adds an event to the outbox.
// It persists the event payload and metadata as part of the current transaction.
func (r *OutboxRepository) Append(ctx context.Context, entry *domain.OutboxEntry) error {
	return r.queries.InsertOutboxEntry(ctx, sqlc.InsertOutboxEntryParams{
		EventID:       entry.ID.String(),
		EventType:     entry.EventType,
		TenantID:      entry.TenantID.String(),
		CorrelationID: textFromString(string(entry.CorrelationID)),
		CausationID:   textFromString(string(entry.CausationID)),
		Payload:       entry.Payload,
		OccurredAt:    timeToTimestamptz(entry.OccurredAt),
	})
}

// FetchUnpublished retrieves unpublished events for publishing.
// It locks rows with FOR UPDATE SKIP LOCKED to support concurrent publishers,
// ordering by occurred_at to maintain event ordering.
func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
	rows, err := r.queries.ListUnpublishedOutbox(ctx, int32(limit))
	if err != nil {
		return nil, err
	}

	entries := make([]*domain.OutboxEntry, 0, len(rows))
	for _, row := range rows {
		occurredAt, err := timestamptzToTime(row.OccurredAt)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid occurred_at: %v", domain.ErrCorruptData, err)
		}
		publishedAt, err := timestamptzToTimePtr(row.PublishedAt)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid published_at: %v", domain.ErrCorruptData, err)
		}

		entries = append(entries, &domain.OutboxEntry{
			ID:            types.EventID(row.EventID),
			EventType:     row.EventType,
			TenantID:      types.TenantID(row.TenantID),
			CorrelationID: types.CorrelationID(row.CorrelationID),
			CausationID:   types.CausationID(row.CausationID),
			Payload:       row.Payload,
			OccurredAt:    occurredAt,
			PublishedAt:   publishedAt,
		})
	}

	return entries, nil
}

// MarkPublished marks events as published.
// It is a no-op when the input list is empty.
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []types.EventID) error {
	if len(ids) == 0 {
		return nil
	}

	stringIDs := make([]string, len(ids))
	for i, id := range ids {
		stringIDs[i] = id.String()
	}

	return r.queries.MarkOutboxPublished(ctx, sqlc.MarkOutboxPublishedParams{
		PublishedAt: timeToTimestamptz(time.Now()),
		EventIds:    stringIDs,
	})
}

// Verify interface implementation.
var _ domain.OutboxRepository = (*OutboxRepository)(nil)
