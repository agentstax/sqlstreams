---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# The reliability lab is a ledger and a checker, with scenarios written as Go and printed, never parsed

## Context

The failure-injection labs each prove one mechanism for a few minutes;
nothing runs for an hour and accounts for every message. Kafka, Jepsen,
Redpanda, and etcd converged on one shape; the Postgres job-queue
projects ship no such harness. The proposal page is
`website/src/content/docs/concepts/reliability-lab.mdx`.

## Decision

- The lab is `bench/reliability/`: one binary with a role flag
  (producer, consumer, checker), a compose file, `just reliability-lab`.
- The idempotency key is `<producer>-<seq>`. Every produce appends two
  ledger facts: attempted, then the outcome -- committed, rejected with
  its VK code, or unknown when the reply was lost. Every handler
  invocation appends one fact: message id, key, group, attempt, outcome.
  Tables `produce_ledger`, `handler_ledger`, `run_phase` share the
  JSON-lines column names.
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
- The verdict is pass, fail, or unknown (a named fault never happened,
  nothing produced, checker errored); exit 0, 1, 2, 3 for lab failure.
- Each scenario is a hand-written Go declaration; the `.scenario` text
  file beside it is for readers. The report prints the scenario back
  from the Go value in that format and a test diffs it. No parser.
- Ledger transport is a JSON-lines file per role on a mounted volume,
  COPY'd into a `lab` schema by the checker; the report is a SQL join.
  The verdict record carries `synchronous_commit`.
- Producers are open loop, latency measured from scheduled time;
  `saturate N in flight` is the one closed-loop phase. Docker is driven
  by shelling out to the CLI.

## Consequences

- v1 is one plain topic, one group, fail rate 0, constant rate, fixed
  instance count, the six checks above, and two scenarios (quiet, dev).
- Every later stage adds one bucket and one fault the run must prove it
  reached: kills, Postgres pause, compacted keys, schema mix, bindings.
- A green run is trusted only after sabotage (a deleted message_log
  row, a dropped handler line) turns the verdict to fail. Scale-down
  shortens phases, never leases or timeouts.
- **Rejected:** a scenario-file parser (a printer solves two sources
  of truth); the ledger written straight into the Postgres under test
  (doubles its write load); a boolean verdict; the Docker Go SDK.
