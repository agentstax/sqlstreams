<!-- Im copying https://github.com/ghostty-org/ghostty/blob/main/README.md layout. You can hate me, but it's so fucking clean. -->

<h1>
<p align="center">
  <br>SQLStreams
</h1>
  <p align="center">
    It's Kafka on Postgres.
    <br />
    Fast, reliable and easy to use.
    <br />
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
</p>

## About

<!-- This is the "marketing" bullshit, I don't like it but its gotta be done -->

I love using kafka and you should to. There is a reason it is so widely used in the industry.

But you know what I hate about kafka. Running kafka. Maintaining kafka.

Well no more! Get yourself the kafka functionality you love with the ease of just running your friendly neighborhood Postgres database.

<!-- This is the juicy bit, the meat and potatoes if you will -->

SQLStreams is a pure SQL library which uses Postgres as its data broker.

Built with speed in mind toping out at around ~100k req/s.

And bundling togeather ease of use features look automatic retry and dead letter queues.

## Usage

### Go Library

few commands for starting project and installing dependency

then show basically the producer/consumer-only playgrounds

link to other playground examples

### CLI

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
