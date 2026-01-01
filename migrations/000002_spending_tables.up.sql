-- Spending context tables
-- Following the Qonto pattern: atomic operations require all related data
-- (entities + events) to be written in a single transaction

-- Card accounts with spending limits
CREATE TABLE spending.card_accounts (
    id UUID PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    spending_limit_amount DECIMAL(19, 4) NOT NULL,
    spending_limit_currency VARCHAR(3) NOT NULL,
    rolling_spend_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    rolling_spend_currency VARCHAR(3) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_spending_limit_positive CHECK (spending_limit_amount >= 0),
    CONSTRAINT chk_rolling_spend_non_negative CHECK (rolling_spend_amount >= 0),
    CONSTRAINT chk_currency_match CHECK (spending_limit_currency = rolling_spend_currency)
);

CREATE INDEX idx_card_accounts_tenant ON spending.card_accounts(tenant_id);

-- Authorizations
CREATE TABLE spending.authorizations (
    id UUID PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    card_account_id UUID NOT NULL REFERENCES spending.card_accounts(id),
    authorized_amount DECIMAL(19, 4) NOT NULL,
    authorized_currency VARCHAR(3) NOT NULL,
    captured_amount DECIMAL(19, 4) NOT NULL DEFAULT 0,
    captured_currency VARCHAR(3) NOT NULL,
    merchant_ref VARCHAR(255),
    reference VARCHAR(255),
    state VARCHAR(20) NOT NULL DEFAULT 'authorized',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_authorized_amount_positive CHECK (authorized_amount > 0),
    CONSTRAINT chk_captured_amount_non_negative CHECK (captured_amount >= 0),
    CONSTRAINT chk_state_valid CHECK (state IN ('authorized', 'captured', 'reversed', 'expired')),
    CONSTRAINT chk_auth_currency_match CHECK (authorized_currency = captured_currency)
);

CREATE INDEX idx_authorizations_tenant ON spending.authorizations(tenant_id);
CREATE INDEX idx_authorizations_card_account ON spending.authorizations(card_account_id);
CREATE INDEX idx_authorizations_state ON spending.authorizations(state);

-- Idempotency keys for replay protection
CREATE TABLE spending.idempotency_keys (
    tenant_id VARCHAR(255) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    resource_id VARCHAR(255) NOT NULL,
    status_code INTEGER NOT NULL,
    response_body BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, idempotency_key)
);

-- Outbox for reliable event publishing (Qonto pattern)
-- Events are written atomically with domain changes, then published asynchronously
CREATE TABLE spending.outbox (
    event_id VARCHAR(255) PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    correlation_id VARCHAR(255),
    causation_id VARCHAR(255),
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ,

    CONSTRAINT chk_event_type_not_empty CHECK (event_type <> '')
);

CREATE INDEX idx_outbox_unpublished ON spending.outbox(occurred_at) WHERE published_at IS NULL;
CREATE INDEX idx_outbox_tenant ON spending.outbox(tenant_id);
