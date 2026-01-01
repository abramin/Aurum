# Load Test Plan: Aurum Spending Context

## Prerequisites

- Docker Compose stack running (Postgres + Kafka)
- Load testing tool: `vegeta` or `k6`
- Optional: Prometheus + Grafana for visualization

## Scenario 1: Single Tenant Contention Test

**Intent:** Measure optimistic lock conflict rate under concurrent writes to same tenant.

### Setup

```yaml
Tenant: tenant-001
Card Account: pre-created with $100,000 limit
Duration: 5 minutes
```

### Load Pattern

```bash
# Using vegeta
echo 'POST http://localhost:8080/authorizations' | \
  vegeta attack -rate=50/s -duration=60s \
    -body=request.json \
    -header="Content-Type: application/json" \
    -header="X-Tenant-ID: tenant-001" \
    -header="X-Idempotency-Key: $(uuidgen)" | \
  vegeta report
```

### Request Body (request.json)

```json
{
  "tenant_id": "tenant-001",
  "card_account_id": "<pre-created-id>",
  "amount": {
    "amount": "10.00",
    "currency": "USD"
  },
  "merchant_ref": "load-test",
  "idempotency_key": "<unique-per-request>"
}
```

### Rates to Test

| Phase | Rate | Duration |
|-------|------|----------|
| 1 | 10 req/s | 60s |
| 2 | 50 req/s | 60s |
| 3 | 100 req/s | 60s |
| 4 | 200 req/s | 60s |

### Metrics to Capture

- `db_optimistic_lock_conflicts_total` by rate step
- p50/p95/p99 latency at each rate
- Error rate (409 Conflict expected for lock failures)

### Success Criteria

- < 5% optimistic lock conflicts at 100 req/s
- p99 < 500ms at 50 req/s
- 0 timeout errors at all rates

---

## Scenario 2: Multi-Tenant Baseline

**Intent:** Establish baseline throughput without contention.

### Setup

```yaml
Tenants: 100 pre-created card accounts (tenant-001 through tenant-100)
Duration: 5 minutes
```

### Load Pattern

```bash
# Using k6
k6 run --vus 100 --duration 5m multi-tenant-test.js
```

### k6 Script (multi-tenant-test.js)

```javascript
import http from 'k6/http';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

export const options = {
  stages: [
    { duration: '60s', target: 100 },  // 100 req/s
    { duration: '60s', target: 500 },  // 500 req/s
    { duration: '60s', target: 1000 }, // 1000 req/s
    { duration: '60s', target: 500 },  // ramp down
    { duration: '60s', target: 100 },
  ],
};

export default function () {
  const tenantNum = Math.floor(Math.random() * 100) + 1;
  const tenantId = `tenant-${String(tenantNum).padStart(3, '0')}`;

  const payload = JSON.stringify({
    tenant_id: tenantId,
    card_account_id: `card-${tenantId}`,
    amount: { amount: '10.00', currency: 'USD' },
    merchant_ref: 'load-test',
    idempotency_key: uuidv4(),
  });

  http.post('http://localhost:8080/authorizations', payload, {
    headers: {
      'Content-Type': 'application/json',
      'X-Tenant-ID': tenantId,
      'X-Idempotency-Key': uuidv4(),
    },
  });
}
```

### Metrics to Capture

- Overall throughput vs error rate
- Connection pool utilization (`db_pool_connections_in_use`)
- Transaction duration distribution

### Success Criteria

- Linear throughput scaling up to 500 req/s
- Connection pool < 80% utilized at 500 req/s
- < 1% error rate at 500 req/s

---

## Scenario 3: Idempotency Replay Storm

**Intent:** Validate idempotency cache effectiveness under replay conditions.

### Setup

```yaml
Tenant: tenant-001
Pre-populated: 10,000 existing authorizations with known idempotency keys
Duration: 3 minutes
```

### Load Pattern

90% of requests use existing idempotency keys (should return cached response), 10% are new requests.

### k6 Script (idempotency-test.js)

```javascript
import http from 'k6/http';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { SharedArray } from 'k6/data';

// Pre-populated idempotency keys from setup phase
const existingKeys = new SharedArray('keys', function () {
  return JSON.parse(open('./existing-keys.json'));
});

export const options = {
  vus: 50,
  duration: '3m',
};

export default function () {
  const isReplay = Math.random() < 0.9;
  const idempotencyKey = isReplay
    ? existingKeys[Math.floor(Math.random() * existingKeys.length)]
    : uuidv4();

  const payload = JSON.stringify({
    tenant_id: 'tenant-001',
    card_account_id: 'card-tenant-001',
    amount: { amount: '10.00', currency: 'USD' },
    merchant_ref: 'load-test',
    idempotency_key: idempotencyKey,
  });

  const res = http.post('http://localhost:8080/authorizations', payload, {
    headers: {
      'Content-Type': 'application/json',
      'X-Tenant-ID': 'tenant-001',
      'X-Idempotency-Key': idempotencyKey,
    },
  });
}
```

### Metrics to Capture

- `idempotency_cache_hits_total` rate
- Latency difference: cache hit vs cache miss
- Database query count

### Success Criteria

- Cache hit latency < 10ms p99
- Database query count proportional to 10% new requests only
- 0 duplicate authorizations created

---

## Setup Script

```bash
#!/bin/bash
# setup-load-test.sh

# Start infrastructure
docker compose up -d

# Wait for services
sleep 5

# Create test card accounts
for i in $(seq -w 1 100); do
  curl -X POST http://localhost:8080/card-accounts \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: tenant-$i" \
    -d '{
      "tenant_id": "tenant-'"$i"'",
      "spending_limit": {"amount": "100000.00", "currency": "USD"}
    }'
done

echo "Setup complete. Ready for load testing."
```

## Running Tests

```bash
# Scenario 1: Contention test
vegeta attack -rate=50/s -duration=60s ... | vegeta report

# Scenario 2: Multi-tenant
k6 run multi-tenant-test.js

# Scenario 3: Idempotency
k6 run idempotency-test.js
```
