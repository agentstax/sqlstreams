---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# Heartbeats are jittered like ticks

## Context

With [0779]'s losing-claim convoy gone, the 160-stream three-replica
cell showed the renew transaction slowing from 0.04 ms to 7 ms over a
ten-minute hold at a constant 119 renewals a second, with no claim
traffic, steady bloat, and no pinned snapshot. Per-second WAL records
spiked to 7k-14k every 15 s over a floor of 300; two probe samples
caught 100 to 170 backends active at once on `BufferContent` and WAL
locks while the rest showed one to five. The instance runner's heartbeat
was a bare `time.NewTicker(InstanceTTL / 2)`: every instance a fleet
claims in the same minute renews in the same instant forever, on the
same heap pages and the rightmost leaf of the `expires_at` index. Every
other pacing loop in the library jitters; the convoy had been smearing
this one by accident.

## Decision

- `InstanceRunnerConfig.JitterFraction` (default 0.1, must be below 1),
  the tick runner passing its own fraction through.
- The heartbeat is a timer re-armed after each renewal at
  `InstanceTTL/2 × (1 ± JitterFraction)`, so instances claimed together
  drift apart; the widest wait is 0.55 of the ttl, inside the lease.

## Consequences

- Renewals spread across the beat instead of landing in one instant; the
  cost of a renewal no longer grows with the fleet's size.
- A unit test pins the delay bounds; the confirming cell is cited in
  HISTORY.
