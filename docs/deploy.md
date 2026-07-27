# Deploying Tally for free

This describes the **free hosted demo** configuration. It runs the same code as
local development, with one difference: the fraud service is scored
synchronously over HTTP instead of through Kafka, so nothing needs to stay
running 24/7. Locally, `make up` still runs the full event-driven pipeline
(Redpanda + a Kafka consumer); see the main [README](../README.md).

The dashboard runs on Vercel, the backend on Render. Both free.

## The shape of it

```
Browser -> Vercel (Next.js dashboard, always warm)
                |  REST, public URL
                v
           Render (tally-gateway, public)
             gateway + ledger, one process (services/ledger/cmd/renderall)
                |  in-process, loopback only
                |  HTTP nudge (public URL)
                v
           Render (tally-fraud) -> Neon (Postgres)
```

- **Neon** hosts Postgres (free tier is persistent and scales to zero).
- **Vercel** hosts the Next.js dashboard. Its free tier does not spin down, so
  the page always paints immediately.
- **Render** runs two processes (gateway+ledger combined, fraud) as free web
  services.

### Why the dashboard is not on Render

It was, and it worked, but it made the demo slow to open. Render's free tier
spins a service down after about 15 minutes idle and allows **750 instance-hours
a month across the whole account**, which funds exactly one always-awake service
(730h). With the dashboard and the gateway both on Render, two services sit in
the path of the first page load and only one of them can be kept warm, so a
visitor waits either way.

Vercel hosts the dashboard for free without spinning down and without touching
that budget, which frees the full 730h for the gateway. Both hops are then warm
and the demo opens instantly. Low traffic makes this matter more, not less:
visits are rare and spread out, so essentially every visit would be a cold one.
- The gateway and ledger run **in one process** here (`services/ledger/cmd/renderall`),
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

Everything here is free. The tradeoff is **cold starts** on the Render side.
Nothing needs waking by hand: any inbound HTTP request to a service's *public*
URL wakes it, and the dashboard's own fetch is such a request. Keeping the
gateway pinged (step 3 below) removes the wait a visitor would see.

What is left after that:

- **`tally-fraud`** still sleeps, and that is fine. After a transfer commits the
  gateway fires a background `POST /score-pending` with a 90s timeout
  (`services/internal/app/app.go`), so the fraud service wakes on its own. A
  transfer appears in the dashboard immediately; its fraud flag appears a beat
  later. Because `/score-pending` scores *all* unscored transfers, even a nudge
  that dies against a cold container is picked up by the next one.
- **Vercel caps a server render at 60 seconds** on the free plan, and a cold
  Render container can take nearly that long. If the gateway is ever cold, the
  first load can fail instead of just being slow. The dashboard catches this and
  shows "Could not reach the API" with a retry button (`web/app/error.tsx`)
  rather than a stack trace. One retry works, because the wake is already under
  way. The keepalive ping is what stops this happening at all.

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

1. Push this repo to GitHub.
2. In Render: **New > Blueprint**, select the repo. Render reads
   [`render.yaml`](../render.yaml) and creates the two services
   (`tally-gateway`, `tally-fraud`) plus the `tally-secrets` env group. (If you
   previously created a standalone `tally-ledger` service while debugging, it's
   no longer needed, you can delete it.)
3. Set `DATABASE_URL` in the **tally-secrets** env group to the Neon string from
   step 1. The gateway (which now embeds the ledger) and fraud services share it.
4. Set `FRAUD_SCORE_URL` on `tally-gateway` to `tally-fraud`'s public URL. It
   needs an `http://` or `https://` scheme. Set it by hand in the dashboard.
5. Deploy, then check `https://tally-gateway-*.onrender.com/healthz` responds.

`tally-gateway` runs `services/ledger/cmd/renderall`, the ledger and gateway
combined in one process (see the architecture note above).

If you are migrating from the older all-Render setup, delete the leftover
`tally-web` service in the Render dashboard. Removing it from `render.yaml`
stops Render managing it, but does not delete it, and an orphaned free service
keeps consuming your 750 instance-hours.

### 3. Vercel (dashboard)

1. In Vercel: **Add New > Project**, select the same repo.
2. Set **Root Directory** to `web`. Vercel detects Next.js on its own; leave the
   build and output settings alone.
3. Add one environment variable: `API_URL` = `tally-gateway`'s public URL, for
   example `https://tally-gateway-abc.onrender.com`. Apply it to all
   environments.
4. Deploy. The Vercel URL is the app.

The browser only ever calls same-origin `/api/*`, which
[`web/next.config.mjs`](../web/next.config.mjs) rewrites to `API_URL`, so there
is no CORS to configure anywhere. Server components read `API_URL` directly.
`output: "standalone"` in that config is for the local Docker build; Vercel
ignores it.

Vercel's free tier is Hobby, which is non-commercial only. A portfolio project
qualifies.

### 4. Keep the gateway awake

Without this, the first visit after a quiet spell waits up to a minute for
Render to wake the gateway. With it, the demo opens instantly.

Point any free cron pinger at the gateway's health endpoint every **5 minutes**:

```
https://tally-gateway-abc.onrender.com/healthz
```

[cron-job.org](https://cron-job.org) is the simple option. Avoid GitHub Actions
for this: scheduled workflows are disabled automatically after 60 days of repo
inactivity, which a finished portfolio repo hits, and its cron firing times drift
by tens of minutes under load.

**Why 5 minutes.** Render spins a free service down after 15 minutes idle, so
that is the ceiling. The interval does not change the cost: instance-hours are
billed on wall-clock uptime, not per request, so a service that never sleeps
costs the same 730h a month whether you ping it every 5 minutes or every 14.
Since frugality buys nothing here, pick for reliability instead. At a 10 minute
interval a single late or failed ping opens a 20 minute gap and the service
sleeps anyway; at 5 minutes one miss is a 10 minute gap, still inside the
window. A `/healthz` request is a few hundred bytes, so the extra pings cost
nothing against the 100GB free bandwidth.

That 730h leaves about 20 of your 750 monthly free instance hours for
`tally-fraud`. That is plenty, since it only wakes when a transfer is made. If
you exceed the cap Render suspends free services for the rest of the month, so
if you expect heavy demo traffic, ping on a daytime window only (say 12 hours a
day, ~360h) instead of 24/7.

### 5. Seed demo data (optional)

Point the seed script at the live gateway:

```bash
API="https://tally-gateway-abc.onrender.com" ./scripts/seed.sh
```

The date-spreading step at the end of the script only runs against a local
Docker Postgres, so against Neon it is skipped automatically (transfers just keep
their real timestamps). Each seeded transfer nudges the fraud scorer, so the
fraud page fills in within a few seconds of the run finishing.
