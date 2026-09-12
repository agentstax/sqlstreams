---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# The multistream ladder is retired for unpaced ceiling holds

## Context

The multistream family stepped a paced total rate over one, four, and
sixteen streams to read the held rung per stream count. Nine full-length
runs on 2026-09-12 (three repetitions per stream count, every one passing
with two checkpoints inside it) held 4,000/s total at every stream count
and stopped between 4,300 and 4,700/s total, with Postgres under 1.6 of
its 8 cores. The ceiling did not move with stream count: it was the
producer's default pool of 10 connections shared by 16 batch workers per
stream, a client default, not stream contention or the database. The
runs cost 165 minutes and answered a configuration question. Evidence
stays under `.bench/results/multistream-{1,4,16}/`.

## Decision

- Retire `multistream-1`, `-4`, and `-16` and their constructor. Their
  results directories and ledgers stay as the evidence behind this record.
- The family is `multistream-unpaced-1`, `-4`, and `-16`: two groups on
  every stream with `DeliveryLogMode all`, explicit batches of 250 from
  four callers per stream, a 64-connection pool so no caller waits,
  aggregate counters, a 30-second warmup and a two-minute unpaced hold.
  Backlog and headroom are declared report: outrunning the consumers is
  the finding.
- The family's recurring decision is how many streams one deployment
  can carry and what fans out with them. One run per stream count read
  on 2026-09-12: production near 50,000/s at every count with the
  producer container's two-CPU cap as its wall; consumption 42,000/s on
  one stream, 90,000/s on four, 92,000/s on sixteen with the backlog
  slope falling from +25,674 to +424; Postgres at 3.7, 6.1, and 5.9
  cores; WAL per message flat at about 2 KB. Streams do not contend in
  Postgres; a single stream's groups contend with each other.
- A number from this family reaches the doc site only from three
  repetitions of that stream count, as [0748] requires for max-throughput.

## Consequences

Sixteen minutes replaces 165 for the family's question. Paced holds and
schedule adherence live in `quiet`; identity checks under full recording
live there too. The pool finding is user guidance: sixteen batch workers
on the default ten-connection pool queue at about 4,500 produces per
second, and `PostgresConnectionConfig.MaxConns` is the setting that moves
it. [0750]'s "experimental" label on the family is lifted.
