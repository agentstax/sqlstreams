---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# The record tables are produce_record and handler_record

Supersedes [0687]'s table-name clause only; the rest of that record stands.

## Context

[0687] named the checker's tables `produce_ledger`, `handler_ledger`,
`run_phase`. Building the lab, "ledger" was dropped from every package
and type name in favour of "record": the package that declares the row
shapes is `record`, its types are `ProduceRecord`, `HandlerRecord`,
`PhaseRecord`, and the roles' files are `<name>.produce.jsonl` and
so on. The two table names were the last place the borrowed word
survived, and a reader of the SQL met a noun the code never uses.

## Decision

- The checker's tables are `produce_record`, `handler_record`,
  `run_phase`, in schema `lab`, one per record file kind, columns named
  by the JSON-lines fields.
- "Record" is the lab's noun for a row a role writes; "ledger" is not
  used in code, SQL, or the report. Prose in earlier records stands as
  written.

## Consequences

- The `lab.*` diagram and the sabotage example on the reliability-lab
  page name the new tables.
- No migration: the checker drops and recreates the schema on every run.
