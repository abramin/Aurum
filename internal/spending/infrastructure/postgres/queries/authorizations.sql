-- name: GetAuthorization :one
SELECT id, tenant_id, card_account_id,
       authorized_amount, authorized_currency,
       captured_amount, captured_currency,
       state, version, created_at, updated_at
FROM spending.authorizations
WHERE tenant_id = $1 AND id = $2;

-- name: InsertAuthorization :exec
INSERT INTO spending.authorizations (
    id, tenant_id, card_account_id,
    authorized_amount, authorized_currency,
    captured_amount, captured_currency,
    state, version, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, 1, NOW(), NOW()
);

-- name: UpdateAuthorization :execrows
UPDATE spending.authorizations
SET captured_amount = $3,
    state = $4,
    version = version + 1,
    updated_at = NOW()
WHERE tenant_id = $1 AND id = $2 AND version = $5;
