# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Aurum is a finance workspace backend in Go demonstrating DDD, event-driven architecture, and systems design fundamentals. It combines spend management, payables/invoicing, payments orchestration, bank-feed reconciliation, and a double-entry ledger.

## Development Commands

Expected Makefile targets (see `prd.md` section 15):
```bash
make up              # Start Docker Compose (Postgres, Kafka/Redpanda)
make down            # Stop Docker Compose
make migrate         # Run database migrations
make test            # Run all tests
make run-spending    # Run spending API service
make run-payables    # Run payables API service
make run-ledger      # Run ledger service
make run-workers     # Run worker processes
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
