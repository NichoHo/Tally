# Deploying Tally for free

This describes the **free hosted demo** configuration. It runs the same code as
local development, with one difference: the fraud service is scored
synchronously over HTTP instead of through Kafka, so nothing needs to stay
running 24/7. Locally, `make up` still runs the full event-driven pipeline
(Redpanda + a Kafka consumer); see the main [README](../README.md).

## The shape of it

```
Browser -> Vercel (Next.js dashboard) -> Render (gateway, public)
                                              |  gRPC (private)
                                              v
                                          Render (ledger, private) -> Neon (Postgres)
                                              |  HTTP nudge (private)
                                              v
                                          Render (fraud scorer) -> Neon (Postgres)
```

- **Neon** hosts Postgres (free tier is persistent and scales to zero).
- **Render** runs the three backend processes as free web services.
- **Vercel** hosts the Next.js dashboard (free).

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
wakes the chain (gateway, then ledger, then fraud) and can take up to a minute.
The dashboard shows a transfer immediately; its fraud flag appears a beat later,
once the fraud service has woken and scored. For a portfolio demo this is fine.
Refreshing the fraud page after a few seconds shows the flag.

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

### 2. Render (backend)

1. Push this repo to GitHub (already done: github.com/NichoHo/tally).
2. In Render: **New > Blueprint**, select the repo. Render reads
   [`render.yaml`](../render.yaml) and creates the three services and the
   `tally-secrets` env group.
3. Set `DATABASE_URL` in the **tally-secrets** env group to the Neon string from
   step 1. All three services share it.
4. Deploy. The gateway's public URL (`https://tally-gateway-*.onrender.com`) is
   the API.

Notes:
- `LEDGER_GRPC_ADDR` and `FRAUD_SCORE_URL` are wired automatically from the other
  services' private addresses. If Render rejects the `hostport` references for
  web services when you apply the blueprint, delete those two lines from
  `render.yaml`, redeploy, then copy each service's internal address (Settings >
  Networking > Internal Address) into the gateway's `LEDGER_GRPC_ADDR` and
  `FRAUD_SCORE_URL` env vars by hand. The gateway prepends `http://` to
  `FRAUD_SCORE_URL` if it has no scheme.
- The ledger is a gRPC-only service; it has no HTTP health check on purpose,
  Render just confirms the port is open.

### 3. Vercel (dashboard)

The dashboard is a standard Next.js app in `web/`, so Vercel needs no config file.

1. In Vercel: **New Project**, import the repo, set **Root Directory** to `web`.
2. Add an environment variable `API_URL` = the Render gateway URL from step 2
   (e.g. `https://tally-gateway-abc.onrender.com`). The app uses this both for
   server-side fetches and for the `/api/*` rewrite, so the browser only ever
   talks to your Vercel origin (no CORS).
3. Deploy. Open the Vercel URL.

### 4. Seed demo data (optional)

Point the seed script at the live gateway:

```bash
API="https://tally-gateway-abc.onrender.com" ./scripts/seed.sh
```

The date-spreading step at the end of the script only runs against a local
Docker Postgres, so against Neon it is skipped automatically (transfers just keep
their real timestamps). Each seeded transfer nudges the fraud scorer, so the
fraud page fills in within a few seconds of the run finishing.
