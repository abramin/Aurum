# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Goals

Aurum is a learning project to practice **Staff-level engineering thinking**:
- **DDD modeling**: Bounded contexts, aggregates, invariants, anti-corruption layers
- **Systems design**: Consistency, idempotency, failure modes, eventual consistency
- **Finance domain**: Spend management, payables, payments, ledgers, reconciliation

The goal is depth over breadth—strong design narratives, not feature completeness.

## Development Commands

```bash
docker compose up -d     # Start Postgres + Kafka (KRaft mode)
docker compose down      # Stop infrastructure
go run ./cmd/aurum       # Run the service entrypoint
go test ./...            # Run all tests
```

## Architecture

### Bounded Contexts (Services)

1. **Spending Context** (`spending-api`) - Authorizations, captures, spending limits
2. **Payables Context** (`payables-api`) - Invoices, approvals, payment intents
3. **Payments Context** (`payments-orchestrator`) - Payment execution lifecycle, provider integration via ACL
4. **Ledger Context** (`ledger-service`) - Append-only journal entries, balance projections
5. **Bankfeed Context** (`bankfeed-importer`) - Bank transaction import, reconciliation

### Data & Messaging

- Postgres per context (or schemas in one Postgres for simplicity)
- Kafka/NATS for domain events
- Outbox pattern: state + outbox record in same DB transaction, publisher reads and publishes

### Key Patterns

- **Idempotency**: All state-changing endpoints accept `idempotency_key`, store and replay responses
- **Outbox**: Atomically write domain state + event, publisher handles delivery
- **Consumer Dedupe**: Track `(consumer_name, event_id)` to prevent double-processing
- **Double-Entry Ledger**: Every posting balances to zero, journal is append-only

### Event Envelope

All events include: `event_id`, `event_type`, `occurred_at`, `tenant_id`, `correlation_id`, `causation_id`, `payload`

## Domain Aggregates

- **Authorization** (Spending): States `Authorized → Captured/Reversed/Expired`
- **Invoice** (Payables): States `Draft → Submitted → Approved/Rejected → Paid`
- **PaymentIntent** (Payables): States `Created → Initiated → Settled/Failed → Reconciled`

## Key Invariants

- Cannot capture more than authorized amount; cannot capture twice
- Only `Draft` invoices can be submitted; only `Submitted` can be approved
- Ledger postings must balance (debits = credits)
- Every write requires `tenant_id` for isolation
