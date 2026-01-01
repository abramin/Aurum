-- name: GetIdempotencyEntry :one
SELECT tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
FROM spending.idempotency_keys
WHERE tenant_id = $1 AND idempotency_key = $2;

-- name: UpsertIdempotencyEntry :exec
INSERT INTO spending.idempotency_keys (
	tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (tenant_id, idempotency_key) DO UPDATE SET
	resource_id = EXCLUDED.resource_id,
	status_code = EXCLUDED.status_code,
	response_body = EXCLUDED.response_body;

-- name: InsertIdempotencyIfAbsent :one
WITH new_row AS (
	INSERT INTO spending.idempotency_keys (
		tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
	) VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
	RETURNING tenant_id, idempotency_key, resource_id, status_code, response_body, created_at, true AS inserted
)
SELECT tenant_id, idempotency_key, resource_id, status_code, response_body, created_at, inserted
FROM new_row
UNION ALL
SELECT tenant_id, idempotency_key, resource_id, status_code, response_body, created_at, false AS inserted
FROM spending.idempotency_keys
WHERE tenant_id = $1 AND idempotency_key = $2
	AND NOT EXISTS (SELECT 1 FROM new_row);
