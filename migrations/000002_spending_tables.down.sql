-- Drop spending context tables
-- Order matters due to foreign key constraints

DROP TABLE IF EXISTS spending.idempotency_keys;
DROP TABLE IF EXISTS spending.authorizations;
DROP TABLE IF EXISTS spending.card_accounts;
