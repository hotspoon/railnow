# RailNow

Mobile-first commuter train schedule app built with Go, Chi, templ, HTMX, SQLite/Turso, Goose, and Tailwind CSS.

RailNow supports route-specific departures, searches from the current or a selected time, transparent schedule-snapshot freshness, and device-local saved routes with their next scheduled train.

## Run locally

```sh
task run
```

Open `http://localhost:8080`. The application creates `data/railnow.db`, applies migrations, and loads the sample KRL data on first start.

## Developer commands

```sh
task templ       # regenerate templ Go code after editing templates
task css         # rebuild Tailwind CSS
task sqlc        # generate query code from db/queries
task test         # Go and Vitest unit tests
task test:e2e     # Playwright mobile Chromium and WebKit
task verify       # generation, all tests, and Go build
docker build -t railnow . && docker run -p 8080:8080 railnow
```

Run `npm install` once, followed by `npx playwright install chromium webkit`, before the frontend and E2E commands. The pinned frontend tools make CSS generation and browser tests reproducible on Node 18+.

Saved routes remain private to the current browser through `localStorage`. The Saved page requests the current next scheduled train from RailNow when opened, focused, manually refreshed, or when its countdown ends.

## Importing KRL schedules

The app starts with a small demo schedule. To replace it with the public Jabodetabek schedule dataset from [dedewanta/scraping-krl-jabodetabek](https://github.com/dedewanta/scraping-krl-jabodetabek), run:

```sh
task import:krl
```

This downloads CSV files into the ignored `.cache/` directory and replaces the local SQLite data. The importer preserves each train's ordered stops based on its departure times. The source does not provide per-stop arrival times, so RailNow uses the departure time for both arrival and departure until a more detailed source is available.

To refresh Turso from KAI Commuter's public schedule API, ensure `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN` are set, then run:

```sh
task refresh:schedules
```

The command first reads KAI Commuter's public station catalog, then fetches every enabled station sequentially (500 ms apart), and writes a raw diagnostic snapshot to the ignored `.cache/kci/` directory. It also checkpoints each successful station in a date-scoped progress file, so rerunning after a timeout resumes unfinished stations instead of downloading the completed ones again. It validates the complete result before replacing `trains` and `schedules` in one Turso transaction; `favorites` and `searches` are preserved. A failed request, parse, validation, or write leaves the active timetable unchanged.

Some API routes terminate beyond the station-picker catalog. RailNow records those published terminal destinations with deterministic `KCI_DEST_*` codes so the journey remains searchable; they are marked as destination-only stations.

This is a **KAI Commuter API schedule snapshot**, not real-time train tracking, position, or delay information. The old published-CSV scraper remains available through `go run ./cmd/scraper` and `go run ./cmd/importer` for historical/manual imports.

Install the Task runner once if it is not available:

```sh
brew install go-task/tap/go-task
```

Schedules are demo data. Change `data/seed.sql` for another timetable before first run, or delete only `data/railnow.db` to re-seed during local development.

## Turso

The app uses Turso automatically when both environment variables below are set; otherwise it continues to use local SQLite.

```sh
TURSO_DATABASE_URL=libsql://your-database-organization.turso.io
TURSO_AUTH_TOKEN=your-token
```

Create a Turso database and token, then migrate the existing local database with:

```sh
turso db create railnow
export TURSO_DATABASE_URL="$(turso db show --url railnow)"
export TURSO_AUTH_TOKEN="$(turso db tokens create railnow)"
task migrate:turso
```

`task migrate:turso` applies this repository's migrations to Turso first, then copies all RailNow tables. It refuses to overwrite a non-empty Turso database unless run with `go run ./cmd/migrate-turso --replace`. Deployments only need the two `TURSO_*` variables; do not set `DATABASE_URL` to the Turso URL.

## Deploying to Vercel

This repository is configured as a Go Vercel Function. The function connects directly to Turso and does not run migrations or import local SQLite data during a request.

1. Push this repository to GitHub, then import it in Vercel (leave the project root as this repository).
2. In **Settings → Environment Variables**, add `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN` for Production. Add them to Preview too only if preview deployments should use the same database.
3. Deploy. Vercel detects `api/index.go` as the Go Function and serves files in `public/` as static assets.

For a CLI deployment, run `vercel`, follow the project-linking prompt, then run `vercel --prod`. Use the same two Turso environment variables in the Vercel dashboard; never commit `.env`.

Vercel Functions should run in a region near the Turso database. Select that region in the Vercel project settings before production deployment.
