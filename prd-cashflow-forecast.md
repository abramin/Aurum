# Cashflow Forecast Service PRD (Ruby)

## 1. Summary
The Cashflow Forecast Service provides forward-looking daily balance projections for each tenant using historical ledger balance plus scheduled payables. It emits alerts when projected balances drop below configured thresholds and exposes a read API for dashboards and reports.

## 2. Goals
- Generate daily projected balances for a configurable horizon (default 90 days).
- Update projections near real time as new events arrive.
- Provide traceability from projected balances to source events or assumptions.

## 3. Non-Goals
- No payment execution or ledger mutation.
- No ML-based forecasting in v1.
- No user-facing UI beyond JSON API.

## 4. Users
- SME admins and finance operators.
- Internal services that render dashboards and alerts.

## 5. Service Boundary
**Service name:** `cashflow-forecast-service`  
**Language:** Ruby 3.3  
**Responsibility:** Compute and serve projections; emit alert events.  
**Does not own:** Payment or ledger authoritative state.

## 6. Inputs (Events Consumed)
All events use the standard envelope:
```
{
  "event_id": "uuid",
  "event_type": "string",
  "occurred_at": "RFC3339 timestamp",
  "tenant_id": "uuid",
  "correlation_id": "uuid",
  "causation_id": "uuid|null",
  "payload": { ... }
}
```

### 6.1 LedgerPostingCreated (from ledger-service)
```
{
  "event_type": "LedgerPostingCreated",
  "payload": {
    "posting_id": "uuid",
    "amount": "-123.45",
    "currency": "USD",
    "effective_date": "YYYY-MM-DD",
    "account": "cash",
    "source_type": "SpendCaptured|PaymentSettled|Adjustment",
    "source_id": "uuid"
  }
}
```

### 6.2 PaymentIntentCreated (from payables-api)
```
{
  "event_type": "PaymentIntentCreated",
  "payload": {
    "payment_intent_id": "uuid",
    "amount": "-123.45",
    "currency": "USD",
    "scheduled_settlement_date": "YYYY-MM-DD",
    "vendor_id": "uuid",
    "invoice_id": "uuid"
  }
}
```

### 6.3 PaymentIntentInitiated / Settled / Failed
```
{
  "event_type": "PaymentIntentInitiated",
  "payload": {
    "payment_intent_id": "uuid",
    "amount": "-123.45",
    "currency": "USD",
    "scheduled_settlement_date": "YYYY-MM-DD"
  }
}
```
```
{
  "event_type": "PaymentIntentSettled",
  "payload": {
    "payment_intent_id": "uuid",
    "amount": "-123.45",
    "currency": "USD",
    "settled_date": "YYYY-MM-DD"
  }
}
```
```
{
  "event_type": "PaymentIntentFailed",
  "payload": {
    "payment_intent_id": "uuid",
    "amount": "-123.45",
    "currency": "USD",
    "failed_at": "YYYY-MM-DD",
    "reason": "string"
  }
}
```

### 6.4 InvoiceApproved (optional)
Used when a payment intent is not yet created. This is a weaker signal and will be labeled as an assumption.
```
{
  "event_type": "InvoiceApproved",
  "payload": {
    "invoice_id": "uuid",
    "amount": "-123.45",
    "currency": "USD",
    "due_date": "YYYY-MM-DD",
    "vendor_id": "uuid"
  }
}
```

## 7. Outputs (Events Produced)
### 7.1 CashflowProjectionUpdated
```
{
  "event_type": "CashflowProjectionUpdated",
  "payload": {
    "projection_run_id": "uuid",
    "from_date": "YYYY-MM-DD",
    "to_date": "YYYY-MM-DD",
    "as_of": "RFC3339 timestamp"
  }
}
```

### 7.2 LowBalanceProjected
```
{
  "event_type": "LowBalanceProjected",
  "payload": {
    "date": "YYYY-MM-DD",
    "projected_balance": "-12.34",
    "currency": "USD",
    "threshold": "50.00"
  }
}
```

## 8. API (HTTP/JSON)
- `GET /cashflow?from=YYYY-MM-DD&to=YYYY-MM-DD`  
  Returns daily projected balances with net change and summary drivers.
- `GET /cashflow/drivers?date=YYYY-MM-DD`  
  Returns detailed drivers for a specific date.
- `POST /cashflow/recompute`  
  Triggers a recompute for the tenant. Supports `idempotency_key`.
- `GET /healthz`  
  Liveness/readiness.

## 9. Projection Algorithm (v1)
### 9.1 Definitions
- **As-of balance:** Latest ledger balance at `from_date - 1 day`.
- **Forecast horizon:** `[from_date, to_date]`, default 90 days.
- **Drivers:** Known deltas affecting cash (payment intents and invoice approvals).

### 9.2 Rules
1. Query ledger-service or local ledger snapshot to compute starting balance as of `from_date`.
2. Build a timeline of projected deltas by date:
   - PaymentIntentCreated/Initiated: schedule a projected outflow on `scheduled_settlement_date`.
   - PaymentIntentSettled: if ledger posting exists for the settlement, do not double-count; use ledger as source of truth.
   - PaymentIntentFailed: remove any projected outflow tied to that intent.
   - InvoiceApproved (optional): create an assumed outflow on `due_date` with `assumption_type=INVOICE_APPROVAL`.
3. For each day in the horizon:
   - `opening_balance` = prior day's `closing_balance`.
   - `net_change` = sum of projected deltas on that date.
   - `closing_balance` = `opening_balance + net_change`.
4. Persist daily projections and link each delta to a driver record.
5. Evaluate thresholds; emit `LowBalanceProjected` if `closing_balance < threshold`.

### 9.3 Conflict Resolution
- If a ledger posting exists for a payment intent and date overlaps, ledger posting overrides the projection.
- If multiple signals exist for the same intent, prefer the latest state in the state machine:
  `Created` -> `Initiated` -> `Settled/Failed`.

### 9.4 Traceability
Each projected delta records:
- `source_type` (PaymentIntent, InvoiceApproval, LedgerPosting, Assumption)
- `source_id` (event payload id)
- `event_id` from the envelope

## 10. Data Model (Postgres)
```
projection_runs(
  id uuid pk,
  tenant_id uuid,
  from_date date,
  to_date date,
  status text,            -- running|completed|failed
  computed_at timestamptz
)

daily_projections(
  id uuid pk,
  tenant_id uuid,
  date date,
  opening_balance numeric(18,2),
  net_change numeric(18,2),
  closing_balance numeric(18,2),
  projection_run_id uuid
)

projection_drivers(
  id uuid pk,
  daily_projection_id uuid,
  source_type text,
  source_id uuid,
  event_id uuid,
  amount numeric(18,2),
  description text,
  assumption_type text null
)

thresholds(
  id uuid pk,
  tenant_id uuid,
  currency text,
  min_balance numeric(18,2),
  enabled boolean
)

consumer_dedupe(
  consumer_name text,
  event_id uuid,
  processed_at timestamptz,
  primary key (consumer_name, event_id)
)

outbox(
  id uuid pk,
  event_type text,
  payload jsonb,
  published_at timestamptz null
)
```

## 11. Idempotency and Consistency
- Consumer dedupe on `(consumer_name, event_id)` to avoid double-processing.
- Recompute endpoint requires `idempotency_key`.
- Projections and outbox records written in a single transaction.

## 12. Tech Choices (Ruby)
- Rack + Sinatra (or Rails API mode if preferred).
- Sidekiq for background projection jobs.
- Kafka/NATS consumer for event ingestion.
- `dry-schema` for payload validation.

## 13. Observability
- Structured logs with `tenant_id`, `correlation_id`, `event_id`.
- Metrics: projection duration, event lag, alert counts.
- Tracing across event handlers and recompute endpoints.

## 14. Open Questions
- Source of truth for ledger balance: query ledger-service vs local snapshot table?
- Required horizon for projections in v1 (30/60/90 days)?
- Whether to include historical averages as optional assumptions.
