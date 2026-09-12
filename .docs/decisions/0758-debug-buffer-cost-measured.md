---
status: accepted
date: 2026-09-11
phase: "pre-v1"
---

# The debug buffer stays always-on at its measured cost

## Context

[0559](0559-per-operation-debug-buffer.md) made a published healthy-path
number the adoption gate for always-on capture; [0565] moved the buffer
into the pipeline's capture and drain stages. The number was still owed.
The benchmark-recording design settled that CPU paths are priced with
`go test -bench` and benchstat, not the Postgres harness.

## Decision

- `pkg/common/logging/pipeline_test.go` holds
  `BenchmarkPipelineLoggerOperation`: one operation opens its boundary and
  narrates four Debug lines through a bound, suppressing pipeline into a
  WARN sink, with no Error to drain. `buffer=on` against `buffer=off` is
  the cost of capture.
- Measured 2026-09-11 (Apple M4, darwin/arm64, ten repetitions,
  `benchstat -col /buffer`):

      buffer=off   326.3 ns/op ± 5%   1.031 KiB/op   14 allocs/op
      buffer=on    468.2 ns/op ± 2%   1.531 KiB/op   17 allocs/op
                   +43.5% (p=0.000)   +48.5%         +21.4%

  About 142 ns, 512 bytes, and three allocations per operation, or 35 ns
  per captured line. The allocations are the ring's lazy growth to four
  records.
- The gate passes: an operation is at least one Postgres round trip, so
  the buffer is under a thousandth of the healthy path. Capture stays on
  wherever an instance's pipeline declares Buffer.
- Reproduce with
  `go test ./pkg/common/logging/ -run '^$' -bench PipelineLoggerOperation -count 10 -benchmem`
  and benchstat; a rerun that moves the ratio past this record is the
  trigger to revisit the ring's shape (pre-sizing, a pooled ring).

## Consequences

The number lives here and in the benchmark, not on the doc site: it is a
library-internal cost, not a user-facing capability. No results family is
opened for CPU benchmarks; the code is the reproduction.
