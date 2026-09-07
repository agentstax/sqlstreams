---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Compaction options are constructed inline

## Context

NewCompactionOptions only returned an enabled struct and a nil error.
The user requested its removal and inline options at every call site.

## Decision

- Supersede the constructor choice in
  [0612](0612-message-key-promotion.md); preserve its message-key placement,
  explicit Enable flag, rank ordering, and delivery behavior.
- Construct CompactionOptions inline with Enable true and the desired Rank.
  Omitted Rank is zero. Remove the constructor and its Vulkan alias.
- Keep validation at produce time, including rejection of enabled compaction
  without a message key and a nonzero rank with Enable false.
- Migrate library callers, examples, benchmarks, and current documentation.
  Record CompactionOptions as an exception to the constructor convention.

## Consequences

Callers no longer handle an error that cannot occur. This is a pre-v1 source
API removal; compaction semantics and stored data are unchanged.
