---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# A declined claim backs off in the pool, and live instances keep their poll rate

## Context

[0779] picked rung 2: a tick that made no progress backs off toward ten
times `poll_rate`. Built, it needed the pass to say "idle" through the
tick runner's seam, and every shape of that report was wrong: a bare bool
that three passes had to fake, a disable knob for the schedule producer,
a named error with a code and a docs page for a non-fault, or a flag the
pass sets on its runner. The seam was wrong because the question was.

Two things were called idle. A live instance with nothing to do, such as
a cursor advancer whose committed did not move, costs a statement or two
a tick and honors a cadence its row declares. A manager whose reconcile
re-claims rows another process holds costs a claim per row per tick and
was 480 of the 814 losing claims a second in the 160-stream cell. Only
the second is the problem, and the codebase already has its shape: a
declined manager-row claim waits out `RunnerConfig.RetryDelay`, and every
retry in the library climbs a `common.RetryPolicy`.

## Decision

- The instance tick runner and every pass keep their shape; a live
  instance ticks at its row's `poll_rate`, idle or not.
- The manager's pool remembers each row whose claim was declined, with
  its decline streak and when the next attempt is due, and `diff` skips
  the row until then. A start clears the streak; a row gone from the
  table takes its streak with it.
- The delay is `ManagerConfig.ClaimRetry`, a `common.RetryPolicy` with
  base 1 s and cap 30 s, jittered by the manager's `JitterFraction`. The
  cap bounds a takeover after a holder stops, so it stays near
  `InstanceTTL`; the base re-claims a row released mid-rolling-restart
  within a second or two.
- Supersedes [0779]'s rung-2 clause; its other decisions stand.

## Consequences

- A manager beside rows it cannot claim costs one declined read per row
  per 30 s instead of per second; its chain listing and two sweeps still
  run every second.
- A row's holder that crashes is replaced within `InstanceTTL` plus up to
  30 s, the same bound the manager row already had.
- **Rejected:** an idle report from the pass, in any of the four shapes
  above; a poll-rate backoff for live instances.
