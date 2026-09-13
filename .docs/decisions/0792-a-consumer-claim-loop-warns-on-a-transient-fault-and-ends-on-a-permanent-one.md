---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# A consumer claim loop warns on a transient fault and ends on a permanent one

## Context

The message consumer's prefetch loop treated every claim error that was
not a context cancel as a database blip: sleep one `ClaimPollRate`, claim
again. `RetryDatastore.Wrap` below it logs only at Debug and returns a
Permanent error without retrying, so a group whose cursor row was
deleted, whose grant was revoked, or whose claim hit a permanent error
code polled forever with no Warn line, no counter, and a live worker
row. The instance looked healthy while its backlog grew. The loop was
retry machinery that did not honor the rule the rest of the library
follows: retry stops immediately on Permanent.

The exception consumer's claim loop had the opposite policy, returning
every error and ending `Consume` on the first spent retry curve. Two
policies for one fact.

## Decision

- A claim loop classifies a claim error with
  `common.IsTransientDatastoreError`, the same question `Wrap` asks.
- Transient: the loop logs a declared Warn event, `SQL0108`
  "could not claim messages", with `group`, `stream_id`, `worker`,
  `delay`, and `error`, then waits one `ClaimPollRate` and claims again.
  The instance's suppression window collapses repeats.
- Permanent: the loop returns the error, which ends `Consume` with the
  cause. The stopped line still prints; the error is returned, never
  logged.
- Context cancellation stays the shutdown path it was.
- The message consumer's prefetch loop carries this now. The exception
  consumer's claim loop adopts the same split in its own change so both
  runners share one policy.

## Consequences

- A broken claim path ends the session instead of hiding; a
  misclassified Permanent ends a consumer where it previously spun, the
  direction the retry rule already points.
- A joined retry error whose transient attempts precede a permanent
  cause classifies as transient once, since `errors.AsType` finds the
  first Postgres error in the tree; the next claim carries the permanent
  cause alone and ends the session, so the delay is one poll rate.
- No new test: the loop is only observable through a running consumer
  against Postgres, and assemblers and the client carry no integration
  tests yet. The datastore verbs it calls keep their own tests.
- **Rejected:** a `claim_failed_count` session counter beside the Warn,
  deferred until the stop line's counters are revisited.
