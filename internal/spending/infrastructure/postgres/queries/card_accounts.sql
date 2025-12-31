-- name: GetCardAccount :one
SELECT id, tenant_id,
       spending_limit_amount, spending_limit_currency,
       rolling_spend_amount, rolling_spend_currency,
       version, created_at, updated_at
FROM spending.card_accounts
WHERE tenant_id = $1 AND id = $2;

-- name: GetCardAccountByTenant :one
SELECT id, tenant_id,
       spending_limit_amount, spending_limit_currency,
       rolling_spend_amount, rolling_spend_currency,
       version, created_at, updated_at
FROM spending.card_accounts
WHERE tenant_id = $1
LIMIT 1;

-- name: InsertCardAccount :exec
INSERT INTO spending.card_accounts (
    id, tenant_id,
    spending_limit_amount, spending_limit_currency,
    rolling_spend_amount, rolling_spend_currency,
    version, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, 1, NOW(), NOW()
);

-- name: UpdateCardAccount :execrows
UPDATE spending.card_accounts
SET rolling_spend_amount = $3,
    version = version + 1,
    updated_at = NOW()
WHERE tenant_id = $1 AND id = $2 AND version = $4;
