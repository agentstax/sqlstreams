---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# The payload never reaches a log line or an error

## Context

A review of every log call and raise site found the user's payload
reaching logs on two paths. The schedule redeclaration line (VK0062)
carried the old and new payload and Metadata documents as `old -> new`
text. pgx's encode failure wraps the value it could not encode with
`%#v`, so a payload that encoding/json rejects (a NaN float, a channel
field, a MarshalJSON error) came back from Produce as an error whose
text was the whole document, and the schedule producer logged it at
Warn. A third path was user-controlled rather than Vulkan's: the
delivery-consumer dead-letter line (VK0029) logged the handler's own
error text, which is also stored in `last_error`.

## Decision

A user document -- a message payload, a schedule's Metadata -- never
reaches a log line or an error value: not as an attribute, not inside a
row struct, not through `%v` of a struct that holds one, not through a
wrapped driver error.

- The datastore that binds a payload encodes it with json.Marshal first
  and raises `common.ErrPayloadNotEncodable` (VK0097, Permanent) on
  failure. encoding/json's errors name the type or field, never the
  value. This is the one exception to the "hand pgx the `any`" rule.
- A redeclaration line reports a changed document as `payload_changed`
  or `metadata_changed` and nothing of what it holds.
- The dead-letter line carries ids only; the failure text lives in
  `last_error`, which the event's diagnose query reads.
- Keys (`message_key`, `idempotency_key`) are identifiers and stay in
  logs and errors; the doc pages say so where the key is introduced.

## Consequences

Produce and Scheduler.Register encode the payload once in Go instead of
inside pgx; the bytes go to the driver as json.RawMessage. A payload
that cannot be encoded fails fast with a Permanent code instead of
burning a retry curve. The VK0062 and VK0029 pages describe the new
attribute set. A handler that formats the payload into its returned
error still puts it in `last_error`; the dead-letters guide names that
as the user's choice.
