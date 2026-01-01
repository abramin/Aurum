-- name: InsertOutboxEntry :exec
INSERT INTO spending.outbox (
	event_id, event_type, tenant_id,
	correlation_id, causation_id,
	payload, occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListUnpublishedOutbox :many
SELECT event_id, event_type, tenant_id,
	   COALESCE(correlation_id, '') AS correlation_id,
	   COALESCE(causation_id, '') AS causation_id,
	   payload, occurred_at, published_at
FROM spending.outbox
WHERE published_at IS NULL
ORDER BY occurred_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxPublished :exec
UPDATE spending.outbox
SET published_at = $1
WHERE event_id = ANY(sqlc.arg(event_ids)::varchar[]);
