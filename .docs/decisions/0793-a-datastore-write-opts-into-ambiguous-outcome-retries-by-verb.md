---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# A datastore write opts into ambiguous-outcome retries by verb

## Context

`DatastoreRetry.Wrap` retried every transient SQLSTATE, including the
ones that mean a statement may have committed before the connection died
(08000, 08006, 08007, 40003, 57P01 to 57P05, driver timeouts, net
errors). A wrapped read replays for free. A wrapped write replays
against whatever the first attempt left, and only the SQL's own guard
makes that safe. The classifier carried a comment saying every Wrap
call site had been audited for this; nothing made that true for the
next write.

A first attempt at enforcement was a ledger of one prose line per
wrapped write, checked for presence by a conventions test. It was
rejected: the test could confirm a sentence existed and nothing about
whether it held, so it was a rubber stamp with an AST walk under it.
The audit it drove was kept; its findings are below.

## Decision

- `WrapNonIdempotent` retries only errors the server rolled back or never
  received: deadlock, serialization failure, a connection never
  established, a confirmed cancel, `pgconn.SafeToRetry`. A lost
  connection after a statement shipped returns to the caller as it is.
- `WrapIdempotent` retries those too. It is the verb for a read and for
  a write whose SQL guards a second run: a token match, ON CONFLICT, IF
  NOT EXISTS, a predicate the first run emptied.
- `IsTransientDatastoreError` keeps its broad meaning ("can an unchanged
  retry succeed?") for the consumer loops that end a session on a
  permanent cause; `IsRetryableDatastoreError` is the narrow one Wrap
  uses.
- The choice is made at the call site by the author who can read the
  SQL, and nowhere else. Both verbs carry the closure's property in their
  name and there is no bare default, so a copied neighbor's verb is
  visible on the diff line. CONVENTIONS ## Datastores names which verb a
  write takes; the rule is review-enforced.

## Consequences

- Ten public methods use `WrapNonIdempotent` because no guard covers a replay:
  the delivery lifecycle claim and its three outcome writes, which key on
  group and message id with no token; the exception claim and the fresh
  range claim, whose replay would cost an attempt or a reclaim; the
  worker instance claim and its failure counter; the migration step and
  its failure record. Their callers receive the ambiguous error and
  already handle it: the claim loops claim again on the next poll, the
  runner rides out lease expiry, the migrate command reports it.
- Every other wrapped method, 81 of them, moved to `WrapIdempotent` and
  keeps the retry behavior it had.
- A future write copied from a neighbor takes whichever verb the
  neighbor has; a wrong `WrapNonIdempotent` costs a retry on a rare code,
  a wrong `WrapIdempotent` costs a double write. Review checks the verb
  against the SQL's guard.
- **Rejected:** the presence-checked ledger; a retry-class parameter on
  one verb, which reads as a boolean at every call site; a bare `Wrap`
  as the non-idempotent verb, which made the safe choice the unnamed one.
