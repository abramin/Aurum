# Shared Agent Non-Negotiables (Aurum)

These rules apply to all review agents in this repository.

## Code Style

- Idiomatic Go: prefer stdlib patterns, `errors.Is/As`, `%w` wrapping.
- No globals; dependency injection via constructors.
- Type aliases + `Parse*` at boundaries; domain primitives enforce validity at creation.
- Pointer returns from stores/services to avoid struct copying (value types only for immutable VOs).

## Layering

| Layer | Location | Responsibility |
|-------|----------|----------------|
| Domain | `internal/domain/*` | Pure logic, aggregates, invariants. No I/O, no `context.Context`. |
| Application | `internal/service/*` | Use-case orchestration, transaction boundaries, event publishing. |
| Infrastructure | `internal/store/*`, `internal/client/*` | Persistence, external APIs, adapters. |
| Transport | `internal/handler/*`, `internal/api/*` | HTTP/gRPC parsing, response formatting. Thin, no business logic. |

## Error Handling

- Stores return sentinel errors (`ErrNotFound`, etc.); services own domain errors.
- No error echoing of user input; safe client messages with stable codes.
- Use `Execute` callback pattern for atomic validate-then-mutate to avoid error boomerangs.

## Testing

- Behavior tests over implementation tests.
- Mocks only to induce failure modes, not to verify call sequences.
- Every test should answer: "What invariant breaks if this test is removed?"

## Event-Driven Patterns (Aurum-specific)

- **Outbox pattern**: State + outbox record in same DB transaction; publisher reads and delivers.
- **Consumer dedupe**: Track `(consumer_name, event_id)` to prevent double-processing.
- **Idempotency**: All state-changing endpoints accept `idempotency_key`.
- **Event envelope**: All events include `event_id`, `event_type`, `occurred_at`, `tenant_id`, `correlation_id`.

## Finance Domain Invariants

- **Ledger postings must balance**: Sum of debits equals sum of credits.
- **Tenant isolation**: Every query and write requires and validates `tenant_id`.
- **State machine enforcement**: Aggregates enforce valid transitions (no invalid state jumps).
- **Append-only journal**: Ledger entries are immutable; corrections are new entries.
