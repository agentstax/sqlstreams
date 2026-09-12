# sqlstreams

Admin CLI for [SQLStreams](../../) — inspect, maintain, and destroy streams
against the control-plane Postgres.

## Install

```sh
go install github.com/agentstax/sqlstreams/cmd/sqlstreams@latest
```

## Connect

Every command needs a privileged Postgres URL, passed by flag or environment:

```sh
export SQLSTREAMS_ADMIN_DATABASE_URL="postgres://user:pass@host:5432/db"
# or per-command: --database-url "postgres://..."
```

This is deliberately **not** `DATABASE_URL` — that's your app's low-privilege
runtime role. The CLI runs DDL and `DROP`, so it wants admin credentials wired
in on purpose. Run `sqlstreams system register` to register the system with default config.
Running it again reapplies those defaults; declare custom system config from
application code with `System().Register`.

## Usage

### Creating streams

Streams are created from your code, by `client.Stream[T](name).Register`. There is no
`sqlstreams stream register`: the CLI reads config and never writes it, so a
stream created from a shell would just be overwritten by the next call your
code makes. `sqlstreams scheduler` works the same way — schedules
come from `client.Scheduler(name).Register`.

Names are dot-namespaced by domain and entity, `<domain>.<entity>[.<event>]`
(e.g. `orders.created`, `billing.invoice.paid`); streams are addressed by id
internally, so a name is safe to rename later.

### List streams

```console
$ sqlstreams stream list
NAME             ID
billing.paid     41
orders.created   42

2 streams
```

`list` is a scannable overview; `get` shows a stream's full config.

### Get one stream

```console
$ sqlstreams stream get orders.created
✓ stream "orders.created" (id=42)
  PartitionSize            1,000,000
  RetentionTTL             720h0m0s (30d)
  AllowDropPastCommitted   false
  IdempotencyKeyTTL        24h0m0s
  EmptyCompactionHeadTTL   1h0m0s
  DeliveryLogMode          failures
```

A missing stream exits non-zero, so `get -q` doubles as an existence check:

```sh
if sqlstreams stream get -q orders.created; then echo "exists"; fi
```

`stream get` reads the registration and config. Use a separate read for
payload-version retirement:

```sh
sqlstreams stream health orders.created
sqlstreams stream health orders.created --output json
```

Health returns an array of version verdicts in JSON. Stream get returns the
stream object directly, or `null` with exit 1 when absent. Durations keep
their units, such as `"1h0m0s"`.

### Read consumers

```sh
sqlstreams consumer list orders.created
sqlstreams consumer get orders.created billing
sqlstreams consumer worker list orders.created billing
sqlstreams consumer worker list orders.created billing exception_initial_backoff
```

Consumer get reads the registration; worker list displays the stored config
keys per worker from `Consumer(name).Workers`. Workers are declared at
`Register`. Session settings such as `ConsumeOptions.ClaimPollRate` are not
stored worker config.

Consumer list supports `--quiet` for names only. Consumer get and system get
support `--quiet` for a silent existence check. Consumer list JSON is the
consumer array; consumer get JSON is the row, or `null` with exit 1 when absent.

Read every consumer's binding declaration with `sqlstreams system binding list`,
which calls `System().Bindings`. Read one consumer's declaration with
`sqlstreams consumer binding get orders.created billing`.

### Read a message key

`stream key compaction-head` prints the key's compaction head, the message that
currently wins under it. The CLI has no message type in scope, so the
payload prints as the JSON the row stores (Postgres orders the keys):

```console
$ sqlstreams stream key compaction-head devices.config dev-7
✓ compaction head for "dev-7" on "devices.config"

  MessageId        2
  CreatedAt        2026-09-04 21:45
  RoutingKey
  CompactionRank   0
  Message
    {
      "restarts": 1,
      "device_id": "dev-7",
      "interval_seconds": 30
    }
```

A key nothing was produced under with compaction enabled has no head
and exits non-zero with SQL0066; the fix line names the `messages`
command below. `--output json` prints the message document with the
payload inline under `message`, not string-escaped.

`stream key messages` prints the key's retained history, newest first. A
RANK of 0 is a message that never opted into compaction:

```console
$ sqlstreams stream key messages devices.config dev-7 --limit 3
MESSAGE_ID   CREATED            RANK   MESSAGE
2            2026-09-04 21:45   0      {"restarts":1,"device_id":"dev-7","interval_seconds":30}
1            2026-09-04 21:45   0      {"restarts":0,"device_id":"dev-7","interval_seconds":30}

2 messages
```

- `--limit` — newest N messages, default 20

### Destroy a stream

Prompts for the stream name before deleting anything:

```console
$ sqlstreams stream destroy orders.created
This will PERMANENTLY delete stream "orders.created" (id=42) and every message it holds.
This cannot be undone.

Type the stream name to confirm: orders.created
destroying "orders.created"... done
✓ stream "orders.created" destroyed
```

- `--force` — required to delete a stream that still holds messages
- `--yes` — skip the prompt (for CI). Does **not** imply `--force`.

```sh
sqlstreams stream destroy orders.created --force --yes
```

## Scripting

- `-q` / `--quiet` — `list` prints names only; `get` prints nothing (the exit
  code is the answer).
- `--output json` — stream, scheduler, consumer, and system `get` return the
  resource object directly, or `null` with exit 1 when absent. Stream and
  scheduler durations are strings with units; list returns an array of the
  same resource objects. Operational errors are JSON on stderr.
- Exit codes: `0` success · `1` operation failed (not found, not empty, config
  mismatch, aborted) · `2` usage error.

## Command names

Commands use the client handle and method names in kebab-case. Resource
collections use `list`; reads with distinct meanings keep distinct verbs:

| Command | Client operation |
| --- | --- |
| `stream get <name>` | `Stream(name).Get` |
| `stream health <name>` | `Stream(name).Health` |
| `consumer list <stream>` | `Stream(name).Consumers` |
| `consumer get <stream> <consumer>` | `Consumer(name).Get` |
| `consumer worker list <stream> <consumer> [key]` | `Consumer(name).Workers`, displaying stored config keys |
| `system binding list` | `System().Bindings` |
| `consumer binding get <stream> <consumer>` | `Consumer(name).Binding().Get` |
| `system get --quiet` | `System().Get`, silent existence check |
| `scheduler get <name>` | `Scheduler(name).Get` |
| `scheduler status <name>` | `Scheduler(name).Status` |
| `scheduler messages <name> --limit 20` | `Scheduler(name).Messages` |
| `metric latest <name>` | latest measurement per matching attribute set |
| `metric history <name> --limit 10` | `Metric(name, attributes).History` per matching attribute set |
| `alert latest <name>` | `Alert(name).Latest` |
| `alert history <name> --limit 10` | `Alert(name).History` |
| `stream key compaction-head <stream> <key>` | `Key(key).CompactionHead` |
| `system register` | `System().Register` with default config |

`--limit` controls the number of history entries or messages. It never switches
the operation. Metric reads accept repeatable `--attribute key=value` filters
and `--series-limit` (default 10) to bound the number of attribute sets.
Alert reads accept `--stream` and `--consumer`; the latter requires `--stream`.
With `--output json`, alert latest returns the alert object or `null`, and
alert history returns an array or `[]`. Both exit 1 when no alert is retained.
Stream janitor and vacuum status return the worker snapshot in JSON, with
`unclaimed_for` as a duration string such as `"15s"` or `"0s"`.
Metric list accepts `--builtin` for names starting with `sqlstreams.` across
all scopes, or `--user` for user-produced measurements. The flags are mutually
exclusive; omit both to list both. Scheduler run supports `--concurrency parallel`
or `exclusive`, defaulting to `parallel`. Schedules compact their messages,
so `ordered` concurrency is rejected.
`--metrics-address` on `manager run` names the Prometheus endpoint address.
