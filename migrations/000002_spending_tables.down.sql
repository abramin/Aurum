-- Drop Spending context tables in reverse order of creation

DROP TABLE IF EXISTS spending.outbox;
DROP TABLE IF EXISTS spending.idempotency_keys;
DROP TABLE IF EXISTS spending.authorizations;
DROP TABLE IF EXISTS spending.card_accounts;
