# Deploying Tally for free

This describes the **free hosted demo** configuration. It runs the same code as
local development, with one difference: the fraud service is scored
synchronously over HTTP instead of through Kafka, so nothing needs to stay
running 24/7. Locally, `make up` still runs the full event-driven pipeline
(Redpanda + a Kafka consumer); see the main [README](../README.md).

Everything, dashboard included, runs on Render. No second platform to manage.

## The shape of it

```
Browser -> Render (tally-web, Next.js dashboard, public)
                |  gRPC-free REST, private network
                v
           Render (tally-gateway, public)
                |  gRPC (private)
                v
           Render (tally-ledger, private) -> Neon (Postgres)
                |  HTTP nudge (private)
                v
           Render (tally-fraud) -> Neon (Postgres)
```

- **Neon** hosts Postgres (free tier is persistent and scales to zero).
- **Render** runs all four processes (web, gateway, ledger, fraud) as free web
  services, wired together over Render's private network.

### Why the fraud service changes

The event-driven pipeline needs Redpanda and a consumer both alive all the time.
Render's free tier has no always-on option, so the free build drops Kafka: after
a transfer commits, the gateway sends a best-effort `POST /score-pending` to the
fraud service (`FRAUD_SCORE_URL`), which scores every not-yet-scored transfer.
This is turned on purely by setting `FRAUD_SCORE_URL`; with it unset (local
docker-compose), the ledger's Kafka publish drives scoring exactly as before. The
scores written are identical either way.

`/score-pending` is idempotent (the `fraud_scores` unique index) and scores *all*
pending transfers, so a missed nudge (for example while the service was cold) is
picked up by the next transfer.

## Cost and the one real caveat

Everything here is free. The tradeoff is **cold starts**: free Render services
spin down after about 15 minutes idle, so the first request after a quiet spell
wakes the chain (web, then gateway, then ledger, then fraud) and can take up to
a minute or so. The dashboard shows a transfer immediately once it's awake; its
fraud flag appears a beat later, once the fraud service has woken and scored.
For a portfolio demo this is fine. Refreshing the fraud page after a few seconds
shows the flag.

## Steps

### 1. Neon (Postgres)

1. Create a free project at neon.tech and a database named `tally`.
2. Copy the **pooled** connection string. Append `?sslmode=require` if it is not
   already there.
3. Run the migrations against it once, from a checkout of this repo:

   ```bash
   docker run --rm -v "$PWD/services/ledger/migrations:/m" migrate/migrate:v4.17.1 \
     -path=/m \
     -database "postgres://USER:PASS@ep-xxx.neon.tech/tally?sslmode=require" \
     up
   ```

### 2. Render (everything)

1. Push this repo to GitHub.
2. In Render: **New > Blueprint**, select the repo. Render reads
   [`render.yaml`](../render.yaml) and creates all four services
   (`tally-web`, `tally-gateway`, `tally-ledger`, `tally-fraud`) plus the
   `tally-secrets` env group.
3. Set `DATABASE_URL` in the **tally-secrets** env group to the Neon string from
   step 1. The ledger and fraud services share it.
4. Deploy. The `tally-web` service's public URL
   (`https://tally-web-*.onrender.com`) is the app; open it in a browser.

Notes:
- Render's free plan does not resolve private short hostnames
  (`tally-ledger:PORT`) for `web`-type services, so `LEDGER_GRPC_ADDR` in
  `render.yaml` points at the ledger's public `onrender.com` hostname on
  `:443` instead, and the gateway dials it with TLS (Render terminates TLS at
  its edge and forwards plain HTTP/2 to the container, so gRPC still works).
  `API_URL` on `tally-web` and `FRAUD_SCORE_URL` on the gateway use the same
  public-hostname approach; set them by hand in the dashboard to each
  service's public URL if they are not already wired.
- The ledger is a gRPC-only service; it has no HTTP health check on purpose,
  Render just confirms the port is open (it may log a "no open HTTP ports"
  port-scan timeout even while healthy; ignore it).
- Only `tally-web` and `tally-gateway` need to be reachable from outside Render;
  `tally-ledger` and `tally-fraud` are only ever called from other services,
  but Render's free tier has no private-only instance type, so they end up with
  public URLs too. That's fine, nothing sensitive is exposed.
- Free services spin down after ~15 min idle, and Render only auto-wakes a
  service on an inbound request to its *public* URL, a private/internal call
  from another service cannot wake a sleeping one. If `tally-ledger` has been
  idle, visit `https://tally-ledger.onrender.com` directly once to wake it
  before retrying the dashboard.

### 3. Seed demo data (optional)

Point the seed script at the live gateway:

```bash
API="https://tally-gateway-abc.onrender.com" ./scripts/seed.sh
```

The date-spreading step at the end of the script only runs against a local
Docker Postgres, so against Neon it is skipped automatically (transfers just keep
their real timestamps). Each seeded transfer nudges the fraud scorer, so the
fraud page fills in within a few seconds of the run finishing.
