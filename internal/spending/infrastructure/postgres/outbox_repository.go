package postgres

import (
	"context"
	"fmt"
	"time"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/postgres/sqlc"
)

// OutboxRepository persists OutboxEntry records to implement the outbox pattern.
// Events should be appended within the same transaction as aggregate changes,
// then published asynchronously by a separate process.
type OutboxRepository struct {
	queries *sqlc.Queries
}

// NewOutboxRepository binds sqlc queries to a database handle (pool or tx).
// Callers control transactional scope by passing a pgx.Tx when participating in a unit of work.
func NewOutboxRepository(db sqlc.DBTX) *OutboxRepository {
	return &OutboxRepository{queries: sqlc.New(db)}
}

// Append adds an event to the outbox.
// Side effects: writes to spending.outbox.
// Usage: call within the same transaction as the aggregate change to preserve atomicity.
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
// It uses FOR UPDATE SKIP LOCKED to support concurrent publishers and orders by occurred_at.
// Concurrency: wrap this call in a transaction if you need locks to survive until MarkPublished.
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

// MarkPublished marks events as published with the current time.
// Side effects: writes to spending.outbox.
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
