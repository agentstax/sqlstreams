# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
.docs/decisions/.

## Idle-fleet fix [0779]

Three changes, one item; the 2026-09-12 cells to beat are in
`.bench/results/idle-fleet-*/runs.jsonl` (0.6 Postgres cores idle at 990
rows on one replica; 92% of statement time in the losing claim's row lock
at three replicas; 9,630 rows never finished registering).

Read from the 160-stream, one-replica cell: 814 losing claims/s, of which
480/s are the 160 group managers on the three system rows and 320/s are
the system manager on the stream janitor and cursor advancer rows the
group managers hold. Each group manager also lists its chain and runs two
sweep DELETEs every second; each cursor advancer runs two statements a
second.

- [x] C. The group manager keeps its group's and its stream's rows and
  drops the three system rows [0780]. A `DisableManager` consumer keeps
  alive its consumers, its cursor advancer, and its stream's janitor and
  vacuum. The client and manager pages and the `DisableManager` comment
  no longer claim the opt-out removes DDL. Build, vet, `go test -race`
  on pkg/consumer and client green.
- [x] B. Lock-free losing claim: `claimInstance` reads `target_instances`
  and the live count in one unlocked statement and returns declined
  before `Begin`; the locked path is unchanged. Integration test
  `TestDeclinedClaimDoesNotWaitOnTheWorkerRowLock` fails without the
  change (times out on the lock) and passes with it; the worker
  directory is green under `-race`.
- [x] A, reshaped: the claimant backs off, live instances are left
  alone. The pool remembers a declined claim per row and does not try it
  again until its `ManagerConfig.ClaimRetry` backoff passes (a
  `common.RetryPolicy`, base 1 s, cap 30 s, jittered); a start or a
  vanished row clears the streak. Tick runner and passes untouched. Unit test: a declined
  row is not re-provisioned on the next reconcile and a removed row
  forgets its retry. pkg/worker green under `-race`.
- [x] Heartbeat jitter [0783]: found by the re-run, renewals of a fleet
  claimed together landed in one instant every 15 s. The instance
  runner's heartbeat is now a re-jittered timer. Unit test on the delay
  bounds; pkg/worker and pkg/alert green under `-race`.
- [ ] Re-run `just bench idle-fleet-160 1 2m 1 3` and
  `just bench idle-fleet-1600 1 2m 1 1`; compare with the cells above.
- [ ] Decision record (0780) if the settled DisableManager line or the
  idle report shape departs from 0779; HISTORY entry citing the numbers;
  remove the ROADMAP item.
