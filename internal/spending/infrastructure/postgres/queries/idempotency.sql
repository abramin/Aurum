-- name: GetIdempotencyKey :one
SELECT tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
FROM spending.idempotency_keys
WHERE tenant_id = $1 AND idempotency_key = $2;

-- name: InsertIdempotencyKey :exec
INSERT INTO spending.idempotency_keys (
    tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
) VALUES (
    $1, $2, $3, $4, $5, NOW()
);

-- name: InsertIdempotencyKeyIfAbsent :one
INSERT INTO spending.idempotency_keys (
    tenant_id, idempotency_key, resource_id, status_code, response_body, created_at
) VALUES (
    $1, $2, $3, $4, $5, NOW()
)
ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
RETURNING tenant_id, idempotency_key, resource_id, status_code, response_body, created_at;
