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
                |  REST, public URL
                v
           Render (tally-gateway, public)
             gateway + ledger, one process (services/renderall)
                |  in-process, loopback only
                |  HTTP nudge (public URL)
                v
           Render (tally-fraud) -> Neon (Postgres)
```

- **Neon** hosts Postgres (free tier is persistent and scales to zero).
- **Render** runs three processes (web, gateway+ledger combined, fraud) as
  free web services.
- The gateway and ledger run **in one process** here (`services/renderall`),
  not as two services talking gRPC over the network. Render's free plan
  doesn't resolve private short hostnames between web services, and gRPC over
  its public `onrender.com` edge doesn't work without a custom domain, so two
  separate free services genuinely cannot reach each other over gRPC. Running
  them combined sidesteps that; the real two-service gRPC architecture
  (`services/ledger`, `services/gateway`) is still what `docker-compose` and
  the k8s manifests run, this is purely a free-hosting workaround.

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
wakes the chain (web, then gateway+ledger, then fraud) and can take up to a
minute or so. The dashboard shows a transfer immediately once it's awake; its
fraud flag appears a beat later, once the fraud service has woken and scored.
For a portfolio demo this is fine. Refreshing the fraud page after a few seconds
shows the flag.

Render only auto-wakes a service on an inbound request to its *public* URL. If
a page still 502s a few seconds after you load it, the service it depends on
(e.g. `tally-fraud`) may just still be waking, refresh again in 10-20s.

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
   [`render.yaml`](../render.yaml) and creates the three services
   (`tally-web`, `tally-gateway`, `tally-fraud`) plus the `tally-secrets` env
   group. (If you previously created a standalone `tally-ledger` service
   while debugging, it's no longer needed, you can delete it.)
3. Set `DATABASE_URL` in the **tally-secrets** env group to the Neon string from
   step 1. The gateway (which now embeds the ledger) and fraud services share it.
4. Deploy. The `tally-web` service's public URL
   (`https://tally-web-*.onrender.com`) is the app; open it in a browser.

Notes:
- `tally-gateway` runs `services/renderall`, the ledger and gateway combined
  in one process (see the architecture note above). `API_URL` on `tally-web`
  should point at `tally-gateway`'s public URL; `FRAUD_SCORE_URL` on
  `tally-gateway` should point at `tally-fraud`'s public URL. Both just need
  `http://` or `https://` a scheme, set them by hand in the dashboard if the
  blueprint didn't wire them.
- Free services spin down after ~15 min idle, and Render only auto-wakes a
  service on an inbound request to its *public* URL. If a dependent service
  (e.g. `tally-fraud`) has been idle, the first call to it after a quiet spell
  may fail once while it wakes, retry after a few seconds.

### 3. Seed demo data (optional)

Point the seed script at the live gateway:

```bash
API="https://tally-gateway-abc.onrender.com" ./scripts/seed.sh
```

The date-spreading step at the end of the script only runs against a local
Docker Postgres, so against Neon it is skipped automatically (transfers just keep
their real timestamps). Each seeded transfer nudges the fraud scorer, so the
fraud page fills in within a few seconds of the run finishing.
