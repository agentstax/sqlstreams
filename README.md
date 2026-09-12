<!-- Im copying https://github.com/ghostty-org/ghostty/blob/main/README.md layout. You can hate me, but it's so fucking clean. -->

<p align="center">
  <br />
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".website/public/sqlstreams-dark.svg">
    <img alt="SQLStreams" src=".website/public/sqlstreams-light.svg" height="56">
  </picture>
</p>
<p align="center">
    <strong>It's Kafka on Postgres.</strong>
    <br />
    Fast, reliable and easy to use.
</p>
<p align="center">
    <a href="#about">About</a>
    ·
    <a href="#usage">Usage</a>
    ·
    <a href="#ARCHITECTURE">Architecture</a>
    ·
    <a href="https://sqlstreams.io">Documentation</a>
    ·
    <a href="CONTRIBUTING.md">Contributing</a>
    ·
    <a href="DEVELOPING.md">Developing</a>
</p>

<br />
<hr id="about" />
<br />

I use Kafka, you use Kafka, your mom uses Kafka. *Kafka is great.*

**Buuuuuut....** running and maintaing a Kafka cluster is not fun.

I'd love to use Kafka for my [billion dollar, AI powered TODO app](https://github.com/agentstax/tomorrows-todo-today) but my mental state cannot handle another `"no brokers available"` error.

<p>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".website/public/introducing-dark.svg">
    <img alt="INTRODUCING" src=".website/public/introducing-light.svg" width="140" height="28">
  </picture>
  <br />
  <b>SQLStreams</b> for when you barely know what a Topic is but know Kafka is good...
  <br />
  <em>for some reason or another.</em>
</p>

**SQLStreams is a pure SQL library that uses Postgres as its broker.**

- It's actually a log, not a queue 🤓, and it does [~68k messages/s](https://sqlstreams.io/benchmarks/) on my laptop 😎.
- You get consumer groups, replay, retention and compaction without running a traditional broker.
- Retries are automatic. Dead letters are `WHERE status = 'dead'`. There’s no admin UI. Just write some SQL.
- Every error has a code, and `sqlstreams explain <code>` will hand you the fix because I don't like thinking either.

## Usage

### Go Library

Add it to your module. You need a Postgres.

```sh
go get github.com/allegedlyreliable/sqlstreams@v0.1.1
```

Import the client from the same module:

```go
import "github.com/allegedlyreliable/sqlstreams/client"
```

A message is a struct with a schema version.

```go
type VideoUploaded struct {
	VideoId string `json:"video_id"`
}

func (VideoUploaded) SchemaVersion() int { return 1 } // increment on breaking changes
```

[Produce](examples/01-produce-only/)

```go
ctx, stop := sqlstreams.LifecycleContext(nil)
defer stop()

pool, _ := sqlstreams.NewPostgresPool(ctx, "user", "password", "localhost", "db", nil)
client, _ := sqlstreams.NewClient(ctx, pool, nil)

uploads := client.Stream[VideoUploaded]("videos.uploaded")
uploads.Register(ctx, nil)

producer, _ := uploads.Producer().Register(ctx, nil)
producer.Produce(ctx, &VideoUploaded{VideoId: "video-42"}, nil)
```

[Consume](examples/02-consume-only/)

```go
transcoder := uploads.Consumer("transcoder")
consumer, _ := transcoder.Register(ctx, nil)
consumer.Consume(ctx, func(ctx context.Context, video *VideoUploaded) error {
	fmt.Println("transcoding", video.VideoId)
	return nil
}, nil)
```

[Metrics](examples/11-metrics-read/)

```go
snapshot, _ := transcoder.Metrics().Snapshot(ctx)
fmt.Println("backlog", snapshot.Cursor.Backlog, "dead", snapshot.Exceptions.Dead)
```

[Consume built-in alerts](examples/12-alert-consumer/)

```go
alerts := client.Stream[sqlstreams.Alert](sqlstreams.AlertStreamName)
pager := alerts.Consumer("pager")
alertConsumer, _ := pager.Register(ctx, nil)
alertConsumer.Consume(ctx, func(ctx context.Context, alert *sqlstreams.Alert) error {
	fmt.Println(alert.Status, alert.Name, alert.Message, alert.Hint)
	return nil
}, nil)
```

Retries, dead letters, transactional produce, idempotent produce, keyed ordering, schedules, compaction and the rest are in [`examples/`](examples/).

### CLI

Install the stable CLI on macOS with Homebrew:

```sh
brew install --cask allegedlyreliable/tap/sqlstreams
sqlstreams --version
```

Platform archives are available from [GitHub Releases](https://github.com/allegedlyreliable/sqlstreams/releases/tag/v0.1.1).
Or install the CLI with Go:

```sh
go install github.com/allegedlyreliable/sqlstreams/cmd/sqlstreams@v0.1.1
```

The binary goes into `GOBIN`, or `$(go env GOPATH)/bin` when `GOBIN` is unset.
Add that directory to your `PATH` to run the commands below.

```sh
export SQLSTREAMS_ADMIN_DATABASE_URL=postgres://user:password@localhost/db

sqlstreams stream list                              # every registered stream
sqlstreams stream get videos.uploaded               # one specific stream's info
sqlstreams explain SQL0022                          # what an error code means, the fix, the SQL
sqlstreams metric list                             # current value of every built-in metric
sqlstreams alert list                              # what's active right now
sqlstreams manager run --metrics-address :9464     # run upkeep process, serve Prometheus /metrics
# ...many more
```

## Development

Read ideally in this order.

- [Architecture](ARCHITECTURE.md)
- [Developing](DEVELOPING.md)
- [Contributing](CONTRIBUTING.md)
- [Conventions](CONVENTIONS.md)

## License

SQLStreams is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE). Third-party components remain subject to their respective
licenses.
