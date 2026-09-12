# Developing

Make sure to look through [ARCHITECTURE.md](ARCHITECTURE.md), [CONVENTIONS.md](CONVENTIONS.md), [CONTRIBUTING.md](CONTRIBUTING.md).

## Setup

Install Go 1.27+, [just](https://github.com/casey/just), and Docker with Compose.
Run commands from the repo root unless noted.

```sh
cp .env.example .env
go work init
go work use . ./cmd/sqlstreams ./otel ./examples ./.bench ./.tools ./.tests
```

Skip `go work init` if you already have a workspace. `.env` and `go.work`
are gitignored. Keep the Postgres defaults: examples and e2e tests use them.

## Run

Load `.env` into your shell for Compose and direct Go commands; `just` loads it itself.

```sh
set -a
source ./.env
set +a
docker compose -f .tools/database/docker-compose.yaml up -d --wait
go run ./examples/01-produce-only
go run ./examples/02-consume-only
```

The consumer runs until Ctrl-C. More programs: [`examples/`](examples/README.md).
pgAdmin: <http://localhost:5050>, using the credentials in `.env`.

Run the CLI against your checkout:

```sh
go run ./cmd/sqlstreams stream list
```

Stop the database with `docker compose -f .tools/database/docker-compose.yaml down`.
Add `-v` to delete its data.

## Check

Inside the module you changed:

```sh
go fmt ./...
go build ./...
go vet ./...
go test -race ./client
```

Replace `./client` with the packages you touched. Database tests
run only with `SQLSTREAMS_TEST_DATABASE_URL` set (the `.env.example` value
points at the dev database); unset, they skip. Nested modules have
their own `go.mod`; root `./...` does not include them.

Before opening a pull request:

```sh
just verify
```

This checks every development workspace module. It builds e2e programs but
does not run them. Use `just --list` to find the affected `*-e2e` tests;
run them against the development database, for example `just reclaim-e2e`.
