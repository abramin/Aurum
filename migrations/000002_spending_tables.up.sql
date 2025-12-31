-- Spending context tables
-- Authorization and capture functionality with spending limits

-- Card accounts with spending limits and rolling spend tracking
CREATE TABLE spending.card_accounts (
    id UUID PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    spending_limit_amount DECIMAL(19, 4) NOT NULL,
    spending_limit_currency VARCHAR(3) NOT NULL,
    rolling_spend_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    rolling_spend_currency VARCHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_spending_limit_positive CHECK (spending_limit_amount >= 0),
    CONSTRAINT chk_rolling_spend_non_negative CHECK (rolling_spend_amount >= 0),
    CONSTRAINT chk_currency_match CHECK (spending_limit_currency = rolling_spend_currency)
);

-- Index for tenant lookups
CREATE INDEX idx_card_accounts_tenant_id ON spending.card_accounts(tenant_id);

-- Authorizations with state machine
CREATE TABLE spending.authorizations (
    id UUID PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    card_account_id UUID NOT NULL REFERENCES spending.card_accounts(id),
    authorized_amount DECIMAL(19, 4) NOT NULL,
    authorized_currency VARCHAR(3) NOT NULL,
    captured_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    captured_currency VARCHAR(3) NOT NULL,
    state VARCHAR(20) NOT NULL DEFAULT 'authorized',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_authorized_amount_positive CHECK (authorized_amount > 0),
    CONSTRAINT chk_captured_amount_non_negative CHECK (captured_amount >= 0),
    CONSTRAINT chk_state_valid CHECK (state IN ('authorized', 'captured', 'reversed', 'expired')),
    CONSTRAINT chk_auth_currency_match CHECK (authorized_currency = captured_currency)
);

-- Index for tenant + authorization lookups
CREATE INDEX idx_authorizations_tenant_id ON spending.authorizations(tenant_id);
CREATE INDEX idx_authorizations_card_account_id ON spending.authorizations(card_account_id);

-- Idempotency keys for spending context
-- Prevents duplicate authorization creation
CREATE TABLE spending.idempotency_keys (
    tenant_id VARCHAR(255) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    resource_id VARCHAR(255) NOT NULL,
    status_code INTEGER NOT NULL,
    response_body BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, idempotency_key)
);

-- Add version column for optimistic locking
ALTER TABLE spending.card_accounts ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE spending.authorizations ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
