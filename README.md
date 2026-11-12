# OddsVault

Sports odds intelligence platform: immutable bookmaker odds snapshots, verified
sports results, versioned settlement, and analytics that can only ever be built
on real history.

**Status: Phase 7 (partial) — Live odds + personal journal.** Premier League
fixtures, results, live state, live LiveScoreBet prices, closing history,
versioned settlement and an imported 1xBet bet journal all work end to end.
Nothing is invented to fill the UI.

The full specification lives in [docs/sports_odds_intelligence_master_plan_v2.md](docs/sports_odds_intelligence_master_plan_v2.md).

---

## Stack

| Layer    | Choice                                          |
| -------- | ----------------------------------------------- |
| Frontend | Next.js App Router, React, TypeScript, Tailwind |
| Backend  | Go (single module, five binaries)               |
| Database | PostgreSQL + TimescaleDB (local cluster)        |
| Cache    | PostgreSQL tables only (no Redis yet)           |

Everything runs as native processes. There are no containers.

## Requirements

- Go 1.24 or newer
- Node 20 or newer
- PostgreSQL with the TimescaleDB extension (required, not optional)

On Arch Linux:

```bash
sudo pacman -S --needed postgresql timescaledb
```

`make db-check` verifies both are present and prints their versions.

## Repository layout

```
apps/web/                  Next.js application
backend/cmd/
  api/                     HTTP API
  migrate/                 forward-only migration runner
  sports-sync/             fixtures, live state, results
  bookmaker-collector/     odds snapshot capture
  settlement-worker/       deterministic settlement
  analytics-worker/        derived analytics
backend/internal/
  providers/               shared HTTP cache + OpenFootball / football-data adapters
  store/                   idempotent importer and identity matching
  syncjob/                 scheduled ingestion jobs
  normalize/               team name keys and explicit aliases
  api/                     HTTP handlers
backend/migrations/        embedded SQL migrations
scripts/local-db.sh        per-project PostgreSQL + TimescaleDB cluster
docs/                      specification and design notes
```

## Quick start

```bash
test -f .env || cp .env.example .env

make db-check       # PostgreSQL + TimescaleDB present?
make db-init        # create .local/pgdata and the oddsvault database (once)
make db-start       # start the cluster on :5432
make migrate        # apply migrations
make sync           # import the current football season (network)
make api            # Go API on :8080
```

Then in another shell:

```bash
make web-install    # once
make web-dev        # Next.js on :3000
```

`make workers` runs all four background binaries. They boot and idle until
Phase 1 registers real jobs — that is expected, not a failure.

### Verify

```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
curl -s localhost:8080/api/v1/competitions
curl -s "localhost:8080/api/v1/events?limit=5"
curl -s localhost:8080/api/v1/live
make migrate-status
```

Every endpoint returns `application/json`. `/readyz` returns `503` when the
database is unreachable; `/healthz` stays `200` so the process is not killed for
a dependency it can recover from.

## Hosting (free Cloudflare Pages)

The public site is a static export on Cloudflare Pages. A tiny advanced-mode
worker (`apps/web/public/_worker.js`) serves the export and proxies `/svc/*` to
the Go API, so the browser only ever sees one origin and the API host is a
runtime secret rather than a build-time constant.

The API, PostgreSQL and the workers stay on this machine and are exposed with a
free Cloudflare quick tunnel (`make tunnel`). That keeps the free hosting honest:
Cloudflare serves bytes, it does not pretend to run a Go/Postgres stack.

```bash
make api       # terminal 1
make tunnel    # terminal 2 — prints the API URL and sets API_ORIGIN
```

Full setup, GitHub Actions secrets, everyday runbook and trade-offs:
[docs/deploy-cloudflare.md](docs/deploy-cloudflare.md).

## Database

The cluster lives at `.local/pgdata` and is owned by this repository, so it can
be started and stopped per work session without a system-wide service:

```bash
make db-start      # start, ~40-80 MB idle
make db-stop       # stop, data kept
make db-status     # state and connectivity
make db-reset      # delete the cluster and its data
make db-psql       # psql into it
```

The port and credentials come from `.env` (`PGPORT`, `PGUSER`, `PGPASSWORD`,
`PGDATABASE`) and must match `DATABASE_URL`. The port is applied when the
cluster starts, so changing `PGPORT` needs `make db-reset && make db-init`.

TimescaleDB must be preloaded by PostgreSQL, which `local-db.sh` configures in
`postgresql.conf` at init time. `0001_enable_extensions.sql` fails with an
explicit message if the extension is unavailable instead of silently building
on plain PostgreSQL.

`odds_snapshots` becomes a hypertable in Phase 2. No retention policy may delete
odds history.

## Football data (Phase 1A)

| Concern | Source | Notes |
| --- | --- | --- |
| Schedule | OpenFootball `football.json` | CC0, full season incl. future fixtures, local kickoff time only |
| Results | football-data.co.uk CSV | Played matches, final + half-time score |
| Live state | ESPN public scoreboard | real event ids, status, scores, UTC kickoff |
| Live odds | LiveScoreBet gateway | 1X2, double chance, BTTS, total-goals lines, captured every 5 min |
| Closing odds | football-data.co.uk | Bet365 1X2 and O/U 2.5, one closing price per selection |
| Settlement | this repo | deterministic rules, versioned evaluations |

Both responses are cached in `provider_request_cache` with retry, backoff and
`Retry-After` handling. Completed seasons are cached forever; the current season
for six hours. `make sync` runs one pass; `JOB_RUN_ON_START=true make workers`
runs it inside the long-lived worker.

The importer is idempotent: re-running updates existing events instead of
duplicating them (`0 inserted` on the second pass). Events matched only by
competition, participants and date stay `provisional` until a provider with real
event ids confirms them.

Live polling covers three days back to seven days ahead, cached for 60 seconds,
and only runs when events exist in that window. ESPN ids confirm identity, so
events matched that way stop being provisional.

### Odds provenance

Two kinds of price are stored, never mixed silently:

- `price_kind = 'live'` — captured by the collector from LiveScoreBet's public
  gateway every five minutes. Append-only; each capture is a new snapshot.
- `price_kind = 'closing'` — the final price football-data.co.uk publishes after
  a match. One row per selection, replaced only by a re-import.

The UI shows the bookmaker, the kind and the capture age on every selection.
SportyBet's API returns HTTP 403 to server-side requests, so it stays on the
browser-capture path rather than a bot. 1xBet is captured the same way for now:
a JSON snapshot from the logged-in event page, imported with
`make import-odds FILE=...` and linked to the canonical fixture by team + date.

The LiveScoreBet collector runs every five minutes for the Premier League, the
UEFA Nations League and the Champions League. 1xBet's own event page only exposes the 1X2 tiles to the
DOM (the remaining markets stay behind a client-side loader), so its capture is
currently 1X2 only; expand the capture file's `quotes` as more markets become
readable. Never guess a line or selection that the page did not show.

### Snapshot history

`/snapshots` lists raw observations newest-first with filters for price kind
(live/closing), market and bookmaker, linking each row to its fixture. Backed by
`GET /api/v1/snapshots?kind=&market=&bookmaker=&limit=`.

### Cross-bookmaker view

`GET /api/v1/events/{id}/prices` returns the latest price per bookmaker for each
selection; the event page renders it as a comparison grid and highlights the best
captured price. There is no synthetic "average odds" row.

### Settlement

`make settle` evaluates every selection whose event has a verified final score.
Supported: match result, double chance, draw no bet, both teams to score, total
goals. Every change creates a new `evaluation_version`; earlier versions remain
auditable.

## What Phase 1A guarantees

- **No mock application data.** Every event, team and result comes from a
  provider response. Migrations seed only reference rows (sports, countries,
  competitions).
- **Migrations are forward-only and checksummed.** Editing an already-applied
  file fails the run; changes are new migrations. Concurrent runs serialise on a
  PostgreSQL advisory lock.
- **Missing data is `NULL`.** Not `0`, not a placeholder string.
- **Workers are honest.** A binary with no configured jobs says so and waits.
- **Secrets stay server-side.** `DATABASE_URL` is read by Go binaries only;
  the browser never receives provider credentials.

## Development

```bash
make test           # Go tests
make vet            # go vet
make check          # vet + tests + web build + typecheck
```

`make web-dev` serves the UI on `:3000` and points the browser at
`http://localhost:8080`; `make api` grants that origin through
`CORS_ALLOWED_ORIGINS`. The deployed build never calls the API cross-origin.

Tests must never write to a real database. Integration tests get a disposable
PostgreSQL/TimescaleDB instance or a temporary schema; the runner's integration
test uses `TEST_DATABASE_URL` and skips when it is unset.

## Analytics (Phase 6)

`GET /api/v1/analytics/odds-ranges?market=&line=&bucket=` groups settled
selections by their latest captured price. Only hits and misses enter the rate;
pushes, voids and missing data are reported separately. Every percentage in the
UI carries its sample size. `/analytics` renders the buckets with filters.

`GET /api/v1/events/{id}/movement` returns opening/current/high/low and the
observation series per selection; the event page draws it as a sparkline.

## Team crests

`media_assets` stores provider image URLs (currently ESPN team badges), and
`GET /api/v1/teams/{id}/crest` proxies the bytes through our API with a host
allowlist, size cap and local cache. The UI falls back to initials when a club
has no asset, so a missing badge never looks like a different team.

`make sync` refreshes crests for the enabled competitions; the worker repeats it
daily. Licence metadata is stored as `terms: unverified` — do not redistribute
the images without checking the provider's terms.

## Personal bets (Phase 7)

`make import-bets FILE=...` stores a JSON snapshot captured from the bookmaker's
own bet-history page. Re-importing is idempotent; a changed bet appends a version
rather than rewriting history. Stake, combined odds, potential return and outcome
are provider values. Leg descriptions are kept verbatim ("Corners A - Corners B")
and stay `unresolved` until a verified market mapping exists — the journal does
not guess what was bet.

`GET /api/v1/my-bets` returns the journal plus a staked/returned/profit summary.
Legs are parsed into home/away/market hints and linked to a canonical fixture when
the teams and date match an ingested competition. Competitions outside current
coverage stay `no_matching_fixture` rather than being guessed; ESPN-backed
coverage currently adds Copa del Rey for exactly this purpose.

On the hosted site the journal is private: the Pages worker requires a
passphrase session, and the API requires a bearer token so the public tunnel
cannot read it directly. See
[docs/deploy-cloudflare.md](docs/deploy-cloudflare.md#private-bet-journal).

## Next up

1. Browser-capture import for SportyBet (and any bookmaker that blocks servers).
2. Odds movement and range analytics once enough live snapshots exist.
3. FotMob team/player statistics for prop markets.
4. More competitions, each behind the market quality gate.
