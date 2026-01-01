-- name: UpsertCardAccount :execrows
INSERT INTO spending.card_accounts (
	id, tenant_id,
	spending_limit_amount, spending_limit_currency,
	rolling_spend_amount, rolling_spend_currency,
	version, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
	rolling_spend_amount = EXCLUDED.rolling_spend_amount,
	rolling_spend_currency = EXCLUDED.rolling_spend_currency,
	version = EXCLUDED.version,
	updated_at = EXCLUDED.updated_at
WHERE spending.card_accounts.version = EXCLUDED.version - 1;

-- name: GetCardAccountByID :one
SELECT id, tenant_id,
	   spending_limit_amount, spending_limit_currency,
	   rolling_spend_amount, rolling_spend_currency,
	   version, created_at, updated_at
FROM spending.card_accounts
WHERE id = $1 AND tenant_id = $2;

-- name: GetCardAccountByTenantID :one
SELECT id, tenant_id,
	   spending_limit_amount, spending_limit_currency,
	   rolling_spend_amount, rolling_spend_currency,
	   version, created_at, updated_at
FROM spending.card_accounts
WHERE tenant_id = $1
LIMIT 1;
