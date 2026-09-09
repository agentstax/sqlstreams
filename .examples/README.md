# Playground

Thirteen small programs, one concept each, against a local Postgres. Every
scenario is a `main.go` you run from the repo root. Read the header comment
first: it names the concept, the story, and any scenario to run before it.

## Install

- Go 1.27 or newer.
- Docker with the Compose plugin (`docker compose version`).

## Set up the database

From the repo root:

```sh
docker compose -f .examples/docker-compose.yaml up -d --wait
```

Stop it, keeping the data:

```sh
docker compose -f .examples/docker-compose.yaml down
```

Start over with an empty database:

```sh
docker compose -f .examples/docker-compose.yaml down -v
```

## Run a scenario

```sh
go run ./.examples/01-produce-only
```

Scenarios that run a consumer, a scheduler, or a manager keep running until
you press Ctrl-C; the rest print and exit. A "Run first" column means the
scenario reads a stream or group another scenario creates.

| # | Scenario | Command | Run first | Runs until |
| --- | --- | --- | --- | --- |
| 01 | produce-only service | `go run ./.examples/01-produce-only` | | exits |
| 02 | consume-only service | `go run ./.examples/02-consume-only` | 01 | Ctrl-C |
| 03 | consume with retry and dead-lettering | `go run ./.examples/03-consume-retry-dead` | 01 | Ctrl-C |
| 04 | produce inside the caller's own transaction | `go run ./.examples/04-produce-in-tx` | | exits |
| 05 | idempotent produce with a caller-supplied key | `go run ./.examples/05-idempotent-produce` | | exits |
| 06 | tuning a stream for throughput | `go run ./.examples/06-throughput` | | Ctrl-C |
| 07 | a consumer that starts at the head of the stream | `go run ./.examples/07-consume-from-head` | 01 | Ctrl-C |
| 08 | keyed ordering | `go run ./.examples/08-keyed-ordering` | | Ctrl-C |
| 09 | a handler that runs longer than its lease | `go run ./.examples/09-slow-handler` | 01 | Ctrl-C |
| 10 | a schedule that produces on a cron expression | `go run ./.examples/10-schedule-produce` | | Ctrl-C |
| 11 | reading what the system measures about itself | `go run ./.examples/11-metrics-read` | 01, then 02 | exits |
| 12 | consuming `__system.alerts` as a pager feed | `go run ./.examples/12-alert-consumer` | | Ctrl-C |
| 13 | a compacted stream used as a key/value store | `go run ./.examples/13-compacted-kv` | | exits |
