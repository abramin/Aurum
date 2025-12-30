-- Create schemas for each bounded context
-- Each context gets its own schema for isolation

-- Spending context: authorizations, captures, spending limits
CREATE SCHEMA IF NOT EXISTS spending;

-- Payables context: invoices, approvals, payment intents
CREATE SCHEMA IF NOT EXISTS payables;

-- Ledger context: journal entries, balance projections
CREATE SCHEMA IF NOT EXISTS ledger;

-- Bankfeed context: imported bank transactions, reconciliation
CREATE SCHEMA IF NOT EXISTS bankfeed;

-- Grant usage on schemas (for the aurum user)
GRANT USAGE ON SCHEMA spending TO aurum;
GRANT USAGE ON SCHEMA payables TO aurum;
GRANT USAGE ON SCHEMA ledger TO aurum;
GRANT USAGE ON SCHEMA bankfeed TO aurum;

-- Grant all privileges on tables in schemas
ALTER DEFAULT PRIVILEGES IN SCHEMA spending GRANT ALL ON TABLES TO aurum;
ALTER DEFAULT PRIVILEGES IN SCHEMA payables GRANT ALL ON TABLES TO aurum;
ALTER DEFAULT PRIVILEGES IN SCHEMA ledger GRANT ALL ON TABLES TO aurum;
ALTER DEFAULT PRIVILEGES IN SCHEMA bankfeed GRANT ALL ON TABLES TO aurum;
