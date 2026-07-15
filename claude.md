# CLAUDE.md

This file is the single source of truth for this project. Read it fully at the start of every session before writing or changing code. If anything you are about to do conflicts with the rules here, stop and follow this file.

---

## 1. What this project is and why it exists

This is a **payments ledger backend** with a small **fraud-scoring service** and a **web dashboard**. It simulates the engine that moves money between accounts in a banking or e-wallet app.

It is a **portfolio project** built to help a computer science graduate get hired as a backend or full-stack software engineer at fintech and marketplace companies (for example Money Forward, Mercari, Monzo, N26). Those companies care most about one thing: that money is handled **correctly**. So correctness is the top priority of this project, ahead of features, speed, or fancy tooling.

The project must end up being something the author can run locally with one command, click through in a browser, and explain out loud in an interview. Every decision should serve that goal.

### What "good" looks like here
- The money math is always correct and can never silently drift.
- The same payment sent twice never moves money twice.
- The code is clean, tested, and easy to read.
- The README lets a stranger run it in under five minutes.
- The dashboard looks trustworthy and professional, not like a generic template.

### Non-goals
- This is not a real bank. No real money, no real compliance, no real KYC.
- Do not add authentication providers, multi-region infra, or heavy microservice sprawl. Keep it small and correct.
- Do not chase buzzwords. Two or three well-justified services beat fifteen trivial ones.

---

## 2. Golden rules (never break these)

These are the rules that make or break the project. They apply to all code, always.

1. **Never store money as a floating point number.** Store all amounts as **integers in minor units** (for example cents), using `int64` in Go and `BIGINT` in Postgres. `10.10` dollars is stored as `1010`. Floats lose precision and are an instant red flag in fintech.
2. **Every movement of money is double-entry.** Money is never just added or subtracted on one account. It always leaves one account and arrives in another in the same operation, recorded as two matching ledger rows. See section 6.
3. **Every money operation happens inside one database transaction.** Either the whole transfer commits (transfer row + both ledger rows + balance updates + idempotency record) or none of it does. Never leave the database half-updated.
4. **Every transfer must be idempotent.** The client sends an idempotency key. The same key with the same request must never move money twice. See section 6.
5. **Lock accounts in a consistent order** (for example, by account id ascending) when updating balances, to prevent deadlocks and race conditions.
6. **The system-wide invariant must always hold:** the sum of every ledger entry across the whole system is zero. Money is conserved. There is a test for this and it must pass.
7. **Write a test for every money rule** before considering a feature done.
8. **Do not use em dashes in written docs, comments, or UI copy.** Use commas, colons, or parentheses instead. (Author preference.)

If you are ever unsure whether something is safe for money correctness, choose the safer option and leave a `// NOTE:` comment explaining the tradeoff.

---

## 3. Plain-English glossary

The author is newer to fintech vocabulary. Use these terms consistently in code and docs, and keep the definitions in mind.

- **Ledger**: the permanent record of every money movement. Like an accounting notebook that is never erased, only appended to.
- **Double-entry**: every transaction is written twice, once as money leaving an account and once as money arriving in another. The two sides always match, so the books always balance.
- **Debit / credit**: the two sides of a double-entry record. In this project we use a simple convention: a **debit** reduces an account's balance, a **credit** increases it. A transfer debits the source and credits the destination by the same amount.
- **Minor units**: the smallest unit of a currency (cents for USD, so 1 dollar = 100 minor units). We store all money as integer minor units.
- **Idempotency / idempotency key**: a unique id attached to a request so that if it is accidentally sent twice (retry, network hiccup), it only takes effect once. Prevents double charges.
- **Invariant**: a rule that must always be true. Example: an account's balance always equals the sum of its credits minus the sum of its debits.
- **Event bus**: a message pipeline (Kafka). When something happens, a service posts an announcement; other services read it and react, without the sender needing to know who is listening.
- **gRPC**: a fast, structured way for services to call each other over the network. We also expose plain REST for the browser and for easy testing.
- **REST**: the common web request style (`GET`, `POST` to URLs returning JSON).
- **Observability**: logs, metrics, and traces that let you see what the system is doing and prove it works.

---

## 4. System architecture

```
Browser (Next.js dashboard)
        |
        v  REST/JSON
   API Gateway (Go)  ----gRPC----> Ledger Service (Go) ----> Postgres
        |                                  |
        |                                  | publishes events
        |                                  v
        |                              Kafka (event bus)
        |                                  |
        |                                  v
        |                         Fraud Service (Python, ML)
        |                                  |
        |                                  v
        |                              Postgres (fraud_scores)
        v
   Observability (structured logs + metrics)
```

Everything runs in Docker containers, orchestrated locally with `docker-compose`, with a small Kubernetes manifest and Terraform snippet included to show deployment awareness.

### Flow of a transfer, end to end
1. The user submits a transfer in the dashboard.
2. The API gateway validates it and forwards it to the ledger service over gRPC.
3. The ledger service, inside one database transaction: checks the idempotency key, locks the two accounts, writes the transfer row, writes two ledger entries (debit source, credit destination), updates both balances, and stores the idempotency result.
4. It publishes a `transfers.completed` event to Kafka.
5. The fraud service consumes the event, scores the transfer, writes a `fraud_scores` row, and publishes `fraud.scored`.
6. The dashboard shows the transfer, the two ledger entries, and any fraud flag.

---

## 5. Tech stack

Keep the stack exactly as listed. Do not add libraries without a clear reason.

### Backend (core)
- **Language**: Go (latest stable, 1.22+).
- **Why Go**: it is the primary backend language at Mercari and Monzo, so it is the highest-value single choice for the author's target companies.
- **gRPC**: `google.golang.org/grpc` with Protocol Buffers for service-to-service calls.
- **REST**: a thin HTTP layer (standard library `net/http` with `chi` router) for the browser and for curl testing.
- **Database access**: `pgx` (`github.com/jackc/pgx/v5`). No heavy ORM. Write clear SQL.
- **Migrations**: `golang-migrate` with plain `.sql` files.

### Database
- **Postgres** (16+). Single instance is fine.

### Event bus
- **Kafka** (via the `redpanda` image for a lighter local footprint, Kafka-compatible). Go client: `github.com/twmb/franz-go`.

### Fraud service (ML)
- **Language**: Python 3.11+.
- **Libraries**: `scikit-learn` for the model, `confluent-kafka` or `aiokafka` for consuming events, `psycopg` for Postgres.
- **Model**: start rule-based, then add a simple `IsolationForest` (anomaly detection) trained on synthetic transaction data. Keep it honest and explainable. Do not overstate it.

### Frontend (dashboard)
- **Framework**: Next.js (App Router) with **TypeScript**.
- **Styling**: Tailwind CSS.
- **Data fetching**: server components plus `fetch` to the REST gateway. Keep it simple, no Redux.
- **Charts**: `recharts` for the one or two small charts on the dashboard.

### Infrastructure and tooling
- **Docker** and **docker-compose** for local run.
- **Kubernetes**: one set of manifests (`k8s/`) for the ledger service, to show awareness. Does not need to be production grade.
- **Terraform**: one small `infra/` example (for instance, a Postgres instance definition) to show infrastructure-as-code awareness.
- **Make**: a `Makefile` with the common commands (see section 16).
- **CI**: a GitHub Actions workflow that runs lint and tests on push.

### Language swap note
If the author later decides to target Germany (N26, Delivery Hero) as the primary market, the core services can be rewritten in **Kotlin with Spring Boot**, keeping this exact architecture. Do not do both. Go is the default.

---

## 6. The money-movement rules (most important section)

This is the heart of the project. Read carefully.

### 6.1 Amounts
- All amounts are `int64` minor units, always positive for a transfer.
- Every account and transfer has a `currency` (ISO code like `USD`). A transfer's source and destination must share the same currency. No cross-currency conversion in v1.

### 6.2 A transfer, step by step (must run in ONE db transaction)
Given a request: source account, destination account, amount, currency, idempotency key.

1. **Validate**: amount > 0, source != destination, both accounts exist, currencies match.
2. **Idempotency check**:
   - Look up the idempotency key.
   - If it exists and the stored request fingerprint matches this request, return the stored transfer result. Do not move money again.
   - If it exists but the request fingerprint differs, return a `409 Conflict`. (Same key, different request is a client bug.)
   - If it does not exist, continue.
3. **Lock accounts**: `SELECT ... FOR UPDATE` on both account rows, ordered by account id ascending, to avoid deadlocks.
4. **Check funds** (if the source is not allowed to go negative): ensure source balance >= amount. Otherwise reject with `422 Unprocessable Entity`.
5. **Write the transfer row** with status `completed`.
6. **Write two ledger entries**:
   - one `debit` on the source for `amount`
   - one `credit` on the destination for `amount`
7. **Update balances**: source balance minus amount, destination balance plus amount.
8. **Store the idempotency record**: key, request fingerprint, resulting transfer id.
9. **Commit** the transaction. If any step fails, roll back everything.
10. **After commit**, publish `transfers.completed` to Kafka. (Publishing happens after commit so we never announce a transfer that did not actually happen.)

### 6.3 Invariants (there must be tests for each)
- **Per transfer**: total debits equal total credits equal the transfer amount.
- **Per account**: balance equals sum of credits minus sum of debits for that account.
- **System-wide**: the sum of all ledger entries (credits positive, debits negative) is exactly zero.
- **Idempotency**: sending the same transfer with the same key N times results in exactly one transfer and one pair of ledger entries.
- **Concurrency**: many transfers running at once never produce a wrong balance or a lost update. (Test with concurrent goroutines.)

### 6.4 What NOT to do
- Do not update a balance without writing the matching ledger entries.
- Do not compute balances with floats or by reading and writing outside a transaction.
- Do not publish the Kafka event inside the database transaction.

---

## 7. Data model (Postgres)

Use these tables. Keep names and types consistent.

```sql
-- accounts
CREATE TABLE accounts (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT        NOT NULL,
    currency     CHAR(3)     NOT NULL,
    balance_minor BIGINT     NOT NULL DEFAULT 0,   -- cached balance, always kept in sync
    allow_negative BOOLEAN   NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- transfers (one row per money movement)
CREATE TABLE transfers (
    id                 BIGSERIAL PRIMARY KEY,
    source_account_id  BIGINT      NOT NULL REFERENCES accounts(id),
    dest_account_id    BIGINT      NOT NULL REFERENCES accounts(id),
    amount_minor       BIGINT      NOT NULL CHECK (amount_minor > 0),
    currency           CHAR(3)     NOT NULL,
    status             TEXT        NOT NULL,   -- 'completed' | 'failed'
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ledger_entries (two rows per transfer: one debit, one credit)
CREATE TABLE ledger_entries (
    id           BIGSERIAL PRIMARY KEY,
    transfer_id  BIGINT      NOT NULL REFERENCES transfers(id),
    account_id   BIGINT      NOT NULL REFERENCES accounts(id),
    direction    TEXT        NOT NULL CHECK (direction IN ('debit','credit')),
    amount_minor BIGINT      NOT NULL CHECK (amount_minor > 0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- idempotency_keys (dedupe transfers)
CREATE TABLE idempotency_keys (
    key           TEXT        PRIMARY KEY,
    request_hash  TEXT        NOT NULL,   -- fingerprint of the request body
    transfer_id   BIGINT      REFERENCES transfers(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- fraud_scores (written by the fraud service)
CREATE TABLE fraud_scores (
    id            BIGSERIAL PRIMARY KEY,
    transfer_id   BIGINT      NOT NULL REFERENCES transfers(id),
    score         NUMERIC(4,3) NOT NULL,   -- 0.000 to 1.000
    decision      TEXT        NOT NULL,    -- 'allow' | 'review' | 'block'
    model_version TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_account ON ledger_entries(account_id);
CREATE INDEX idx_ledger_transfer ON ledger_entries(transfer_id);
CREATE INDEX idx_transfers_created ON transfers(created_at DESC);
CREATE INDEX idx_fraud_transfer ON fraud_scores(transfer_id);
```

Note: `balance_minor` on accounts is a cached value kept in sync inside the transfer transaction. The ledger entries are the source of truth; there is a test that recomputes balances from entries and compares them to the cached value.

---

## 8. API surface

### 8.1 REST (for the browser and curl)
Base path `/v1`. All money fields are integer minor units.

- `POST /v1/accounts` create an account. Body: `{ "name": "...", "currency": "USD", "allow_negative": false }`.
- `GET /v1/accounts` list accounts.
- `GET /v1/accounts/{id}` account detail with current balance.
- `GET /v1/accounts/{id}/entries` ledger entries for an account.
- `POST /v1/transfers` create a transfer. **Requires header `Idempotency-Key`.** Body: `{ "source_account_id": 1, "dest_account_id": 2, "amount_minor": 1000, "currency": "USD" }`.
- `GET /v1/transfers` list transfers (paginated).
- `GET /v1/transfers/{id}` transfer detail, including its two ledger entries and fraud score if present.
- `GET /v1/fraud/flags` list transfers with decision `review` or `block`.
- `GET /healthz` liveness. `GET /readyz` readiness.

Return proper status codes: `201` on create, `200` on read, `409` on idempotency conflict, `422` on business-rule rejection (for example insufficient funds), `400` on bad input.

### 8.2 gRPC (service to service)
Define in `proto/ledger.proto`. Sketch:

```proto
service LedgerService {
  rpc CreateAccount(CreateAccountRequest) returns (Account);
  rpc GetAccount(GetAccountRequest) returns (Account);
  rpc CreateTransfer(CreateTransferRequest) returns (Transfer);  // includes idempotency_key field
  rpc GetTransfer(GetTransferRequest) returns (Transfer);
}
```

The API gateway calls the ledger service over gRPC. Keep the proto as the contract.

---

## 9. Event schema (Kafka)

Two topics. Events are JSON, keyed by transfer id.

- **`transfers.completed`** (published by ledger service after commit):
```json
{
  "transfer_id": 123,
  "source_account_id": 1,
  "dest_account_id": 2,
  "amount_minor": 1000,
  "currency": "USD",
  "occurred_at": "2026-01-01T12:00:00Z"
}
```
- **`fraud.scored`** (published by fraud service):
```json
{
  "transfer_id": 123,
  "score": 0.870,
  "decision": "review",
  "model_version": "iforest-v1"
}
```

Consumers must be idempotent too: processing the same event twice must not create duplicate `fraud_scores` rows (use `transfer_id` uniqueness or upsert).

---

## 10. Fraud service (Python, ML)

Keep it simple and honest. It is a differentiator because it connects the author's machine-learning background to a real fintech use case, but it must not be oversold.

### Responsibilities
1. Consume `transfers.completed` events.
2. Build features for each transfer: amount, source account's recent transfer count and volume (velocity), whether the destination is new for this source, hour of day.
3. Score the transfer:
   - **v1**: rule-based (for example, amount above a threshold, or unusually high velocity, raises the score).
   - **v2**: an `IsolationForest` trained offline on synthetic transaction data, saved to disk and loaded at startup.
4. Map score to a decision: `allow` (< 0.5), `review` (0.5 to 0.8), `block` (> 0.8). Thresholds configurable.
5. Write a `fraud_scores` row and publish `fraud.scored`.

### Honesty rules
- Document clearly in the README that the model is trained on synthetic data and is illustrative, not production fraud detection.
- Keep a `train.py` script that generates synthetic data and trains the model, so the pipeline is reproducible.

---

## 11. Frontend dashboard

A clean Next.js dashboard so the project is demoable in a browser. This is where recruiters get their first impression, so it must look considered and trustworthy.

### 11.1 Pages and routes
- **`/` Dashboard**: top row of stat cards (total accounts, total transfer volume today, transfer count today, flagged transfers). Below: a small line/bar chart of transfer volume over the last 7 days, and a table of the 10 most recent transfers with status and fraud decision.
- **`/accounts` Accounts**: table of all accounts (name, currency, balance, created). A "Create account" button opens a modal form. Each row links to the account detail.
- **`/accounts/[id]` Account detail**: the account's current balance (large, prominent), and a table of its ledger entries (date, transfer id, direction, amount, running balance).
- **`/transfers` Transfers**: a "New transfer" form (source account, destination account, amount, currency). On submit it generates an idempotency key client-side and shows the resulting transfer. Below the form, a paginated table of all transfers.
- **`/transfers/[id]` Transfer detail**: shows the transfer summary and, side by side, its **two ledger entries** (debit on the left, credit on the right) so the double-entry idea is visually obvious. Shows the fraud score and decision if present.
- **`/fraud` Fraud flags**: table of transfers with decision `review` or `block`, showing score, decision, and a link to the transfer.

### 11.2 Shared UI
- A left sidebar with links: Dashboard, Accounts, Transfers, Fraud.
- A top bar with the project name and an environment tag ("demo data").
- Reusable components: `StatCard`, `DataTable`, `StatusPill`, `Money`, `IdTag`, `Modal`.

---

## 12. Design language

The look should say "trustworthy financial tool", not "generic admin template". Follow these concretely.

### Principles
- Calm, restrained, lots of whitespace. Money apps earn trust by looking careful.
- Content-first: tables and numbers are the stars. Minimal decoration.
- One accent color used sparingly. Never rainbow.

### Color
- Background: near-white, warm neutral (for example `#FAFAF8`). Cards: pure white with a hairline border (`#E7E7E2`), no heavy shadows.
- Text: near-black (`#1A1A18`) for primary, muted gray (`#6B6B66`) for secondary.
- Single accent: a calm blue or teal (for example `#1D7A8C` or `#2563EB`). Use it for links, primary buttons, active nav.
- Status colors, used only for status: green `#1D9E75` (allow / completed), amber `#BA7517` (review / pending), red `#DC2626` (block / failed).
- Text on any colored pill uses a darker shade of that same color, never plain black.

### Typography
- UI font: `Inter` or the system sans stack. Two weights only: 400 regular, 500 medium. Never 700.
- **Money and numeric columns use tabular figures** (`font-variant-numeric: tabular-nums`) and are **right-aligned**, so digits line up.
- IDs (account id, transfer id, idempotency key) use a monospace font and a subtle gray pill.
- Sentence case for all labels and headings. Never Title Case, never ALL CAPS.

### Layout and components
- Base spacing unit 8px. Card padding 24px. Comfortable row height in tables (48px).
- Corner radius: 8px for buttons and inputs, 12px for cards. Full-radius pills for status only.
- Buttons: primary is filled accent, secondary is bordered neutral. Clear hover states.
- Tables: hairline row separators, muted uppercase-off column headers, hover row highlight. No zebra striping.
- Status pills: small, rounded, colored background tint with the darker text shade.
- **Always format money from minor units**: divide by 100, show two decimals and the currency (for example `$10.00 USD`). Never show raw minor units to the user.
- Empty states: every table has a friendly empty state ("No transfers yet. Create one to get started.").
- Fully responsive down to a phone width. The sidebar collapses to a top menu on small screens.
- Respect dark mode if quick, but light mode is the priority. Do not ship a broken dark mode.

### What to avoid
- No gradients, glows, heavy drop shadows, or neon.
- No em dashes in any UI copy.
- No fake data that looks unrealistic (use plausible names and amounts in seed data).

---

## 13. Repository structure

```
.
├── CLAUDE.md                # this file
├── README.md                # how to run, architecture, screenshots
├── Makefile
├── docker-compose.yml
├── .github/workflows/ci.yml
├── proto/
│   └── ledger.proto
├── services/
│   ├── gateway/             # Go: REST -> gRPC gateway
│   ├── ledger/              # Go: core ledger service
│   │   ├── cmd/
│   │   ├── internal/
│   │   │   ├── domain/      # transfer logic, invariants
│   │   │   ├── store/       # pgx queries, transactions
│   │   │   ├── grpc/        # server impl
│   │   │   └── events/      # kafka publisher
│   │   └── migrations/      # .sql migration files
│   └── fraud/               # Python: fraud scoring service
│       ├── consumer.py
│       ├── model.py
│       ├── train.py
│       └── requirements.txt
├── web/                     # Next.js dashboard (TypeScript, Tailwind)
├── k8s/                     # Kubernetes manifests (ledger service)
├── infra/                   # Terraform example
└── scripts/
    └── seed.go              # seed demo accounts and transfers
```

---

## 14. Build phases

Build in this order. Do not start a phase until the previous one is fully working and tested. A finished earlier phase is worth more than several half-finished later ones.

### Phase 1: correct ledger core (the must-have)
- Postgres schema and migrations.
- Ledger service in Go: create account, get account, create transfer (with idempotency and double-entry, in one transaction), get transfer.
- REST gateway exposing the endpoints.
- Full test suite for all invariants in section 6.3, including a concurrency test.
- `docker-compose up` runs Postgres + ledger + gateway.
- **Done when**: you can create accounts and transfers via curl, all invariant tests pass, and sending a duplicate idempotency key does not double-move money.

### Phase 2: events and fraud (the differentiators)
- Add Kafka to docker-compose.
- Ledger publishes `transfers.completed` after commit.
- Python fraud service consumes events, scores, writes `fraud_scores`, publishes `fraud.scored`.
- `train.py` produces the IsolationForest model from synthetic data.
- **Done when**: creating a transfer results in a fraud score appearing, and consuming the same event twice does not duplicate scores.

### Phase 3: dashboard and polish (the "hire this person" layer)
- Next.js dashboard with all pages in section 11 and the design language in section 12.
- Seed script for realistic demo data.
- README with architecture diagram, run instructions, and screenshots.
- Kubernetes manifest and Terraform example.
- CI workflow green.
- **Done when**: a stranger can clone the repo, run one command, open the browser, and understand what it does in two minutes.

---

## 15. Testing strategy

- **Unit tests** for the domain logic (transfer rules, idempotency decisions).
- **Integration tests** against a real Postgres (use a test container or a disposable database) for the full transfer transaction.
- **Invariant tests** (section 6.3): these are the most important tests in the repo. Include:
  - balances recomputed from ledger entries match cached balances,
  - system-wide sum of entries is zero,
  - duplicate idempotency key moves money once,
  - concurrent transfers never corrupt balances.
- **Fraud service**: unit test the feature builder and the score-to-decision mapping; test that duplicate events do not duplicate rows.
- Aim for meaningful coverage on the money paths, not a coverage percentage for its own sake.
- `make test` must run everything and must pass before any phase is considered done.

---

## 16. Local development and commands

Provide a `Makefile` with at least:

```
make up            # docker-compose up, start everything
make down          # stop everything
make migrate       # run database migrations
make seed          # insert demo accounts and transfers
make test          # run all Go and Python tests
make lint          # run linters (golangci-lint, ruff, eslint)
make proto         # regenerate gRPC code from proto
make web           # run the Next.js dev server
```

The whole system must come up with `make up` and be usable at `http://localhost:3000` (dashboard) with the API at `http://localhost:8080`.

---

## 17. Coding conventions

### Go
- Standard Go style, `gofmt`, `golangci-lint` clean.
- Errors are wrapped with context (`fmt.Errorf("...: %w", err)`), never ignored.
- Keep the domain logic (transfer rules) separate from the database and transport code.
- No global state. Pass dependencies explicitly.
- Money is always `int64` minor units. Create a small `Money` type if it improves clarity, but never a float.

### Python (fraud service)
- Format with `black`, lint with `ruff`, type hints everywhere, checked with `mypy` if quick.
- Keep model code (`model.py`) separate from the Kafka consumer (`consumer.py`).

### TypeScript (web)
- Strict mode on. No `any` without a comment explaining why.
- Server components by default; client components only when interactivity requires it.
- A single `Money` component is the only place that formats minor units into display strings.

### Commits
- Conventional commits (`feat:`, `fix:`, `test:`, `docs:`, `chore:`). Small, focused commits.

---

## 18. How Claude Code should work in this repo

- **Read this file first, every session.** When in doubt, follow it over any assumption.
- **Correctness over speed.** For anything touching money, prefer the safer approach and add a test.
- **Do not add dependencies or services** not listed here without flagging it and explaining why.
- **When starting a new phase**, restate the phase's "done when" criteria and work toward them.
- **Before saying a task is done**, run `make test` and `make lint` and report the results honestly. Do not claim something works if the tests were not run.
- **If you find a money-correctness bug**, fix it and add a regression test, even if it was not the current task.
- **Keep the README in sync** with what actually exists. Do not document features that are not built.
- **Ask a clarifying question** only when a decision genuinely changes the design and is not answered here. Otherwise proceed and note the assumption in a comment.
- **Write plain, honest comments and docs.** No hype, no em dashes.

---

## 19. Definition of done for the whole project

The project is portfolio-ready when all of these are true:
- `make up` starts everything; the dashboard works end to end in a browser.
- All invariant and concurrency tests pass.
- Duplicate transfers are provably idempotent.
- The fraud service scores transfers and the dashboard shows flags.
- The README has an architecture diagram, one-command run instructions, and screenshots.
- CI is green.
- The author can explain, out loud and without notes: what double-entry is, why money is stored as integers, how idempotency prevents double charges, and what happens inside the transfer database transaction.

That last point is the real goal. The code exists to make that conversation credible.