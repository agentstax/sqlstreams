---
status: accepted
date: 2026-09-11
phase: "pre-v1"
---

# Throughput scenarios can disable message recording while retaining aggregate measurements

## Context

Per-message JSON recording materially changes the throughput being measured.
The user approved disabling it for throughput scenarios while preserving full
recording for reliability scenarios. This supersedes the always-record policy
in [0745](0745-maximum-throughput-uses-the-reliability-lab.md); its other workload,
environment, retention, and warmup decisions remain in force.

## Decision

- Scenario.DisableMessageRecording defaults false. Set it true for throughput
  and max-throughput; existing reliability scenarios retain full records.
- Disabled mode bypasses construction and writing of per-message producer and
  handler records. Record cumulative counters once per second per process and
  stream/group, plus a final snapshot at graceful shutdown.
- Keep the existing roles, observer, checker, resource measurements, phase
  records, results, and run index. Rate calculations use actual sampled spans,
  summing independent process rates; missing or decreasing counters are errors.
- Keep attempted/committed/rejected/unknown and handler success/error counts.
  Disabled scenarios must declare an errors expectation. Handler totals and
  error checks describe loaded snapshots, not a per-message proof.
- Mark identity-based checks and schedule adherence unavailable in both JSON
  and printed results. Detailed latency is unavailable; never print zero
  latency as a measurement. A paced phase cannot claim all guards held when
  schedule adherence was unavailable.
- Record the flag in the declaration and verdict. A passing aggregate run
  proves only the checks that ran; it does not prove exactly-once handling,
  absence of message loss, or crash-recoverable evidence completeness.

## Consequences

- Throughput runs avoid recorder disk traffic and large post-run ledger imports.
- Reliability and future failure scenarios keep the default full evidence.
- No alternative production-library behavior or public client API is introduced.
- Measurement endpoints exclude partial sampled intervals; aggregate throughput
  is an observed rate, not reconstruction of every message's completion time.
