---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# The reliability lab is a ledger and a checker, with scenarios written as Go and printed, never parsed

## Context

The failure-injection labs each prove one mechanism for a few minutes.
Nothing runs for an hour, runs everything at once, and accounts for
every message. Research across Kafka's system tests, Jepsen's queue
checkers, Redpanda's chaos harness, etcd's robustness suite, and the
Postgres job-queue projects (which ship no such harness at all)
converged on one shape and a handful of rules those suites learned
the hard way. The proposal page is
`website/src/content/docs/concepts/reliability-lab.mdx`.

## Decision

- The lab lives under `bench/reliability/`: one binary with a role flag
  (producer, consumer, checker), a compose file, `just reliability-lab`.
- Every produce writes two ledger facts: attempted, then the outcome --
  committed with its idempotency key, rejected with its VK code, or
  unknown when the reply was lost. Every handler invocation writes one
  fact: message id, group, attempt, outcome.
- The checker runs after producers stop and consumers drain until each
  producer's last committed key has a delivery outcome, bounded by a
  budget that fails the run. Its buckets are Jepsen's `total-queue`:
  lost (committed, never delivered) and unexpected (delivered, never
  attempted) fail; duplicated and recovered (unknown produce that
  appeared) are reported. A rejected produce whose row appears fails.
  The partition `produced = success + dead + compacted away + other
  schema version` has no "dropped" bucket.
- Safety checks ignore chaos windows. Reclaims, dead rows under fail
  rate 0, and recovery time are judged against `run_phase` rows.
- The verdict is pass, fail, or unknown; unknown when a fault the
  scenario names never happened, nothing was produced, or the checker
  errored. Exit codes 0, 1, 2, and 3 for lab-infrastructure failure.
- Each scenario is a hand-written Go declaration. The `.scenario` text
  file beside it is a description for readers; the report prints the
  scenario back from the Go value in the file's format, and a test
  diffs that output against the file. There is no parser.
- Ledger transport is a JSON-lines file per role on a mounted volume,
  COPY'd into a `lab` schema on the same Postgres by the checker; the
  report is a SQL join. The verdict record carries `synchronous_commit`.
- Producers are open loop with a pacer measuring from scheduled time;
  `saturate N in flight` is the one closed-loop phase and is labelled.
- Docker is driven by shelling out to the CLI; no Docker SDK.

## Consequences

- v1 is one plain topic, one group, fail rate 0, constant rate, fixed
  instance count, the six checks above, and two scenarios (quiet, dev).
- Every later stage adds one bucket and one fault the run must prove it
  reached: kills and Postgres pause, compacted keys with a per-key
  order check, schema-version mix, bindings, metrics assertions.
- A green run is trusted only after sabotage: a deleted message_log
  row and a dropped handler line must each turn the verdict to fail.
- The lab is scaled down by shortening phases, never lease or timeout
  durations; a dev run that never crosses a lease expiry reads unknown.
- **Rejected:** a scenario-file parser (two sources of truth solved by
  a printer instead); writing the ledger straight into the Postgres
  under test (doubles its write load at 2000/s); a boolean verdict; the
  Docker Go SDK (deprecated at v29, 24 transitive requires).
