# PRD: Finance Workspace Core (Spend + Invoices + Payments + Reconcile) in Go

## 1. Summary

Build a small but realistic “mini Qonto” backend that combines:
- Spend management (authorization, capture, limits)
- Payables (invoices, approvals)
- Payments orchestration (provider adapter, async settlement)
- Bank-feed import and reconciliation
- Double-entry ledger with balance projection

The point is not feature breadth. The point is strong systems design narratives (consistency, idempotency, failure modes) and DDD modeling (bounded contexts, aggregates, invariants, anti-corruption layers).

## 2. Goals

### Product goals
- Provide a coherent backend slice that feels like a finance workspace core.
- Demonstrate robust handling of partial failures and retries.
- Demonstrate auditability and correctness through an immutable ledger and explicit state machines.

### Engineering goals
- Practice DDD boundaries: clear bounded contexts, small aggregates, domain invariants enforced in domain layer.
- Practice event-driven architecture: outbox pattern, consumer dedupe, eventual consistency.
- Practice system design fundamentals: idempotency, pagination, reliability, observability, resilience.

## 3. Non-goals

- Real money movement, PCI compliance, real KYC.
- Full auth, full RBAC, multi-region, and production-grade SRE posture.
- UI beyond minimal API usage (curl/Postman).
- Complex currency FX, fees, chargebacks.
- Full accounting exports (Xero, Sage) beyond mocked placeholders.

## 4. Personas and Use cases

### Persona A: SME admin user
- Creates invoices, submits and approves them.
- Initiates payment on approved invoices.
- Views balances and transaction history.

### Persona B: System operator
- Monitors worker health, dead letters, retry backlogs.
- Traces a transaction from API call to ledger entry.

### Persona C: Bank feed job
- Imports bank transactions (mocked feed).
- System reconciles them to internal payments.

## 5. Key user stories

1) As an SME admin, I can authorize and capture a spend transaction, and see it reflected in my ledger and balance.
2) As an SME admin, I can create an invoice, submit and approve it, initiate payment, and later see it settled.
3) As an SME admin, I can import bank transactions and see payments reconciled (matched) or flagged unmatched.
4) As an operator, I can trace each workflow end-to-end via correlation IDs and event logs.

## 6. Scope and deliverables

### Must-have (Week scope)
- Spend API:
  - Create authorization (idempotent)
  - Capture authorization (idempotent)
  - Enforce a simple per-account spending limit
- Payables API:
  - Create invoice (draft)
  - Submit invoice
  - Approve invoice
  - Create payment intent (implicit on approval or explicit endpoint)
- Payments:
  - Orchestrate payment initiation via Provider adapter
  - Receive async settlement events (simulated webhook or Kafka topic)
- Bank feed + Reconciliation:
  - Import bank transaction records (mock source)
  - Reconcile by reference to payment intent
- Ledger:
  - Double-entry postings for spend capture and payment settlement
  - Balance projection per account
  - Transaction history query with cursor pagination
- Reliability primitives:
  - Outbox publisher in each context that emits domain events reliably
  - Consumer dedupe using event IDs
  - Retries with backoff for transient failures
- Observability:
  - Structured logs with correlation ID
  - Minimal metrics (counts, latency histograms, consumer lag)
  - Tracing optional but nice-to-have

### Nice-to-have (if time allows)
- Simple “unmatched transactions” workflow
- Dead-letter queue topic and UI-less inspection endpoint
- Basic RBAC or staff impersonation guardrails
- Separate Postgres databases per service rather than schemas

## 7. High-level architecture

### Bounded contexts and services (recommended)
1) **Spending Context**
   - Service: `spending-api`
   - Owns: Authorizations, Captures, Limits, Spending events

2) **Payables Context**
   - Service: `payables-api`
   - Owns: Invoices, Approvals, PaymentIntents (the intent, not settlement)

3) **Payments Context**
   - Service: `payments-orchestrator` (worker)
   - Owns: Payment execution lifecycle (Initiated, Settled, Failed)
   - Integrates with external Payment Provider via an Anti-Corruption Layer

4) **Ledger Context**
   - Service: `ledger-service` (worker + query API or separate `ledger-api`)
   - Owns: Append-only journal entries and balance projections

5) **Bankfeed Context**
   - Service: `bankfeed-importer` (worker)
   - Owns: Imported bank transactions
   - Emits: `BankTransactionImported` events for reconciliation

### Data and messaging
- Postgres for each context (ideal), otherwise one Postgres with separate schemas.
- Kafka topics for domain events (NATS acceptable for local simplicity).
- Outbox pattern per service to publish events reliably:
  - Write domain state + outbox record in same DB transaction
  - Publisher reads outbox and publishes to Kafka
  - Mark outbox record as published

### Eventing conventions
- Every event has:
  - `event_id` (UUID)
  - `event_type`
  - `occurred_at`
  - `tenant_id`
  - `correlation_id`
  - `causation_id` (optional)
  - `payload` (json)

## 8. Domain design (DDD)

### 8.1 Spending context

**Aggregates**
- `Authorization` (aggregate root)
  - States: `Authorized`, `Captured`, `Reversed`, `Expired`
  - Invariants:
    - Cannot capture more than authorized amount
    - Cannot capture twice
    - Capture requires `Authorized` state
- `CardAccount` (aggregate root or entity depending on implementation)
  - Has `SpendingLimit` policy and rolling spend counters (simple)

**Value objects**
- `Money` (amount + currency)
- `MerchantRef`, `AuthorizationRef`, `IdempotencyKey`

**Domain events**
- `SpendAuthorized`
- `SpendCaptured`
- `SpendLimitBreached` (optional)
- `SpendReversed` (optional)

### 8.2 Payables context

**Aggregates**
- `Invoice` (aggregate root)
  - States: `Draft`, `Submitted`, `Approved`, `Rejected`, `Paid`
  - Invariants:
    - Only `Draft` can be submitted
    - Only `Submitted` can be approved or rejected
    - Paid only after a settled payment
- `PaymentIntent` (aggregate root)
  - States: `Created`, `Initiated`, `Settled`, `Failed`, `Reconciled`
  - Invariants:
    - Only `Created` can be initiated
    - Only `Initiated` can become `Settled` or `Failed`

**Domain events**
- `InvoiceCreated`
- `InvoiceSubmitted`
- `InvoiceApproved`
- `PaymentIntentCreated`
- `PaymentIntentInitiated` (produced by payments worker or payables depending on design)

### 8.3 Payments context

**Anti-Corruption Layer**
- `PaymentProvider` interface:
  - `InitiatePayment(intent) -> provider_payment_id`
  - `GetStatus(provider_payment_id) -> status` (optional)
- Adapter: `MockProvider`
  - Simulates async settlement by emitting `PaymentSettled` or `PaymentFailed` later.

**Domain events**
- `PaymentInitiated`
- `PaymentSettled`
- `PaymentFailed`

### 8.4 Ledger context

**Model**
- Double-entry ledger entries:
  - Each posting is a set of lines with debit/credit accounts
  - Must balance to zero (sum of signed amounts equals 0)

**Invariants**
- Each journal posting must be balanced
- Journal is append-only
- Balance projection is derived and can be rebuilt

**Domain events consumed**
- `SpendCaptured` -> ledger posting: debit expense, credit cash
- `PaymentSettled` -> ledger posting: debit accounts payable, credit cash

**Domain events produced**
- `LedgerPosted` (optional, mostly for debugging)

### 8.5 Bankfeed context and reconciliation

**BankTransaction**
- Imported record with:
  - reference
  - amount/currency
  - date
  - counterparty
- Reconciliation logic:
  - Match on `reference` -> PaymentIntent
  - If match found and amounts align, mark PaymentIntent `Reconciled`
  - Else create “unmatched” record

**Domain events**
- `BankTransactionImported`
- `PaymentReconciled` (optional)

## 9. API surface (MVP)

### 9.1 Spending API

#### POST /authorizations
Creates an authorization hold.

Request
```json
{
  "tenant_id": "t_123",
  "account_id": "a_123",
  "idempotency_key": "idem_abc",
  "amount": { "currency": "EUR", "value": "12.34" },
  "merchant_ref": "m_456",
  "reference": "auth_ref_001"
}
````

Response

```json
{
  "authorization_id": "auth_123",
  "status": "AUTHORIZED"
}
```

Rules

* Idempotent by `(tenant_id, idempotency_key)`
* Enforces spending limit (simple: daily limit or fixed available limit)
* Emits `SpendAuthorized` via outbox

#### POST /authorizations/{id}/capture

Captures an authorization.

Request

```json
{
  "tenant_id": "t_123",
  "idempotency_key": "idem_def",
  "amount": { "currency": "EUR", "value": "12.34" }
}
```

Response

```json
{
  "authorization_id": "auth_123",
  "status": "CAPTURED"
}
```

Rules

* Idempotent by `(tenant_id, idempotency_key)`
* Cannot exceed authorized amount
* Emits `SpendCaptured` via outbox

### 9.2 Payables API

#### POST /invoices

Request

```json
{
  "tenant_id": "t_123",
  "supplier": "ACME SAS",
  "amount": { "currency": "EUR", "value": "199.00" },
  "reference": "inv_2026_0001"
}
```

Response

```json
{ "invoice_id": "inv_123", "status": "DRAFT" }
```

#### POST /invoices/{id}/submit

Request

```json
{ "tenant_id": "t_123", "idempotency_key": "idem_x1" }
```

Response

```json
{ "invoice_id": "inv_123", "status": "SUBMITTED" }
```

#### POST /invoices/{id}/approve

Request

```json
{ "tenant_id": "t_123", "approver_id": "u_999", "idempotency_key": "idem_x2" }
```

Response

```json
{
  "invoice_id": "inv_123",
  "status": "APPROVED",
  "payment_intent_id": "pi_123"
}
```

Rules

* On approval, create PaymentIntent in `Created` state
* Emit `InvoiceApproved` and `PaymentIntentCreated`

### 9.3 Ledger Query API

#### GET /accounts/{id}/balance?tenant_id=...

Response

```json
{
  "tenant_id": "t_123",
  "account_id": "a_123",
  "currency": "EUR",
  "balance": "1234.56",
  "as_of": "2026-01-03T12:00:00Z"
}
```

#### GET /accounts/{id}/transactions?tenant_id=...&cursor=...&limit=...

Response

```json
{
  "items": [
    { "type": "SPEND_CAPTURE", "amount": "-12.34", "ref": "auth_123", "ts": "..." },
    { "type": "PAYMENT_SETTLED", "amount": "-199.00", "ref": "pi_123", "ts": "..." }
  ],
  "next_cursor": "..."
}
```

## 10. Event flows (end-to-end)

### Flow A: Spend capture -> ledger update

1. Spending API captures authorization
2. In same DB transaction: update authorization state + insert outbox `SpendCaptured`
3. Outbox publisher publishes to Kafka topic `spending.events`
4. Ledger worker consumes `SpendCaptured`
5. Ledger posts balanced journal entry and updates balance projection
6. Ledger query reflects new balance

### Flow B: Invoice approval -> payment -> settlement -> ledger

1. Payables API approves invoice
2. In same DB transaction: update invoice + create PaymentIntent + outbox `InvoiceApproved`, `PaymentIntentCreated`
3. Payments worker consumes `PaymentIntentCreated` and initiates payment via Provider
4. Payments worker marks intent Initiated and emits `PaymentInitiated`
5. Mock Provider emits `PaymentSettled` later
6. Payments worker updates PaymentIntent to Settled, emits `PaymentSettled`
7. Ledger worker consumes `PaymentSettled` and posts journal entry

### Flow C: Bank transaction import -> reconciliation

1. Bankfeed importer reads mock feed and writes `bank_transactions` + outbox `BankTransactionImported`
2. Reconciliation consumer matches to PaymentIntent by reference
3. Mark PaymentIntent Reconciled (and optionally emit `PaymentReconciled`)

## 11. Data model (minimum tables)

### Spending DB

* `authorizations`:

  * id, tenant_id, account_id, amount_value, currency, status, reference, created_at, updated_at
* `idempotency_keys`:

  * tenant_id, key, request_hash, response_body, created_at
* `outbox`:

  * event_id, event_type, payload_json, correlation_id, created_at, published_at

### Payables DB

* `invoices`:

  * id, tenant_id, supplier, amount_value, currency, status, reference, created_at, updated_at
* `payment_intents`:

  * id, tenant_id, invoice_id, amount_value, currency, status, provider_payment_id, reference, created_at, updated_at
* `idempotency_keys`
* `outbox`

### Ledger DB

* `journal_entries`:

  * id, tenant_id, ref_type, ref_id, created_at, correlation_id
* `journal_lines`:

  * entry_id, account, direction, amount_value, currency
* `balances_projection`:

  * tenant_id, account_id, currency, balance_value, updated_at
* `consumer_dedupe`:

  * consumer_name, event_id, processed_at

### Bankfeed DB

* `bank_transactions`:

  * id, tenant_id, reference, amount_value, currency, booked_at, counterparty, raw_json
* `outbox`
* `consumer_dedupe` (optional)

## 12. Reliability and failure handling

### Idempotency

* All state-changing endpoints accept `idempotency_key`
* Store response by `(tenant_id, idempotency_key, endpoint)` and replay it on retry

### Outbox

* Atomicity: state + outbox in one transaction
* Publisher retries publish failures, marks published_at only on success

### Consumer dedupe

* Maintain `consumer_dedupe` table keyed by `(consumer_name, event_id)`
* If event already processed, ack and skip

### Retries

* Exponential backoff for transient errors
* Poison messages go to DLQ topic after N attempts (nice-to-have if time)

### Consistency

* Ledger and balances are eventually consistent relative to API writes
* APIs should return 202 or reflect local state only, but document “balance may lag by seconds”

## 13. Security and compliance (MVP posture)

* Tenant isolation: every query and write requires `tenant_id` and validates it
* Correlation IDs: generated per request and propagated into events
* Logging: structured logs, no raw PII fields
* Secrets: env vars, local dev uses .env, no secrets committed

## 14. Observability

### Logs

* JSON logs with fields:

  * service, env, tenant_id, correlation_id, event_id, entity_id, error_code
* Log key transitions:

  * authorization captured
  * invoice approved
  * payment initiated/settled
  * ledger posting created
  * reconciliation matched/unmatched

### Metrics (minimal)

* HTTP:

  * request count, latency
* Workers:

  * messages consumed, failures, retries
* Outbox:

  * backlog size, publish latency

### Tracing (optional)

* OpenTelemetry with traceparent propagation to Kafka headers

## 15. Local development setup

* Docker Compose:

  * Postgres (one per service or one with schemas)
  * Kafka + Zookeeper (or Redpanda)
* Makefile targets:

  * `make up`, `make down`
  * `make migrate`
  * `make test`
  * `make run-spending`, `make run-payables`, `make run-ledger`, `make run-workers`

## 16. Testing strategy

### Domain tests

* Spending:

  * capture cannot exceed authorized
  * double capture rejected
* Payables:

  * approve only from submitted
  * payment intent cannot initiate twice
* Ledger:

  * posting must balance
  * projection updates correctly
* Reconciliation:

  * match by reference and amount rules

### Integration tests

* Outbox publisher publishes events after DB transaction
* Consumer dedupe prevents double-processing

### End-to-end test

* Happy path:

  * authorize -> capture
  * create invoice -> submit -> approve
  * payments settle
  * ledger shows expected balance and transactions

## 17. Acceptance criteria (MVP)

* Spend capture results in a ledger transaction and balance change within a short time window (seconds).
* Invoice approval results in a PaymentIntent; later settlement results in a ledger posting.
* Re-running the same endpoint call with the same idempotency key returns the same response and does not duplicate state.
* Replaying the same event does not duplicate ledger postings (consumer dedupe).
* System can explain the end-to-end flow with correlation IDs in logs.

## 18. Week plan (suggested slicing)

Day 1

* Repo skeleton, common libs (logging, config), Postgres migrations, docker compose
* Spending domain model + endpoints

Day 2

* Outbox pattern + publisher for spending
* Ledger worker consumes `SpendCaptured` and posts journal entry + projection

Day 3

* Payables domain model + invoice endpoints
* PaymentIntent creation on approval + outbox

Day 4

* Payments worker + Provider adapter + async settlement simulation
* Ledger consumes `PaymentSettled`

Day 5

* Bankfeed importer + reconciliation
* E2E tests, docs, diagrams, runbook

## 19. Open questions (to decide early)

* Should PaymentIntent live in Payables or Payments context (ownership)? Recommended: Payables owns intent, Payments owns execution state.
* One Postgres with schemas vs separate DBs? Separate is more “real”, schemas are faster to build.
* Kafka vs NATS for local dev? Kafka matches Qonto, NATS is faster to run.
* Ledger accounts naming: keep it simple (Cash, Expenses, AccountsPayable).

## 20. Documentation to include in repo

* `docs/architecture.md`:

  * context map diagram (bounded contexts and event flows)
* `docs/api.md`:

  * endpoint list + examples
* `docs/events.md`:

  * event schemas
* `docs/operability.md`:

  * how to run locally, how to inspect outbox, how to trace correlation IDs
