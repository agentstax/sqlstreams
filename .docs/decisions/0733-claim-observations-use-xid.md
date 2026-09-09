---
status: accepted
date: 2026-09-09
phase: "pre-v1"
---

# Claim observations use xid

## Context

The active claim observation holds its own transaction id, but its SQL
alias and Go field still called it xmax, the former snapshot boundary.
The user approved xid, Xid, and pending_xid as the shorter accurate names.

## Decision

Name the observation SQL value xid, its Go field Xid, and the stored
cursor column pending_xid. Keep PostgreSQL snapshot xmax terminology where
it still describes that actual value, including the regression test.
This names the observation mechanism retained by [0715](0715-throughput-excludes-archived-delivery-consumer.md).

## Consequences

The pre-v1 table definition, SQL callers, website sandbox, and current docs
use the new names together. Existing databases with pending_xmax need their
cursor tables migrated or recreated before running this code. No database
is changed by this rename. Claim behavior is unchanged.
