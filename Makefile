SHELL := /bin/bash

# Load local configuration when present.
-include .env
export

PGPORT ?= 5432
DATABASE_URL ?= postgres://oddsvault:oddsvault@127.0.0.1:$(PGPORT)/oddsvault?sslmode=disable
LOCAL_DB := ./scripts/local-db.sh

# `next dev` loads the UI from :3000 and calls the API on :8080 directly, so the
# browser needs an explicit CORS grant. The deployed site is served behind the
# same-origin Cloudflare Pages proxy and needs no origin at all.
CORS_ALLOWED_ORIGINS ?= http://localhost:3000

.DEFAULT_GOAL := help
.PHONY: help db-check db-init db-start db-stop db-status db-reset db-psql migrate migrate-status sync settle odds import-bets import-odds api workers test vet fmt web-install web-dev web-build web-preview tunnel pages-login pages-create pages-deploy pages-origin journal-lock journal-token check

help:
	@echo "OddsVault"
	@echo ""
	@echo "  Database (local cluster in .local/)"
	@echo "  make db-check       verify PostgreSQL + TimescaleDB are installed"
	@echo "  make db-init        create the cluster and database (once)"
	@echo "  make db-start       start the cluster on :$(PGPORT)"
	@echo "  make db-stop        stop it (keeps data)"
	@echo "  make db-status      show state and connectivity"
	@echo "  make db-reset       delete the local cluster and its data"
	@echo "  make db-psql        open psql against it"
	@echo ""
	@echo "  Application"
	@echo "  make migrate        apply pending migrations"
	@echo "  make migrate-status list applied and pending migrations"
	@echo "  make sync           import football fixtures and results once"
	@echo "  make settle         settle selections against finished results"
	@echo "  make odds           capture live bookmaker prices once"
	@echo "  make import-bets FILE=path  import a bookmaker bet-history export"
	@echo "  make import-odds FILE=path  import a browser odds capture"
	@echo "  make api            run the Go API on :8080"
	@echo "  make workers        run all four background workers"
	@echo "  make test           run Go tests"
	@echo "  make check          vet, test, typecheck and build everything"
	@echo "  make web-install    install web dependencies"
	@echo "  make web-dev        run the Next.js app on :3000"
	@echo "  make web-build      production static export of the web app"
	@echo "  make web-preview    build and serve the Pages worker on :8788"
	@echo "  make tunnel         expose the API and update the Pages origin"
	@echo "  make pages-login    authenticate wrangler (one time)"
	@echo "  make pages-create   create the Cloudflare Pages project (one time)"
	@echo "  make pages-deploy   build and upload the static export"
	@echo "  make pages-origin URL=…  set the API_ORIGIN secret by hand"
	@echo "  make journal-lock   set or rotate the journal passphrase"
	@echo "  make journal-token  rotate the API/worker journal token"

db-check:
	PGPORT="$(PGPORT)" $(LOCAL_DB) check

db-init:
	PGPORT="$(PGPORT)" $(LOCAL_DB) init

db-start:
	PGPORT="$(PGPORT)" $(LOCAL_DB) start

db-stop:
	$(LOCAL_DB) stop

db-status:
	PGPORT="$(PGPORT)" $(LOCAL_DB) status

db-reset:
	@echo "This deletes the local cluster at .local/pgdata."
	$(LOCAL_DB) reset

db-psql:
	PGPORT="$(PGPORT)" $(LOCAL_DB) psql

migrate:
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate

migrate-status:
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate -status

sync:
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/sports-sync -once

settle:
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/settlement-worker -once

odds:
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/bookmaker-collector -once

import-bets:
	@test -n "$(FILE)" || (echo "usage: make import-bets FILE=.local/imports/<file>.json" && exit 2)
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/import-bets -file "$(abspath $(FILE))"

import-odds:
	@test -n "$(FILE)" || (echo "usage: make import-odds FILE=.local/imports/<file>.json" && exit 2)
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/import-odds -file "$(abspath $(FILE))"

api:
	cd backend && DATABASE_URL="$(DATABASE_URL)" CORS_ALLOWED_ORIGINS="$(CORS_ALLOWED_ORIGINS)" go run ./cmd/api

workers:
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/sports-sync & \
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/bookmaker-collector & \
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/settlement-worker & \
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/analytics-worker & \
	wait

test:
	cd backend && go test ./...

vet:
	cd backend && go vet ./...

fmt:
	cd backend && gofmt -l -w .

web-install:
	cd apps/web && npm install

web-dev:
	cd apps/web && NEXT_PUBLIC_API_BASE_URL="$${NEXT_PUBLIC_API_BASE_URL:-http://localhost:8080}" npm run dev

web-build:
	cd apps/web && npm run build

web-preview:
	cd apps/web && npm run build && npm run preview

tunnel:
	./scripts/tunnel.sh

pages-login:
	cd apps/web && npx wrangler login $(if $(CALLBACK_PORT),--callback-port $(CALLBACK_PORT),)

pages-create:
	cd apps/web && npx wrangler pages project create oddsvault-web --production-branch=main

pages-deploy:
	cd apps/web && npm run build && npx wrangler pages deploy out --project-name=oddsvault-web --branch=main --commit-dirty=true

pages-origin:
	@test -n "$(URL)" || (echo "usage: make pages-origin URL=https://<tunnel>.trycloudflare.com" && exit 2)
	cd apps/web && printf '%s' "$(URL)" | npx wrangler pages secret put API_ORIGIN --project-name=oddsvault-web
	@echo "Pages binds env vars per deployment: run 'make pages-deploy' to activate this origin."

journal-lock:
	./scripts/set-journal-passphrase.sh

journal-token:
	./scripts/set-journal-token.sh

check: vet test web-build
	cd backend && go build ./...
	cd apps/web && npm run typecheck
