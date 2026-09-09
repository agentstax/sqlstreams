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
    <a href="https://vulkan-5ss.pages.dev">Documentation</a>
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

I'd love to use Kafka for my [agentic powered TODO app](https://github.com/agentstax/tomorrows-todo-today) but my mental state cannot handle another `"no brokers available"` error.

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

- It's actually a log, not a queue 🤓, and it does [N msgs/s](.bench/) on my laptop 😎.
- You get consumer groups, replay, retention and compaction without running a single broker.
- Dead letters are `WHERE status = 'dead'`. There’s no admin UI. Just write some SQL.
- Every error has a code, and `vulkan explain <code>` will hand you the fix because I don't like thinking either.

## Usage

### Go Library

Add it to your module. You need a Postgres.

```sh
go get github.com/agentstax/vulkan
```

A message is a struct with a schema version.

```go
type VideoUploaded struct {
	VideoId string `json:"video_id"`
}

func (VideoUploaded) SchemaVersion() int { return 1 } // increment on breaking changes
```

[Produce](.examples/01-produce-only/)

```go
ctx, stop := vulkan.LifecycleContext(nil)
defer stop()

pool, _ := vulkan.NewPostgresPool(ctx, "user", "password", "localhost", "db", nil)
client, _ := vulkan.NewClient(ctx, pool, nil)

uploads := client.Topic[VideoUploaded]("videos.uploaded")
uploads.Register(ctx, nil)

producer, _ := uploads.Producer().Register(ctx, nil)
producer.Produce(ctx, &VideoUploaded{VideoId: "video-42"}, nil)
```

[Consume](.examples/02-consume-only/)

```go
transcoder := uploads.Consumer("transcoder")
consumer, _ := transcoder.Register(ctx, nil)
consumer.Consume(ctx, func(ctx context.Context, video *VideoUploaded) error {
	fmt.Println("transcoding", video.VideoId)
	return nil
}, nil)
```

[Metrics](.examples/11-metrics-read/)

```go
snapshot, _ := transcoder.Metrics().Snapshot(ctx)
fmt.Println("backlog", snapshot.Cursor.Backlog, "dead", snapshot.Exceptions.Dead)
```

[Consume built-in alerts](.examples/12-alert-consumer/)

```go
alerts := client.Topic[vulkan.Alert](vulkan.AlertTopicName)
pager := alerts.Consumer("pager")
alertConsumer, _ := pager.Register(ctx, nil)
alertConsumer.Consume(ctx, func(ctx context.Context, alert *vulkan.Alert) error {
	fmt.Println(alert.Status, alert.Name, alert.Message, alert.Hint)
	return nil
}, nil)
```

Retries, dead letters, transactional produce, idempotent produce, keyed ordering, schedules, compaction and the rest are in [`.examples/`](.examples/).

### CLI

macOS

```sh
brew install --cask agentstax/tap/vulkan
```

Windows

```sh
choco install vulkan
```

Linux, or anywhere with Go

```sh
go install github.com/agentstax/vulkan/cmd/vulkan@latest
```

```sh
export VULKAN_ADMIN_DATABASE_URL=postgres://user:password@localhost/db

vulkan topic list                              # every registered topic
vulkan topic get videos.uploaded               # one specific topic's info
vulkan explain VK0022                          # what an error code means, the fix, the SQL
vulkan metric list                             # current value of every built-in metric
vulkan alert list                              # what's active right now
vulkan manager run --metrics-address :9464     # run upkeep process, serve Prometheus /metrics
```

## Development

- [Architecture](ARCHITECTURE.md)
- [Developing](DEVELOPING.md)
- [Contributing](CONTRIBUTING.md)
- [Conventions](CONVENTIONS.md)

## License

Vulkan is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE). Third-party components remain subject to their respective
licenses.
