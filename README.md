# Tally

A payments ledger backend with correct double-entry money movement, idempotent
transfers, a fraud-scoring service, and a web dashboard. This repository is a
portfolio project; correctness of the money math is the top priority.

Money is never a float. All amounts are integer minor units (cents), stored as
`BIGINT` in Postgres and `int64` in Go.

![Dashboard](docs/dashboard.png)

## Status

**All three build phases are complete**: the ledger core, events + fraud
scoring, and the dashboard.

What works today:

- Create accounts and transfers over a REST API.
- Every transfer is double-entry (a debit on the source, a credit on the
  destination) written in one database transaction.
- Transfers are idempotent: the same `Idempotency-Key` never moves money twice.
- Accounts are locked in a consistent order, so concurrent transfers cannot
  corrupt balances.
- After each transfer commits, the ledger publishes a `transfers.completed`
  event to Kafka (Redpanda).
- A Python fraud service consumes those events, scores each transfer, writes a
  `fraud_scores` row, and publishes `fraud.scored`. Consuming the same event
  twice cannot create a second score (unique index + upsert).
- Transfer detail includes the fraud score; `/v1/fraud/flags` lists transfers
  whose decision is `review` or `block`.
- A live reconcile check (`/v1/reconcile` and the Reconcile page) recomputes
  every balance from ledger entries, compares it to the cached balance, and
  confirms the system sums to zero.
- A Next.js dashboard at `http://localhost:3000`: stat cards, a 7 day volume
  chart, account and transfer browsing with running balances, a transfer form
  that generates idempotency keys client-side, and a fraud flags page. Transfer
  detail shows the two ledger entries side by side so the double-entry idea is
  visually obvious.
- Test suites cover the money invariants, idempotency, concurrency, the fraud
  feature builder, decision mapping, and duplicate-event dedupe.

**About the fraud model, honestly:** the IsolationForest is trained on
synthetic data with a fixed seed (`services/fraud/train.py`) and blended with
simple explainable rules. It is illustrative, not production fraud detection.

## Architecture

```
Browser
     |
     v  http://localhost:3000
  Dashboard (Next.js, Tailwind)
     |
     v  REST/JSON  (:8080)
  Gateway (Go, chi)
     |
     v  gRPC       (:9090)
  Ledger service (Go) ----> Postgres
     |                         ^
     | publishes after commit  | writes fraud_scores
     v                         |
  Redpanda (Kafka API) --> Fraud service (Python, scikit-learn)
```

The gateway is a thin translator: it validates request shape, forwards to the
ledger service over gRPC, and maps gRPC status codes to HTTP codes. All the
money rules live in the ledger service (`services/ledger/internal/domain` and
`.../store`). Events are published only after the database transaction commits,
so a transfer is never announced unless it actually happened.

## Design decisions

Short notes on why the project is built the way it is. These are the questions
an interviewer tends to ask, so the reasoning lives here next to the code.

**Money is stored as integer minor units, never a float.** `10.10` USD is stored
as the integer `1010`. Floating point cannot represent most decimal fractions
exactly, so repeated float arithmetic drifts (`0.1 + 0.2` is not `0.3`). In a
ledger that drift is money quietly appearing or vanishing. Integers are exact,
so the math is exact. Amounts are `int64` in Go and `BIGINT` in Postgres.

**Balances come from double-entry ledger entries, not a single balance column.**
Every transfer writes two immutable rows: a debit on the source and a credit on
the destination, for the same amount. An account's balance is the sum of its
credits minus its debits. A plain "current balance" column with no history can
be wrong with no way to prove it; with a ledger the balance is always provable
from the entries, and the entries are never edited or deleted. The cached
`balance_minor` column exists only as a performance shortcut, and the reconcile
check (below) proves it never disagrees with the entries.

**The whole transfer runs in one database transaction.** The transfer row, both
ledger entries, both balance updates, and the idempotency record either all
commit together or none of them do. There is no window where the books are half
updated.

**Accounts are locked in a consistent order (by id ascending).** Two transfers
touching the same pair of accounts in opposite directions could otherwise each
hold one lock and wait on the other forever (a deadlock). Always taking the
lower account id first makes that impossible.

**Transfers are idempotent by key.** The client sends an `Idempotency-Key`. The
same key with the same request returns the original transfer without moving
money again; the same key with a different request is a client bug and returns
`409`. This is what makes a retry after a network timeout safe.

**Events publish after the commit, and the next step is an outbox.** The ledger
publishes `transfers.completed` only after the transaction commits, so it never
announces a transfer that did not happen. Today that publish is fire and forget:
if the process crashed in the gap between commit and publish, the event would be
lost (consumers are idempotent, so a missed event is the only real risk, never a
double count). The fix is the transactional outbox pattern: write the event to
an `outbox` table inside the same transaction, then let a small worker publish
unsent rows and mark them sent. The event can then neither be lost nor sent for a
transfer that did not commit. See the `ponytail:` note in
`services/ledger/internal/events/publisher.go`.

**Corrections are appended, never edited.** A ledger is immutable history. To
reverse a wrong transfer you do not edit or delete its rows; you append a new,
opposite pair of entries that reference the original. History is preserved and
still reconciles. The reversal endpoint is a small planned addition; the schema
is already append-only, which is the part that matters.

**Reconciliation is exposed live, not just tested.** `GET /v1/reconcile` and the
Reconcile dashboard page recompute every account's balance from its ledger
entries, compare it to the cached balance, and confirm the system sums to zero.
Real ledger teams run exactly this kind of job and alert on any mismatch.

**What I would do differently at scale.** The cached `balance_minor` column is
the obvious bottleneck: every transfer updates two account rows, so a hot account
serialises all of its transfers. At real volume I would move to time-bucketed
balance snapshots (periodically materialise a balance as of a point in time, then
sum only the entries since), which is roughly the direction Monzo took.
Reconciliation would become a single SQL pass behind its own gRPC RPC rather than
the current per-account fetch, and the outbox above would replace fire-and-forget
publishing.

## Run it

Docker is the only prerequisite (no local Go, Node, protoc, or Postgres needed).

```bash
make up      # build and start everything
make seed    # recommended: insert demo accounts and transfers
```

Then open `http://localhost:3000` for the dashboard. The API is at
`http://localhost:8080`. Stop with `make down`.

### Deploying it for free

`make up` runs the full event-driven pipeline (Redpanda + a Kafka consumer). For
a completely free hosted demo (Neon + Render, dashboard included), the fraud
service is scored synchronously over HTTP instead, so nothing has to stay
running 24/7. The same code drives both: setting `FRAUD_SCORE_URL` on the
gateway switches it on, leaving it unset keeps the local Kafka path. See
[docs/deploy.md](docs/deploy.md).

## Screenshots

| Transfer detail (double entry) | Fraud flags |
| --- | --- |
| ![Transfer detail](docs/transfer-detail.png) | ![Fraud flags](docs/fraud.png) |

| Accounts | Transfers |
| --- | --- |
| ![Accounts](docs/accounts.png) | ![Transfers](docs/transfers.png) |

### Try it with curl

```bash
# create two accounts (treasury may go negative so it can fund others)
curl -s -X POST localhost:8080/v1/accounts \
  -H 'content-type: application/json' \
  -d '{"name":"treasury","currency":"USD","allow_negative":true}'
curl -s -X POST localhost:8080/v1/accounts \
  -H 'content-type: application/json' \
  -d '{"name":"alice","currency":"USD"}'

# move $15.00 (1500 minor units), account 1 -> account 2
curl -s -X POST localhost:8080/v1/transfers \
  -H 'Idempotency-Key: my-key-1' \
  -H 'content-type: application/json' \
  -d '{"source_account_id":1,"dest_account_id":2,"amount_minor":1500,"currency":"USD"}'

# send the SAME request again with the SAME key: money does not move twice,
# the same transfer is returned
curl -s -X POST localhost:8080/v1/transfers \
  -H 'Idempotency-Key: my-key-1' \
  -H 'content-type: application/json' \
  -d '{"source_account_id":1,"dest_account_id":2,"amount_minor":1500,"currency":"USD"}'

# see the transfer and its two ledger entries
curl -s localhost:8080/v1/transfers/1
```

## API (phase 1)

Base path `/v1`. All money fields are integer minor units.

| Method | Path | Notes |
| ------ | ---- | ----- |
| POST | `/v1/accounts` | create an account |
| GET | `/v1/accounts` | list accounts |
| GET | `/v1/accounts/{id}` | account detail with balance |
| GET | `/v1/accounts/{id}/entries` | ledger entries for an account |
| POST | `/v1/transfers` | create a transfer; requires header `Idempotency-Key` |
| GET | `/v1/transfers` | list transfers (`?limit=&before_id=`) |
| GET | `/v1/transfers/{id}` | transfer detail with its two ledger entries and fraud score if scored |
| GET | `/v1/fraud/flags` | transfers with fraud decision `review` or `block` |
| GET | `/v1/reconcile` | recompute balances from entries, check cache and system sum to zero |
| GET | `/healthz`, `/readyz` | liveness / readiness |

Status codes: `201` create, `200` read, `400` bad input, `404` not found,
`409` idempotency conflict (same key, different request), `422` business rule
rejection (insufficient funds, currency mismatch).

## Tests

```bash
make test
```

This spins up a throwaway Postgres and runs the Go and Python suites, including:

- per-transfer invariant: debits equal credits equal the transfer amount;
- per-account invariant: cached balance equals the balance recomputed from
  ledger entries;
- system-wide invariant: the signed sum of every ledger entry is exactly zero
  (money is conserved);
- idempotency: a duplicate key moves money once; a reused key with a different
  request conflicts;
- concurrency: 50 simultaneous transfers never lose an update, and 20
  goroutines racing with the same key produce exactly one transfer;
- fraud: feature builder validation, rule scoring, score-to-decision thresholds,
  and consuming the same event twice writes exactly one fraud_scores row.

The concurrency tests are also run under the Go race detector.

## Layout

```
proto/ledger.proto                     gRPC contract
services/ledger/
  cmd/ledger/                          service entrypoint
  internal/domain/                     money rules (pure Go, no DB)
  internal/store/                      pgx queries + the transfer transaction
  internal/grpcserver/                 gRPC server, error mapping
  internal/events/                     kafka publisher (after commit only)
  migrations/                          .sql schema migrations
services/gateway/                      REST -> gRPC
services/fraud/
  consumer.py                          kafka consumer, idempotent score writes
  model.py                             features, rules, IsolationForest scoring
  train.py                             trains the model on synthetic data
web/                                   Next.js dashboard (TypeScript, Tailwind)
k8s/                                   Kubernetes manifests (ledger service)
infra/                                 Terraform example (RDS Postgres)
scripts/seed.sh                        demo data
docker-compose.yml, Makefile, Dockerfile
.github/workflows/ci.yml               lint + tests for Go, Python, and web
```

## Regenerating gRPC code

The generated `proto/*.pb.go` files are committed. Regenerate them after editing
`proto/ledger.proto`:

```bash
make proto
```
