# Hosting OddsVault on Cloudflare (free tier)

The site is public at a `*.pages.dev` URL for **$0**, with the Go API, PostgreSQL
and the workers still running on the machine that owns the database.

```text
Browser
  └─▶ https://oddsvault-web.pages.dev        Cloudflare Pages (static, free)
        ├─ / , /events , /live , …           static HTML/JS export
        └─ /svc/*  ──▶ public/_worker.js ──▶ https://<random>.trycloudflare.com
                                                 └─▶ Go API :8080 ──▶ PostgreSQL
```

## Why the web app is a static export

Cloudflare's free plan gives static assets **unlimited** requests but only
**10 ms of CPU** per Worker/Function invocation. Next.js server rendering runs
on every request and routinely needs more than that, so an SSR deployment would
fail intermittently. Instead:

- `next build` emits `apps/web/out` (`output: "export"`);
- every page is a client component that loads its data after hydration;
- loading and error states are explicit — a stale build-time snapshot would
  violate the project's freshness rule, so no page data is baked in;
- `/svc/*` is proxied by an advanced-mode worker, so the browser never makes a
  cross-origin call and the upstream host is a runtime secret.

Nothing in the Go backend runs on Cloudflare. That is deliberate: TimescaleDB,
pgx, advisory locks and the five-minute collectors do not fit a Workers runtime.

## One-time setup

`wrangler` is already a dev dependency of `apps/web`.

```bash
# 1. Authenticate the CLI (browser prompt; use CLOUDFLARE_API_TOKEN when headless)
make pages-login

# 2. Create the Pages project once
make pages-create

# 3. Build and upload the first copy
make pages-deploy
```

Start the API and the tunnel in separate terminals:

```bash
# terminal 1 — the API and its database
make db-start
make api

# terminal 2 — public HTTPS URL for the API
make tunnel
```

`make tunnel` downloads `cloudflared` into `.local/bin` on first use, prints the
`https://<random>.trycloudflare.com` hostname, and stores it as the Pages secret
`API_ORIGIN`. Open the Pages URL and the site is live.

Verify locally before deploying:

```bash
make web-preview        # builds, then serves out/ with the worker on :8788
curl -s localhost:8788/svc/healthz
```

## Deploys from GitHub

`.github/workflows/deploy-web.yml` is **manual only** — pushing to `main` does
not deploy, so development cannot consume Cloudflare quota. Run it from
**Actions → Deploy web → Run workflow**, or deploy from the machine that owns
the database with `make pages-deploy`.

Add two repository secrets:

| Secret | Where to get it |
| --- | --- |
| `CLOUDFLARE_API_TOKEN` | Dashboard → My Profile → API Tokens → create a token with **Cloudflare Pages: Edit** (account scope) |
| `CLOUDFLARE_ACCOUNT_ID` | Dashboard → Workers & Pages → Account ID, or `npx wrangler whoami` |

`API_ORIGIN` is **not** a GitHub secret. It lives on the Pages project and can
change without a rebuild.

## Everyday runbook

```bash
make db-start        # local PostgreSQL/TimescaleDB
make api             # terminal 1
make tunnel          # terminal 2 — always refresh the secret after a restart
```

A quick tunnel gets a new hostname every time it starts. `make tunnel` sets
`API_ORIGIN`, then rebuilds and redeploys the worker, because Pages binds
environment variables per deployment. If you set the origin by hand, redeploy
afterwards:

```bash
make pages-origin URL=https://your-tunnel.trycloudflare.com
make pages-deploy
```

Nothing needs rebuilding: the worker reads the secret per request.

### Keep the API and tunnel alive

Run both under tmux, or as systemd user units so they survive logout:

```ini
# ~/.config/systemd/user/oddsvault-api.service
[Unit]
Description=OddsVault API
After=network-online.target

[Service]
WorkingDirectory=%h/Apex/oddsVault
ExecStart=/usr/bin/make api
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

```ini
# ~/.config/systemd/user/oddsvault-tunnel.service
[Unit]
Description=OddsVault Cloudflare quick tunnel
After=oddsvault-api.service

[Service]
WorkingDirectory=%h/Apex/oddsVault
ExecStart=%h/Apex/oddsVault/scripts/tunnel.sh
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
```

```bash
systemctl --user daemon-reload
systemctl --user enable --now oddsvault-api oddsvault-tunnel
loginctl enable-linger "$USER"
```

## Private bet journal

`/my-bets` is not public. It is protected in two layers:

1. **Pages worker** (`_worker.js`) serves a passphrase login page for
   `/my-bets/*` and refuses `/svc/api/v1/my-bets` without a signed session
   cookie (7 days, HttpOnly, SameSite=Lax).
2. **Go API** requires `Authorization: Bearer <JOURNAL_TOKEN>` on
   `/api/v1/my-bets`. The tunnel URL is public, so this closes the direct-access
   hole; only the Pages proxy adds the token.

Secrets (Pages project, production environment):

| Secret | Purpose | Rotation |
| --- | --- | --- |
| `JOURNAL_PASSPHRASE` | the passphrase you type | `make journal-lock` |
| `JOURNAL_SESSION_SECRET` | HMAC key for session cookies | `make journal-lock` (regenerates) |
| `JOURNAL_TOKEN` | shared API token | `make journal-token` + restart `make api` |
| `API_ORIGIN` | upstream API base URL | `make tunnel` |

Rotating the passphrase invalidates every session. Rotating the token requires
restarting the API so `.env` is re-read.

To lock the current browser again, use **Lock journal** at the bottom of the
page. With a custom domain, Cloudflare Access is the better long-term upgrade —
it moves the check off the worker and can add email/SSO.



| Limit | Free plan | What this site uses |
| --- | --- | --- |
| Static asset requests | unlimited | all pages, JS, CSS, fonts |
| Function requests | 100,000/day | one per `/svc/*` API read (not per page) |
| Function CPU | 10 ms | a streaming proxy, well under 1 ms |
| Builds | 500/month | one per push to `main` |
| Bandwidth | unlimited | — |

## Known trade-offs

- **The API host must stay awake.** If the machine sleeps or the tunnel stops,
  the site keeps loading but shows “API unavailable” — never invented data.
- **The quick tunnel is best-effort.** Cloudflare offers no SLA for it, and the
  URL is public (the API is a read-only public feed anyway). A custom domain on
  Cloudflare gives you a named tunnel and a stable hostname later; only
  `API_ORIGIN` would change.
- **Postgres stays local.** Back up `.local/pgdata`; Cloudflare holds no data.
- **No server-side rendering.** Pages ship a shell and hydrate data. For this
  product that is the honest option: a prerendered price would be a lie.

## Troubleshooting

| Symptom | Cause | Fix |
| --- | --- | --- |
| `API_ORIGIN is not configured` | secret missing | `make tunnel` |
| `502 upstream unreachable` | tunnel or API stopped | `make api`, then `make tunnel` |
| Tunnel URL changed | quick tunnel restarted | `make tunnel` again |
| CORS error on `localhost:3000` | `next dev` calls :8080 directly | `make api` (sets `CORS_ALLOWED_ORIGINS`) |
| `wrangler` auth error | expired login | `make pages-login`, or set `CLOUDFLARE_API_TOKEN` |
| `404` on a stale bookmark | route was renamed | fixture pages live at `/events/detail?id=…`; `/events/` always works |
