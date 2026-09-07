---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Alert history uses stored message time

## Context

The user accepted storage-time evidence as simpler than source timestamps
encoded in compaction ranks. A delayed write counting as fresh evidence is
an accepted tradeoff, not a case requiring additional ordering machinery.

## Decision

- Supersede the evidence-clock and rank-read choices in
  [0683](0683-alert-pending-duration-is-derived-from-measurement-history.md)
  and [0686](0686-history-based-pending-belongs-to-the-shared-alert-framework.md).
  Preserve their history-derived pending, shared alert framework, independent
  collector checks, and no user-managed state. Continue implementation through
  [0689](0689-alert-implementation-starts-in-existing-recording.md).
- Use StoredMessage.CreatedAt for evidence windows, pending duration, gaps,
  freshness, and historical collector age. Order by CreatedAt with message
  id breaking ties. Read all retained evidence within the required window.
- Produce evidence with rank zero, letting message id select the compaction
  head. Measurement.At remains metadata, not the alert evidence clock.
  Remove the partition-count source timestamp query and its row wrapper.
- Keep existing fresh recovery, insufficient-evidence behavior, repeat rules,
  and atomic recording. Do not enable pending or change cadence in this step.

## Consequences

A delayed write can count an older observation as fresh and affect pending
or recovery. The user accepts that behavior; revisit only if operational
experience warrants it. CreatedAt is the existing database NOW() default
(transaction start), not a claim of commit order. No schema change is needed.
The next history integration needs a CreatedAt-window query, not timestamp
ranks; the previous generic rank read is not required by this design.
