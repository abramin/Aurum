-- Performance improvements: unique constraints and optimized indexes

-- Enforce one card account per tenant (enables index-only lookups)
DROP INDEX IF EXISTS spending.idx_card_accounts_tenant;
CREATE UNIQUE INDEX idx_card_accounts_tenant_unique
    ON spending.card_accounts(tenant_id);

-- Add composite index for authorization lookups by tenant + id (covers common query)
CREATE INDEX IF NOT EXISTS idx_authorizations_tenant_id
    ON spending.authorizations(tenant_id, id);
