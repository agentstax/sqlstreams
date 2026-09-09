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

I'd love to use Kafka for my [agentic powered TODO app](https://github.com/agentstax/tomorrows-todo-today) but if I see one more `"no brokers available"` error I will crash out.

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

- It's a log, not a queue, and it does [N msgs/s](.bench/) on my laptop 😎.
- You get consumer groups, replay, ordering and compaction without running a single broker.
- Dead letters are `WHERE status = 'dead'`. There's no admin UI because it's Postgres.
- Every error has a code, and `vulkan explain VK0022` will hand you the fix because I don't like thinking either.

## Usage

### Go Library

few commands for starting project and installing dependency

then show basically the producer/consumer-only playgrounds

link to other playground examples

### CLI

How to install: homebrew, linux (curl | sh), choco

Few starting out commands 

Then get into fun ones like code explain and metrics

## Development

The `cmd/vulkan` CLI is a nested module (its own `go.mod`) so its dependencies
stay out of the core library's module graph. Building it against your local
library checkout needs a Go workspace, which is gitignored — create it once:

```sh
go work init . ./cmd/vulkan
```

This writes a `go.work` linking the root module and the CLI module, so
`go build` / `go test` / `go run` resolve the CLI's import of the library to
your working tree instead of a published version.

## License

Vulkan is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE). Third-party components remain subject to their respective
licenses.
