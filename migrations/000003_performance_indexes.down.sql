-- Rollback performance indexes

DROP INDEX IF EXISTS spending.idx_authorizations_tenant_id;
DROP INDEX IF EXISTS spending.idx_card_accounts_tenant_unique;

-- Restore original non-unique index
CREATE INDEX IF NOT EXISTS idx_card_accounts_tenant
    ON spending.card_accounts(tenant_id);
