-- name: UpsertAuthorization :execrows
INSERT INTO spending.authorizations (
	id, tenant_id, card_account_id,
	authorized_amount, authorized_currency,
	captured_amount, captured_currency,
	merchant_ref, reference, state, version,
	created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (id) DO UPDATE SET
	captured_amount = EXCLUDED.captured_amount,
	captured_currency = EXCLUDED.captured_currency,
	state = EXCLUDED.state,
	version = EXCLUDED.version,
	updated_at = EXCLUDED.updated_at
WHERE spending.authorizations.version = EXCLUDED.version - 1;

-- name: GetAuthorizationByID :one
SELECT id, tenant_id, card_account_id,
	   authorized_amount, authorized_currency,
	   captured_amount, captured_currency,
	   COALESCE(merchant_ref, '') AS merchant_ref,
	   COALESCE(reference, '') AS reference,
	   state, version, created_at, updated_at
FROM spending.authorizations
WHERE id = $1 AND tenant_id = $2;
