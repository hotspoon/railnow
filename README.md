# RailNow

Mobile-first commuter train schedule MVP built with Go, Chi, templ, HTMX, SQLite, Goose, and Tailwind CSS.

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
task test
docker build -t railnow . && docker run -p 8080:8080 railnow
```

`task css` downloads the pinned Tailwind CLI on demand with `npx`; no large binary is stored in this repository.

Install the Task runner once if it is not available:

```sh
brew install go-task/tap/go-task
```

Schedules are demo data. Change `data/seed.sql` for another timetable before first run, or delete only `data/railnow.db` to re-seed during local development.
