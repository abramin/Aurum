-- Drop all bounded context schemas
-- WARNING: This will delete all data in these schemas

DROP SCHEMA IF EXISTS bankfeed CASCADE;
DROP SCHEMA IF EXISTS ledger CASCADE;
DROP SCHEMA IF EXISTS payables CASCADE;
DROP SCHEMA IF EXISTS spending CASCADE;
