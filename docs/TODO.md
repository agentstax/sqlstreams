# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## Admin responsibilities and consistent patterns [0676]

Agreed design; implementation in progress. Review the concrete diffs against
the code shapes agreed in discussion before closing this work.

Admin owns assembly, cross-domain resolution, operation policy, and
delegation. Keep system bootstrap, migration dispatch, owner resolution,
reserved-topic protection, AllowDestroy/Force, and composed destruction
guards there. Controllers own domain verbs and persistence invariants;
datastores own SQL. Simple input checks may repeat explicitly at boundaries.

- [x] Align the binding rules in CONVENTIONS.md with this division, including
  domain-owned config validation and explicit duplication of simple guards.
  Audit admin's methods against the rule; avoid moving orchestration merely
  because it touches several domains. Move RenameTopic's same-name rejection
  into TopicController.Rename. Remove redundant forwarding-only guards where
  the controller already handles them; preserve intentional preflight checks.

Task 1 audit: GetTopic, TopicHealth, and schedule reads/actions delegate name
validation to their immediate controller call. Consumer-name checks in
GetConsumer, GetBinding, and ConsumerGroupOwner stay before topic lookup;
registration, rename, and destruction retain admin preflight/policy guards.
System, owner, migration, metrics, alert, binding, compaction, and destruction
composition stay in admin. Worker filtering and health placement remain the
separate tasks below.

Task 1 diagnostic changes: empty topic-read and schedule names now return the
controller's `name is required`; a reserved topic renamed to itself returns
ErrReservedTopicName before the controller's same-name guard. Invalid identical
names reach the controller's name-pattern rejection first. These calls still
fail before persistence. Targeted build and race checks for admin, topic/controller,
and vulkan, plus tools/conventions, reserved-topic-lab, schedule-lab, and
schema-evolution-lab passed.

- [x] Validate topic registration before bootstrap writes. In
  MessageAdmin.RegisterTopic, perform the existing explicit name-pattern check
  and nil/default/config validation before RegisterSystem can run. Retain the
  controller's explicit name check and config validation for direct callers.
  Use the existing SlugPattern and TopicConfig methods; add no TopicName type,
  ValidateName helper, or ValidateRegistration API. Preserve automatic
  register-if-absent bootstrap and existing custom system configuration.
  Invalid input must return its validation error without creating resources;
  this does not promise atomic rollback of later registration failures.

Task 2: explicit name-pattern and config validation now precede the system
lookup. Existing empty-name/reserved-name errors retain their precedence;
controller validation is unchanged. The architecture page and quickstart
describe the approved behavior. register-idempotency-lab now uses an isolated
installation and verifies invalid names and every config constraint create
no namespace, valid registration bootstraps, redeclaration semantics hold,
nil config uses defaults, and a custom system alert schedule is preserved.
Targeted build, race checks, conventions checks, and the live lab passed.

- [x] Add WorkerController.ListConsumerGroupWorkers(ctx, owner), validating
  the required consumer-group identity, and the datastore's corresponding
  public retry wrapper/private query pair taking consumerGroupId. Select
  worker_config rows with consumer_group_id = $1 using explicit columns,
  existing owner joins, the existing list row shape, and toWorker handling
  (including the current warning/skip behavior for unreadable owners).
  Admin resolves ConsumerGroupOwner and delegates; delete its post-read
  filtering. Leave the manager's ListWorkers call and owner-chain query
  unchanged. No scope enum or generic selection API; straightforward query
  duplication is acceptable.

Task 3: admin delegates directly to the new consumer-group read. Its SQL
selects consumer_group_id, reuses the existing row/adaptation shape, and
keeps warning/skip handling for unreadable selected owners. Ancestor rows
are no longer read or diagnosed by this group-only operation; the manager's
ListWorkers query and call are unchanged. The expanded consumer-group lab
checks sibling and cross-topic exclusion, full worker declaration equality,
empty results, owner validation, and the manager's ancestor selection.
Targeted build/race and conventions checks passed; the consumer-group lab
passed with -race and worker-claim-lab passed through failover and release.

- [ ] Move TopicVersionHealth to pkg/topic with identical fields and JSON
  tags; update the vulkan alias. Keep TopicHealth composition in admin and
  inline evaluate's logic inside its snapshot loop: compaction heads take
  precedence; otherwise collect groups with Unconsumed > 0 or
  UnresolvedExceptions > 0; otherwise set Safe. Preserve exact Reason text,
  group ordering, and result ordering. Remove evaluate; add no new verdict
  helper or metrics computation. Verify vocabulary imports remain acyclic.
- [ ] Before implementing public behavior changes, write the affected
  doc-site proposal marked Proposed and review it with the user. Explain
  validation-before-bootstrap and preserve the existing error/log contracts
  for worker selection and health; update shipped prose only when implemented.
- [ ] Verify each change in the foreground: build, go test -race on touched
  packages, and directly affected labs. Cover invalid name/config on an empty
  database leaving no bootstrap resources, valid bootstrap and custom-system
  preservation, group-only worker results versus unchanged manager selection,
  and the three health verdict branches. Run alias/convention checks for the
  type move and rule changes. Full fresh-DB labs only at review-ready checkpoint
  or on request; no release checks unless this becomes a release checkpoint.
- [ ] Close out after implementation and verification: remove Proposed labels,
  add the shipped HISTORY entry citing [0676], and remove this TODO section and
  its ROADMAP entry. Do not edit docs/archive/explain-it-back.md.
