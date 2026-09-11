---
status: accepted
date: 2026-09-10
phase: "pre-v1"
---

# Concurrent lab handlers use independent record files

## Context

A bounded consumer profile attributed over 99.9% of sampled mutex delay
and 43% of consumer CPU samples to the shared handler record writer.
Handlers held that lock across JSON encoding and a flushed file write.
The lab must retain complete evidence before each handler returns [0745].

## Decision

- Use a bounded pool of independent handler record files per process.
  A callback borrows an available writer, synchronously encodes and flushes
  its record, then returns the writer. File writes can run concurrently.
- Size the pool from the largest declared instance count multiplied by
  each group's resolved message concurrency, summed across groups/streams.
  An idle-only process opens one file. There is no new scenario setting.
- Number files under the existing process name and handler suffix. Append
  on restart. Preserve consumer identity, timestamps, attempts, and outcomes
  inside each record. The existing checker loads every matching file.
- Keep record write failure fatal to the lab and return it from the handler.
  Stop consumer sessions before closing their writers. Producer recording
  and production library behavior remain unchanged.

## Consequences

- Files and open descriptors grow with configured peak handler concurrency.
  Evidence volume and synchronous flush requirements do not change.
- The pool coordinates access briefly; it does not serialize file writes
  behind one writer lock. Buffered writes still do not promise host-crash
  durability. PostgreSQL write pressure remains a separate throughput limit.
- Recorder microbenchmarks and shortened scenarios validate the mechanism;
  a matched full-duration run is required for steady-state rate claims.
