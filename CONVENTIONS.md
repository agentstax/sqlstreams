# Conventions

Codebase-wide rules. Violations are bugs, not style nits.

Five parts, each a reader's question: where code lives, how it reads,
how it persists, how it reports, and what sits outside the library. A
rule ending in `(checked)` is enforced by a test in `tools/conventions`
(`just verify`); every other rule is enforced by review. The why behind
a rule lives in `docs/decisions/`, indexed by `docs/DECISION_MAP.md`;
this file states only the rule.

# Part 1 -- Where code lives

## Dependencies

- Main module deps stay minimal: std lib + pgx + x/sync. Never
  `go get` a new dep for domain logic.
- When battle-tested code exists for a problem (e.g. cron parsing), VENDOR it:
  copy the needed source files + their tests + license verbatim into the owning
  package with provenance headers, take only the parts needed, keep local diffs
  to marked one-liners. Hand-roll only when nothing battle-tested fits.
- The nested modules are the sanctioned exception -- separate modules, not
  the library: cmd/vulkan (cobra/fang/lipgloss) and otelvulkan
  (otel/prometheus).

## Tooling

- Developer tooling lives in dev-only modules under `tools/` (own go.mod,
  never tagged, outside the root test surface). Production code never
  imports anything under tools/.
- `tools/conventions` runs the machine-checkable rules of this file as
  tests, via `just verify`. It reads library source as data, so its
  tests run with `-count=1` or a pass caches across library edits.
- `tools/compat` is its own nested module so its go.mod can pin a prior
  vulkan release; `just compat-lab` drives it.
- `tools/codeexport` and `tools/conventions` link every root that
  declares codes; a new declaring root is added to both.

## Package layout

Every package is exactly one of three kinds:

- **Infrastructure** -- `common` and its subpackages
  (`common/diagnostic`, `common/logging`, `common/concurrency`), plus
  `datastore`. Vocabulary and seams importable by everything.
- **Domain** -- a `pkg/<root>` vocabulary root, its `controller` and
  `controller/datastore`, and the worker packages that maintain the
  domain's tables (template: topic, consume). A thing domain's root is
  named for the resource (topic, system, worker, alert, metrics); an
  activity domain's root for the verb (schedule, migrate, compaction,
  consume, produce). The root's own controller and datastore are
  `<Root>Controller` / `<Root>Datastore` (ScheduleController); a worker
  package's keep the worker's name.
- **API package** (`producer`, `consumer`, `scheduler`, `admin`,
  `systemmanager`, `vulkan`) -- constructors, configs, instances;
  assembles domains and workers. An activity domain's assembler is its
  agent noun (produce -> producer, consume -> consumer, schedule ->
  scheduler). Declares no codes, owns no SQL, holds no vocabulary.
  `vulkan` is the client plus aliases: it declares Client, ClientConfig,
  the pool and its config, the handles, and the three instance wrappers;
  every other exported name is an alias or var into the declaring package,
  and the client holds assemblers only.

Admin owns assembly, cross-domain identity resolution, operation policy,
and delegation. System bootstrap, migration dispatch, reserved-topic
protection, AllowDestroy/Force, and composed destruction guards belong
there. A forwarding method needs no additional logic to justify its place.
Controllers own domain verbs and persistence invariants; datastores own SQL.

### The seam law

Anything another stack imports is a seam -- a vocabulary root or a
domain controller. What only your own tree imports nests freely
(producer keeps its controller/datastore/batcher: nothing else imports
them).

### The placement law

A worker package lives under the domain whose tables it maintains,
never under its assembler.

### The one-declaration law

Every exported type, const, named error, and declared event is declared
once, in the lowest package that reads it, with a floor:

- Machinery (a controller, datastore, batcher, or worker package)
  declares nothing a user spells except its own Config and `*Row`
  structs and its controller / datastore / instance / provisioner
  types. (checked)
- So a user-spelled name lives in exactly one of `common` (shared
  vocabulary without a single domain owner), a root (vocabulary or
  resource declarations owned by that domain), or an assembler (inputs
  specific to its operations).
- A resource declaration stays with its domain even when an assembler
  applies it across several domains; the orchestration's location does
  not determine the declaration's owner.
- The only `type X = pkg.X` lines in the repo are pkg/vulkan/alias.go,
  an alias keeps its declaration's name, and the alias set is computed
  by the closure test, never hand-kept: whatever pkg/vulkan's exported
  surface reaches must be spelled there. pkg/vulkan imports no
  machinery. (checked)
- A root type whose bare name is a generic noun (Kind, Status, Severity,
  Unit, Error, Expression) takes the root's noun as its prefix --
  MetricKind, AlertStatus, ScheduleExpression, DiagnosticError -- and
  its consts follow the type's prefix (MetricKindCounter, the
  DeliveryLogMode pattern); the rename lands on the declaration, never
  on the alias.

### The domain layers

- `pkg/<x>` -- vocabulary: pure read-models, consts, named error
  variables, declared events and metrics, and resource declaration inputs
  (TopicConfig, SystemConfig), including declarations applied by an assembler.
  Imports infrastructure and, for domain-owned data composition, other
  vocabulary roots. Root-to-root dependencies must remain acyclic; roots
  never import controllers, datastores, workers, or assemblers, except the
  infrastructure `pkg/datastore`. No constructors for read-models, no
  fields without production readers.
- `pkg/<x>/controller` -- the only path to persistence: domain verbs,
  input validation before persistence, `to*` adapters, schema asserts.
- `pkg/<x>/controller/datastore` -- all SQL; trusts inputs, no re-validation.
  Table-exact `*Row` structs live in `model.go`, never beside the query that
  returns them. An enum type travels with its const block.
- Import arrows point strictly downward.

### Read-models

- Every public read-model field carries a `json:"snake_case"` tag -- the
  wire name is the field's contract, the json sibling of the datastore
  `db:` rule. Keys spell the log attribute registry's name where one exists
  (topic, version, group, message_id); otherwise the field's own name
  snake_cased. Write shapes, configs, and instances carry no tags.

## Supported public API

- `pkg/vulkan` is the supported public entry point. Its exported names and
  all exported fields and methods reachable through its types, aliases,
  parameters, and results belong to that contract. Moving the entry package
  later does not change this boundary.
- Client, handle, and instance methods use unnamed results by default.
  Do not add result names solely to repeat what the verb and return type
  already communicate. Keep return expressions explicit.
- Other packages remain importable for advanced use, without a stability
  commitment or guides presenting them as alternative public entry points.
  Do not move them to `internal/` solely to reduce the supported surface.
- Review exposure at its source. An alias exposes its exported methods as
  well as its fields; a declaration below `pkg/vulkan` is not exempt from
  review when reachable. The alias-closure tests verify that reachable
  library types can be named through `vulkan`; deleting an alias while
  leaving its type reachable is not a surface trim. Third-party types keep
  their upstream contracts.
- Every reachable declaration states its contract in its comment; the
  rules are under ## Comments.

## Structure

- Extend existing machinery, never build a parallel mechanism beside it.
  Start from "what one-clause change lets the existing path carry this?" An
  oversized diff for the feature's conceptual size is itself the smell.
- No new shared packages for logic both producer and consumer need -- write it
  on each side's datastore in its local style. Duplication beats abstraction.
  That rule covers logic with NO owning domain: when a domain package already
  owns the logic (an alert's condition, its thresholds and texts), each side
  builds that domain's controller from `ds` and injects it -- never a copy.
  An import cycle in the way is a layering bug to fix, not a reason to
  duplicate.
- One established mechanism per fact -- never introduce a second read path or
  derivation for something the codebase already computes one way.
- One concept = one named home. Policy tables become a pure classify function
  returning a named-action enum + an exhaustive switch driver. Prefer the
  simplest construct that fits the mental model.
- A function returns one value plus an error; three return types are the
  sign it is doing too much. First ask whether the extra value has a real
  consumer -- usually another verb already owns that fact, so delete it,
  don't wrap it. Only when callers genuinely need both does the pair become
  a named result struct (with its New<Struct> constructor). The comma-ok
  bool for expected absence is the exception.

# Part 2 -- How code reads

## Naming & terminology

- Never coin shorthand for a mechanism -- not in code, comments, or names.
  Describe things as the row/column/status/action they literally are.
  Design-round vocabulary (analogies, research jargon like "exclusive arc")
  is for discussion only -- translate back to the codebase's own nouns
  before writing names or comments. The ## Vocabulary table is the
  registry of banned terms and their replacements.
- Name a new method after the verb the codebase already uses for the same
  concept (Record*/Claim* pairs), not an outside domain's jargon.
- Use the plainest verb for the action -- `put`, `set`, `read`, `write`,
  `send`, `delete`. A more vivid verb (`stamp`, `hydrate`, `wire up`,
  `bake in`, `ferry`, `hand off`, `thread through`) must carry information
  the plain one doesn't; otherwise it is decoration the reader has to
  translate back. Applies to prose in comments, not just identifiers.
- When a bad name turns out to be an established pattern, fix every
  occurrence, not just the new one.
- Spell variable and param names out -- `message`, `instance`, `duration`,
  `policy`, `attempt`, `token`, `owner`. Don't truncate (`msg`, `inst`, `dur`,
  `pol`, `att`, `tok`, `own`) and don't reduce to initials (`m`, `i`, `p`).
  Vowel-dropping and first-syllable clipping are the same offense: the reader
  should never have to expand a name to know what it holds.
- The ONLY sanctioned abbreviations are the established house set -- `ctx`,
  `tx`, `cfg`, `ds`, `err`, `id`/`ids`, `sql`, `wg`. Nothing gets added to
  this list ad hoc; if a name feels too long, the type is probably doing too
  much.
- Single letters are for loop indices and receivers only (`i`, `p`, `c`), and
  the receiver matches the initial of the type's FINAL word -- `d` on
  `*TopicDatastore`, `c` on `*ControllerConfig`, `i` on `*JanitorInstance`,
  `d` on every `*Definition`. A single letter never holds a domain value.
- Every func param carries its own explicit type, never combined
  (`(topicId int64, version int32)`, not `(topicId, version int64)`).

### Exported type suffixes

A suffix names the type's semantic role, not the fact that it contains
fields. (checked)

| Suffix | Role |
| --- | --- |
| `<Noun>Handle` | a lazy identity plus client |
| `<Noun>` (bare) | a materialized domain resource or payload |
| `<Noun>Config` | configuration |
| `<Verb>Options`, `<Verb>Item` | command inputs |
| `<Verb>Result` | command output |
| `<Noun>Snapshot` | point-in-time observability |
| `<Noun>Status` | current state |
| `<Noun>Summary` | an aggregate projection |
| `<Noun>Health` | a readiness or retirement verdict |
| `<Noun>Row` | a table-exact datastore scan |
| `<Noun>Instance` | running process state |

- When a command type needs a subject qualifier, put the subject before the
  operation: `<Subject><Verb>Options`, `<Subject><Verb>Item`, or
  `<Subject><Verb>Result` (ScheduleRunOptions). Keep unqualified names when
  the operation identifies the activity (ProduceOptions, ConsumeOptions,
  ProduceItem, ProduceResult) or the type serves several subjects
  (DestroyOptions). Methods remain verb-first (RunSchedule). Settings for a
  concept use `<Noun>Options` (MessageOptions, CompactionOptions, AlertOptions).
- `Data` and `Info` are not exported type suffixes. They describe
  representation without identifying the value's role. Qualify a projection
  by its subject instead (`ScheduleGroupSummary`, `TopicVersionHealth`).

## Vocabulary

Consumer operations name the registered resource `Consumer` (GetConsumer,
DestroyConsumer; CLI `consumer` and `--consumer`). One running instance is
`ConsumerInstance`. Shared identity, ownership, and state across instances
use `ConsumerGroup` (ConsumerGroupId, ConsumerGroupOwner,
ConsumerGroupSnapshot), even when only one instance currently runs. Do not
shorten new API or CLI names to bare `Group`.

One registry of banned terms for the whole repo -- code identifiers,
comments, log messages, and all user-facing prose including the doc site.
A term is banned when it hides the mechanism behind borrowed or coined
language; the replacement is the system's own noun in its plainest form,
understandable to a developer who has never seen Vulkan. A new banned
term or approved alternative adds its row in the same change that
surfaces it.

| Banned | Why | Use instead |
| --- | --- | --- |
| stream (Vulkan's noun) | a second name for what the API calls a topic | topic |
| event (Vulkan's rows) | the system's noun is message; "event" smuggles in event-sourcing expectations | message; the message log |
| offset | Kafka's position model; Vulkan positions are ids and cursors | message id; cursor |
| enqueue, publish | extra names for the API's one verb | produce |
| subscribe | not the API's verb | consume; declare the consumer group |
| job (the unit a consumer processes) | job-queue framing for what is a message | message |
| cron job | a second name for the resource the API calls a schedule | schedule; "cron expression" for the string it runs on |
| worker (a consuming process) | collides with Vulkan's own worker fleet | consumer instance; "worker" only for Vulkan's maintenance workers |
| ack, nack | protocol jargon for a protocol Vulkan doesn't have | the handler succeeds / returns an error; the delivery is recorded |
| reaper | vivid coinage for lease reclaim | "expired leases are reclaimed" |
| visibility timeout | SQS's term for what Vulkan calls a lease | lease; lease expiry |
| DLQ, dead-letter queue (as a place) | dead is a delivery status, not a separate queue | dead-lettered messages; delivery status dead |
| door | coinage for the controller layer | API package; "the controller -- the only path to persistence" |
| sentinel | coinage for a declared error value | named error variable; error value |
| attr, attrs | shorthand for a word the reader should never have to expand | attribute; log attribute (`slog.Attr` is stdlib and keeps its name) |
| hole (a template's blank) | coinage for a blank the reader fills in | placeholder |
| park, give-back, IOU, slot, settle, cede, squat, arm (a predicate branch) | coined mechanism shorthand | the row/column/status/action it literally is |
| work (a variable holding a message) | a job-queue noun for the deserialized message | `message` |
| staffed, unstaffed (a worker row) | coinage for the count of live claim rows | claimed, unclaimed (live worker_instance rows) |
| firing (an alert) | Prometheus's word for an alert status | active; a republish is a "repeat", never a "re-fire" |
| snooze | a job-queue verb for what is a handler-requested later run | delay; `consume.Delay`, the `delays` column, `RetryPolicy.MaxDelays` |
| allow, defer (concurrency policy values) | verbs for what the new message does; the values name what the key permits | parallel, exclusive, ordered (`deferred` stays the row status) |
| compaction key (the message's key) | the key is a message property; compaction is one of its two readers | message key (compaction_head's own compaction_key column keeps its name) |
| schema (a migration target) | a third sense of a word Postgres already owns; the rows a migrate command reports are the system and its topics, not schemas | name the resource -- `system`, `topic`; `schema` is the Postgres namespace and nothing else |
| control-plane schema (the shared tables) | same collision: the shared tables are not a Postgres schema | the control-plane tables |

## Constructors & configs

- Every new struct gets `New<Struct>(required params) (*Struct, error)` and
  call sites use it -- never bare literals. Exception: vocabulary read-models
  built only by controller adapters get no constructor.
- Required params inline in the signature; optional ones in a slim sparse
  Config struct. Never pass a whole data struct for a couple of fields.
- Param order is primary collaborator first, ambient last: the dep the struct
  is *about* leads, then its remaining deps, then `cfg`, and the bare
  `logger logging.Logger` always trails. A logger in the first position is
  the tell that a signature was copied from somewhere else -- readers scan
  position 1 for what the thing operates on.
- No functional-options pattern. Every config struct: exported
  `WithDefaults()` (fills zero fields, mutates + returns receiver) then
  `Validate()` (validates the RESOLVED config), both in the config's own
  file. Constructors nil-check required deps, then default+validate their
  own config, returning errors all the way through.
- A config file is named for the struct it declares, never bare `config.go`
  -- `<x>_config.go`, `controller_config.go`, `datastore_config.go`. A
  package that grows a second config gets a second file rather than a
  shared one.

### What a Config holds

- A Config struct holds ONLY optional fields: every field is either filled
  by WithDefaults or meaningful at zero. A Validate error on a field
  WithDefaults never fills is a required value hiding in the config -- move
  it into the constructor's params.
- A Config holds static values only -- never a func or other runnable
  field, even an optional one. Two things that must run together are
  composed in the layer that already holds both (the
  `vulkan.ConsumerInstance` wrapper running the system manager beside
  Consume), never through a callable on a config or a runnable param.
- Config fields order domain-first, grouped by concern with blank lines,
  ending with any per-loop retry curves (SweepRetry, TickRetry).
  WithDefaults and Validate walk fields in declaration order; a default
  computed from other fields may trail its inputs instead.

### Logger and Retry

- `Logger` and `Retry` are held once, on `PostgresDatastore`, filled from
  entry-point configs (`ClientConfig` and otelvulkan's pool-taking
  constructor configs) through `PostgresDatastoreConfig` -- configs below
  those entry points carry neither. `Retry` is read from `ds` everywhere.
- A constructor takes a trailing `logger logging.Logger` only when what it
  builds owns a suppression window or a bound identity `ds.Logger` lacks,
  or is part of something that does: a long-lived instance's parts
  (batcher, metrics producer), and every controller, datastore, worker
  provisioner, and runner an instance composes, so their Warn lines land
  in that instance's window.
- A top-level instance (system manager) opens its own window over
  `ds.Logger`; a caller with no window (admin, the CLI, a lab) passes
  `ds.Logger`.

### Validation

Config constraints live in the owning config's Validate method. An
assembler may call it, and perform simple input checks, before
coordinating domain operations; controllers still validate inputs for
direct callers.

- Prefer explicit duplication of simple guards to a new validation helper,
  type, or preflight API solely for deduplication. More complex validation
  may justify sharing; duplication alone does not.
- Remove guards from forwarding methods when the immediate controller call
  already checks them; retain preflight checks that reject input before
  other work or preserve deliberate error order.

## Pointers & receivers

- Pointer receivers on everything a constructor builds -- controllers,
  datastores, configs, workers. Value receivers only on small immutable
  vocabulary types (Owner's accessors, a string enum's `Validate`). Never
  mix receiver kinds on one type: pointer-receiver methods are absent from
  the value's method set, so `T` and `*T` satisfy interfaces differently
  and a copied value silently loses the mutating methods.
- A pointer param states that the callee mutates or shares the value --
  never a performance reflex. Flat row-shaped structs and stdlib values
  (`time.Time`, `uuid.UUID`) pass by value; nested read-models travel as
  pointers end to end. Slices and maps are already reference-backed, so
  element types are values (`[]Data` out of datastores) unless the element
  is itself pointer-classified (`[]*Topic`); never `[]*T` to make room for
  nil entries.
- Config structs are passed as `*Config` while being resolved --
  `WithDefaults()` mutates in place. A long-lived instance stores the
  resolved pointer as `Config *<X>Config`, the shape every instance,
  provisioner, and worker kind already has; never convert one to a value
  copy.
- A loop passing value-slice elements to a read-only adapter takes the range
  variable's address: `for _, data := range xs { toThing(&data) }`, never
  `&xs[i]`. Reserve `&xs[i]` for a callee that must mutate the element in
  place.
- Constructors return `(*Struct, error)`, nil on error: a caller that
  ignores the error panics at first use with a stack trace, instead of
  proceeding on a zero value that looks meaningful.
- Outside the error path a pointer is never nil (`*common.Owner` is the
  template): no nil-safe receivers, no nil-means-unset params. Expected
  absence is comma-ok; a param nothing can populate yet gets deleted, not
  nil-tolerated.
- A field's absence is its zero value, and only where zero can never be real
  data -- mark it with a `// "" if unset` comment. A zero that could be real
  data means the design needs a named state or a widened domain (Batcher's
  `ShutdownGrace < 0`), never a nil pointer; a bool whose default would be
  true gets the inverted `Disable*` name so zero stays the default under
  `WithDefaults`. Absence of a whole entity is a nil struct return from its
  Get (the `(nil, nil)` comma-ok); pointers keep their one meaning --
  mutation/sharing -- and never encode optionality.
- A struct holding a mutex, atomic, or connection pool is pointer-only:
  copying it copies the lock, which is a data race, not a style slip.
- A value copy is not isolation: any slice, map, or pointer field inside it
  still aliases the original's backing memory, so mutating a copy can break
  the original's invariants.
- Accept interfaces only at real seams (`logging.Logger`; `Querier` stays
  private); return concrete `(*Struct, error)`. Never return a concrete
  pointer through an interface-typed return -- a typed nil stored in an
  interface compares non-nil, so every downstream nil guard lies.

## File layout

- A package comment sits below the `package` clause, above the imports.
  Deliberate trade: godoc/pkg.go.dev only surface a comment placed above
  the clause -- these are for readers of the file, not the doc site.
- Top of file: only the file's free vars/consts. A const or var block owned
  by a type (an enum's values, a type's sentinels) stays glued to its type,
  never hoisted.
- Then each type's block: struct, New<Struct>, WithDefaults/Validate.
- Then methods. Files with exported methods: pair-by-pair -- each public
  immediately followed by its same-named private, then the next pair;
  deeper helper methods a private calls follow the pair that uses them.
  Never all publics then all privates. Files with no exported funcs:
  lifecycle order -- the entry point first, then each step in the order
  the running code reaches it.
- A helper is an unexported non-receiver func -- excluding a type's
  new<Struct> constructor, which stays in its type's block -- in a file that
  otherwise holds methods. Exported free funcs are API verbs and order with
  the other publics. Helpers go at the bottom of the file, behind one
  banner:

      // ***************
      // *** HELPERS ***
      // ***************

  Methods are never helpers -- a private method stays in the flow above the
  banner. A file of only free funcs (an adapter.go) has no banner.

## Blank lines

Function bodies read as paragraphs: one blank line between steps, none
inside a step.

- A step is the group of statements one comment could name. Two groups you
  would caption separately get a blank line between them.
- Glue -- never a blank line between a statement and what consumes its
  result: the `if err != nil` check, a nil/comma-ok branch on the returned
  value, the `defer` that releases what it acquired, the Exec/QueryRow that
  runs a SQL literal declared above it.
- A mid-body comment binds downward: blank line before the comment, never
  between the comment and its statement. Exception: switch/select arms are
  table rows -- a comment captioning a case stays glued on both sides.
- A validation preamble is one step: the input guards glued, one blank line
  after the last.
- At most one consecutive blank line inside a body; none directly after `{`
  or before `}`.

## Comments

- Avoid large blocks of text, break apart using formatting or single
  statements per line.
- Default is no comment. A comment earns its place only by stating a why or
  gotcha the adjacent code cannot show -- restating the code/SQL/signature
  below it, boundary/wiring narration, and design essays all get cut.
- 1-2 lines, rationale directly above the statement it explains. Parallel
  facts enumerate one per line (`condition -> outcome`).
- Every comment stands alone -- it is read cold by someone who was not in the
  discussion that produced it. Name the subject instead of opening mid-thought
  (`held for the length of a Consume call` -> `the permit is held for...`),
  finish the sentence (`the reclaim needs its own` -- its own what?), and never
  lean on a noun the surrounding code never introduces (`the drain rides this
  ctx` where nothing nearby is called a drain). A comment that argues against
  an alternative someone raised in review is that conversation leaking into the
  file: cut it, or state only the rule it settled on.
- Never point outside the code: no plan/phase references, no symbols that
  don't exist yet, no benchmark allusions.
- "Improve the comments" means delete first, then wordsmith survivors.

### The supported surface

The exception to "default is no comment": every declaration a caller
reaches through `vulkan` (the alias closure) states its contract.

- A field WithDefaults fills ends its comment with `Default: <value>.`
  (checked)
- A verb names each Err* variable it returns.
- A blocking verb says that ctx cancellation returns it and what runs
  beside it.
- A destructive verb names what it deletes and the
  ClientConfig.AllowDestroy gate.
- The path spelled is the caller's own (`client.Topic(name).Register`),
  never the machinery verb behind it.
- An aliased declaration's public-contract comments stay with its owning
  declaration.

# Part 3 -- Persistence

## Datastores

- Every public datastore method is EXACTLY a `DatastoreRetry.Wrap` around a
  same-named private method -- all SQL, scanning, and result shaping live in
  the private, even for one-query reads. A method that runs inside a caller's
  transaction cannot Wrap, and still keeps the pair: its public is a bare
  pass-through (`return d.claimDueSchedule(ctx, q, id)`). Never collapse a
  pair. File order is pair-by-pair per ## File layout.
- Every scan-destination row struct tags each field `db:"column"` with the
  column or alias its query returns -- the tag is the field's column
  contract regardless of scan style. Write shapes, derived outcomes, and
  composite aggregates carry no tags.
- Datastore methods are dumb resource verbs on the domain's own tables --
  get, register, delete, list. A caller-shaped read (a guard, a health
  check) is composed above the datastore, in the controller or admin, from
  those verbs or from the domain that already owns the fact (metrics
  snapshots, worker liveness). A datastore read that re-derives another
  mechanism's fact -- or reaches another domain's tables outside a
  transaction -- is a second read path, not a convenience.
- Method bodies are a linear sequence of named calls -- no inline shaping
  wads. `any` values go straight to pgx as query args (driver encodes JSONB;
  never hand-call json.Marshal); nil/empty shaping happens SQL-side
  (`NULLIF`, `COALESCE`). The one exception is the user's payload: the
  datastore that binds it calls json.Marshal first and raises
  `common.ErrPayloadNotEncodable` on failure, because pgx's own encode
  error prints the value it could not encode.
- Controllers own verbs, not tables. A datastore transaction contains every
  statement its operation needs, inline, even on tables another domain
  primarily manages.

### Transactions

- A transaction never crosses a package boundary -- no exported tx-taking
  methods, no `Querier`/`pgx.Tx` in any public signature. If an operation
  seems to need two controllers, it is one operation with a wrong home: pick
  the owner whose invariant the transaction protects.
- The ONE sanctioned crossing is the produce-transaction seam:
  `datastore.InTransaction` hands its closure a `datastore.Tx`, and a
  method built to run inside that closure takes the `Tx` (when it runs a
  ProducerFunc) or `q datastore.Querier` (when it only runs statements).
- `datastore.Querier` is the one statement seam: Exec / Query / QueryRow /
  SendBatch / CopyFrom -- what pool, conn, and tx can all do, minus
  transaction control. A private that runs inside a boundary it doesn't own
  takes `q datastore.Querier`; `pgx.Tx` appears only as a local in the
  private that owns Begin/Commit and in pkg/datastore's own
  transaction.go, which builds the `Tx`.
- No Beginner/pool interface, no wrapping of Rows/Row/CommandTag --
  pgx's own result types pass through. `*pgxpool.Conn` stays concrete in
  migrate: the advisory lock pins a session, and the concrete type is that
  contract.

## Tables

Every table is either shared control-plane schema or a member of one
topic's family -- never both.

- Shared: the catalog (system_config, topic_config, topic_config_log,
  consumer_group_config), the fleet (worker_config, worker_config_log,
  worker_instance, worker_instance_log, schedule_config, schedule_cursor), and cross-scope
  history (migration_log). Created by system createSystemTables.
- Per-topic: everything else -- message_log, idempotency_key,
  exception_queue, delivery_log, consumer_group_cursor, claim_lease,
  message_key_lease, compaction_head, binding_config, binding_config_log
  -- one physical table per topic, created by topic createTopicTables.
  Everything names them ONLY through pkg/topic's table-name funcs
  (`topic.MessageLogTable(topicId)`) -- library code, labs, and a user
  writing a diagnostic query alike.
- A new table splits per-topic when every row has exactly one owning topic
  (directly or through its consumer group) and no reader needs the table
  before knowing the topic. It stays shared when rows can exist at system
  scope with no topic at all, or when it is the catalog that resolves
  names to topic ids.
- A per-topic table carries no topic_id column -- the table name is the
  scope. A cross-topic read resolves topic ids from the catalog first,
  then loops the per-topic tables.
- Topic destroy DROPs the family's tables outright -- cleanup never runs
  cross-table DELETEs, and an after-destroy assertion checks table absence
  (to_regclass), never zero rows.

### Table names

Every table name is `<root>_<kind>`: the leading words name the resource
a row is about, the trailing word the table's kind. (checked)

| Kind | Holds |
| --- | --- |
| `_config` | declared state, written by declaration verbs |
| `_config_log` | that config table's declaration trail |
| `_log` | append-only event history; an event root stands alone with no sibling table (message_log, delivery_log, migration_log) |
| `_queue` | mutable work rows |
| `_lease` | expiring locks, prefixed by what is leased |
| `_instance` | live copies |
| `_cursor`, `_head` | singleton runtime state |

- A table 1:1 with another resource's rows carries that owner's name
  (consumer_group_cursor, schedule_cursor); a `_cursor` table keeps its
  own `id BIGSERIAL PRIMARY KEY` first and the owner's id as
  `NOT NULL UNIQUE`. (checked)
- FK columns keep the resource's noun (topic_id), never the table's name.
- idempotency_key is the standing exception outside the kind set.
- Every `_config` table carries `created_at` and `updated_at`, both
  `TIMESTAMPTZ NOT NULL DEFAULT NOW()`, as its last two columns before
  any constraint, and every UPDATE on a config row sets
  `updated_at = NOW()`. Consistency across the kind outranks the
  redundancy with the `_config_log` trail. (checked)

### Column names

| Pattern | Meaning |
| --- | --- |
| `<past participle>_at` | an instant a past event happened (created_at, attempted_at); expiry is always expires_at (checked) |
| `<verb>_after` | a lower-bound gate (can_run_after) (checked) |
| `<noun>_ns` | a duration, BIGINT nanoseconds (checked) |
| `payload` | the user's opaque document, everywhere |
| bare noun | a fact about the row itself |
| `<concept>_<noun>` | a fact about an attached concept (claim_lease.token vs exception_queue.lease_token) |
| `last_<noun>` | latest-of-many |
| singular (`attempt`) | an ordinal |
| plural (`attempts`, `reclaims`) | a running count |

- A version column is an ordinal and INTEGER; BIGINT is for ids, `_ns`
  durations, sizes, and compaction_rank -- never widen a version column.
- A table's own `id BIGSERIAL` surrogate is never dropped in favour of a
  natural key, and a column kept for flexibility
  (migration_log.consumer_group_id) is never dropped for being unwritten.

### Index names

An index is named `<table>_<columns>`: its table, then its leading
columns in index order, as many as it takes to be distinct from the
primary key and the table's other indexes (`worker_instance_expires_at`,
`worker_config_name_topic_id`). A partial predicate adds nothing to the
name. Postgres truncates names at 63 bytes, so a per-topic index checks
its length with a ten-digit topic id. (checked)

## SQL

- Never `SELECT *` -- always name columns explicitly. A column ADD must be
  invisible to live binaries built before the column existed; `SELECT *` makes
  even additive schema changes breaking (pgx errors when the field count no
  longer matches the scan destination count). Explicit column lists are what
  keep adds non-breaking, leaving column removal as the only change that needs
  the two-release expand/contract dance.
- That ban covers CTEs and subqueries, which have no scan destination to break:
  a `SELECT *` there silently widens with the table, so a later column lands in
  a `FOR UPDATE` row image or a join nobody re-read. Name the columns the CTE's
  own body reads and nothing more -- the list is then the CTE's contract, and a
  new column reaches it only when someone adds it deliberately.
- More than 3 selected columns go one per line. A wrapped column list hides
  which columns moved in a diff and makes the scan destinations hard to line
  up against. 3 or fewer stay on the `SELECT` line.
- Inline `--` comments right-align as a group, to the furthest-out one. A
  ragged comment column reads as unrelated notes; an aligned one reads as the
  table it is.
- No CHECK-constrained enums: a value-set CHECK makes every new value a
  migration. An enum-shaped TEXT column lists its values in an inline comment
  (`-- 'installed' | 'waiting'`); Go typing and validation are the only
  enforcement. Structural constraints (NOT NULL, FKs, uniqueness) stay in SQL.

### The literal

- Every SQL literal's first line is a comment naming its owner --
  `-- vulkan: <package>.<method>`. Constant text per query, so statement
  caching is unaffected; pg_stat_statements and the server log attribute
  load back to library verbs. (checked)
- A SQL literal is a raw string shaped one way everywhere: opening
  backtick then newline, the `-- vulkan:` owner comment and the statement
  indented one level past the declaring line, closing backtick on its own
  line at the declaring line's indent.
- Every table a literal names is schema-qualified `%[1]s.<name>`: the
  schema is Sprintf verb `[1]`, filled from the datastore's own `Schema`,
  and table names follow as `[2]`, `[3]`. Indexed, not positional -- the
  schema repeats at every reference. The pool sets no search_path, so an
  unqualified name resolves through the connection's own default --
  another installation's rows on a read, its table on a DROP, or nothing
  at all. An index name stays bare (an index lands in its table's
  schema), and a name reaching Postgres as a bind parameter or inside a
  quoted string is built qualified in Go at the call site. (checked)

## Migrations

- Pre-v1, every schema change edits the baseline `CREATE TABLE` DDL in place
  -- no ALTER/DROP trail. Removed tables' DDL is deleted outright. Verify by
  drop+recreate of the dev DB.
- Release-era changes are registry steps, and every step declares
  MinCompatibleVersion -- the oldest build schema version whose SQL still
  runs against the schema the step produces: 0 = additive, the step's own
  version = breaking. The gate admits a build iff
  `min_compatible_version <= build <= current`, so additive steps never lock
  out older binaries (the rolling-deploy window) and a breaking release
  means stopping older binaries before migrating.
- A release that changes only what binaries read still ships a step -- an
  empty version bump -- so later steps have a version to name. Column
  removal is the two-release shape: first a release whose binaries stop
  reading the column, shipping an empty bump; then the DROP step declaring
  MinCompatibleVersion = that bump's version.
- `just compat-lab` (tools/compat) is the empirical check at release
  checkpoints: the pinned prior release must match the registry's declared
  verdict.

# Part 4 -- Diagnostics

## Errors

Every error is five parts plus a recovery classification, carried by one
struct (diagnostic.DiagnosticError) and rendered by one renderer per
surface -- raise sites never format anything. Renderer mechanics live in
pkg/common/diagnostic/error.go and the CLI errorHandler; the rules here
are the choices the mechanism cannot make.

The whole shape, one example:

    // pkg/topic/errors.go -- the declaration owns everything but the values
    var ErrTopicNotFound = diagnostic.NewDiagnosticError("VK0005", diagnostic.RecoveryPermanent,
    	"topic not found",
    	"register it with Client.Topic(name).Register first")

    // raise site -- attach values, nothing else
    return topic.ErrTopicNotFound.With("topic", topicName, "version", version)

    // Error() one-liner (logs, wrapped chains) -- the code is the docs link
    topic not found: topic "orders", version 3 -- register it with
    Client.Topic(name).Register first [VK0005]

The CLI block, slog output, and --output json render these same parts as
fields; only the fix wording differs per surface (Go API in the library, a
vulkan command in the CLI).

Error and event declaration data is private and read through accessors.
Declare queries as trailing NewDiagnosticError/NewDiagnosticEvent arguments;
constructors copy query values before registering the completed declaration.
Queries returns detached values. With and Wrap retain per-raise copy behavior;
arbitrary attached application values are not deep-copied.

### When declaring a new error condition

- A condition earns a declaration (and code) by any one of: a caller in
  another package branches on it with errors.Is; its recovery must differ
  from what IsTransientDatastoreError concludes on its own; it is a
  user-facing condition worth a docs page. Constructor/config validation,
  internal invariant guards, and unexported same-package control-flow
  signals stay plain errors on the templates below -- promote one to a
  declaration the moment it crosses the boundary.
- Declare a named Err* variable in the owning pkg/<x>/errors.go via
  diagnostic.NewDiagnosticError -- code, recovery, problem, and fix fixed
  at declaration.
- Every code -- `diagnostic.NewDiagnosticError`, `NewDiagnosticEvent`,
  `NewDiagnosticMetric`, `NewDiagnosticAlert` -- initializes an exported
  var in the owning root's `errors.go`, `events.go`, `metrics.go`,
  `alerts.go`, or a subject-named `*_metrics.go`; a condition raised
  across different stacks lives in `pkg/common`. Machinery and assemblers
  declare none. Whichever layer detects the condition raises it -- admin
  for guards it composes, a datastore for facts its own query discovers.
  (checked)
- Code = "VK" + the next four-digit serial after the current max (same
  scheme as decision records). Never reuse or renumber; a deleted
  condition retires its number.
- Classify recovery by one question -- can an unchanged retry succeed?
  Transient = yes; Permanent = no. Retry machinery stops immediately on
  Permanent, so a wrong Transient burns a backoff curve on a lost cause.
- Land the docs page (…/errors/VK0005, headed by the verbatim problem
  text) in the same change -- readers and agents find it by pasting the
  message into search. Pages are hand-written under
  website/src/content/docs/errors/ (never generated); a change to a
  declaration's problem, recovery, or fix updates its page in the same
  change, and the page title stays the verbatim problem text.
- A declaration that carries diagnose queries points at `vulkan explain`
  by its own code; a query's placeholders name registered attributes,
  its columns are named, and its tables are schema-qualified. (checked)

### When writing the problem line

- One lowercase clause, fact only -- what is wrong and why. The fix is
  advice and lives in its own part, never blended into the fact.
- Use the template the condition kind already has: nil dep `<param> must
  not be nil` · required `<Field> is required` · empty `<param> must not
  be empty` · constraint `<Field> must be <constraint>` · absence
  `<noun> not found` · conflict `<noun> already <state>`.
- Tense follows recovery: Transient reads "could not <verb>"; Permanent
  reads "cannot" / "is" / "must". (checked)
- Never write: "failed", "invalid", "bad", "illegal", "unable", "unknown",
  "error", "please", "sorry", exclamation points, the raising function's
  name, or blame ("you passed"). "unrecognized <thing>: %q" replaces
  "unknown <thing>". The same ban covers event messages, metric
  descriptions, and alert descriptions. (checked)
- When the outcome could be unclear, state what did or did not happen
  ("nothing was published").

### When writing the fix

- One imperative action naming the exact field, method, or command with
  the caller's real values interpolated; a CLI fix runs verbatim as
  pasted -- one that doesn't is a bug.
- Interpolate with the same `{attribute_name}` placeholders a diagnose
  query uses, named from the log attribute registry under ## Logging.
  `Error()` and `LogValue()` fill them from the values the raise attached,
  and the fix text carries the quoting its position needs -- the value
  goes in raw (`register "{schedule}" with ...`).
- A fix placeholder must be attachable at EVERY raise site of its code:
  one fix string serves all of them, so a name one site cannot supply is
  a blank on a real operator's line. Diagnose queries are exempt -- a
  declaration carries an ordered SET of them, so a name-keyed and an
  id-keyed query can sit side by side and whichever value the line
  carries finds one it can fill. (checked)
- A closed set names every legal value, so the caller fixes the input
  without opening docs; a near-miss gets offered ("a topic with a similar
  name exists: \"order\"").
- Leave the fix empty only when the code cannot know it -- never guess a
  cause or remedy.

### When raising and wrapping

- Attach values only through With, as named pairs -- identifiers quoted,
  durations and sizes with units. Every key is a registered log
  attribute. Never fmt.Errorf a part the struct owns. (checked)
- Branch with errors.Is against the Err* variable, never by matching
  message text -- wording stays free to improve everywhere at once.
- A wrapping layer adds only the fact it owns (`item %d: %w`).
- An error is returned or logged, never both: the caller owns what it
  receives; a layer with no caller -- a goroutine top, a tick loop -- is
  the one place logging a failure belongs.
- A user document -- a message payload, a schedule's `Metadata` -- is never
  attached, wrapped, or formatted into an error: not as a With value, not
  inside a struct rendered with `%v`, not through a driver error that prints
  its argument.

### When writing a plain error

The errors that stay below the declaration boundary -- validation guards,
internal invariants, same-package control-flow signals.

- The problem-line templates, banned words, and tense rules above apply
  identically -- a plain error is the same fact minus the code, recovery,
  and registry. Before writing prose, check every root's `errors.go` (or
  `vulkan explain`): restating a declared condition is a bug, raise the
  Err* variable. (checked)
- A constraint guard ends with the violating value:
  `<name> must be <constraint>, got <value>` -- %d for ints, %v for
  durations (units come free), %q for strings. Absence guards
  (nil / required / empty) carry no value clause. (checked)
- `<name>` is the identifier as the caller knows it: the param name for
  constructor args, the exported field spelled exactly for config fields,
  the column or JSON key when validating stored data.
- `errors.New` for static text; `fmt.Errorf` only when a value is
  interpolated. (checked)
- A plain error may carry the same ` -- <fix>` clause, under the
  fix-writing rules above. A fix naming another package's method or a
  CLI command is the promotion tell -- the condition is user-facing, so
  declare it.
- Wrapping is the same rule as above -- only the owned fact, spelled as
  declared: `<Field>: %w` in config Validate chains, `item %d: %w` per
  element. Never restate the cause's content.

## Logging

Logs and errors are one message system with two mouths: an error speaks
when a caller receives a value; a log speaks when there is no caller.
They share the attribute vocabulary, the problem-line grammar, and a
classification question. The returned-or-logged rule is under ## Errors.

### The seam

- Every log call goes through the `logging.Logger` its owner was built
  with and passes the caller's ctx -- `context.Background()` in a log
  call is a bug outside process-shutdown paths (there,
  `context.WithoutCancel(ctx)`).
- The default logger writes text lines to stderr, WARN and up. Logs never
  share stdout with program output.
- `logging.NewPipelineLogger` is the ONE wrapper: its config declares
  what the pipeline composes -- `Buffer` (WithLogBuffer boundaries),
  `Suppress` (repeat collapse), `Args` (bound attributes) -- and building
  over an existing pipeline merges instead of nesting. `Args` CONCATENATE on
  merge, so a bound logger goes to a local, never back into the config field
  it was built from -- two clients sharing one `*ClientConfig` would
  otherwise name both schemas on every line.
- Identity is bound once: a long-lived component binds its attributes at
  construction -- `NewPipelineLogger` with `Args` -- and its call sites
  never repeat the bound keys.
- A long-lived instance (producer, consumer, system manager) declares
  `Buffer` + `Suppress` once at construction: repeats of one (level,
  message) Warn/Error line inside a one-minute window collapse to the
  first line, and the next emission carries the dropped total as
  `suppressed_count`. All the instance's workers share the window.

### Levels

Classify by one question -- who must act? -- the sibling of
Transient/Permanent (can an unchanged retry succeed?).

- Error: vulkan's own machinery stopped doing its job and no caller
  receives an error value -- a backoff curve exhausted, a worker
  suspended, a lock that could not be released. An operator must act.
- Warn: degraded but self-healing, or a durable data consequence -- a
  lease reclaimed from an expired worker, a message dead-lettered,
  stored options clamped. An operator should learn of it eventually.
- Info: lifecycle transitions and completed admin verbs only -- instance
  started/stopped, topic registered/destroyed, partition dropped, N rows
  swept. Info volume tracks state changes, never traffic.
- Debug: per-message and per-attempt narration -- claims, produces,
  batches, retries in progress. The domain working as designed
  (duplicate publish skipped, request superseded) is Debug no matter how
  dramatic it looks.

Steady state is silent: a tick that changed nothing logs nothing at any
level; a tick that changed rows logs one line with counts, never a line
per row.

### Messages

- The message is a static lowercase clause naming the event, constant
  across occurrences; every variable fact is an attribute. A value
  interpolated into a message is a bug.
- Problem-line rules apply verbatim: the banned words, tense follows the
  fact (a self-healing failure reads "could not <verb>"; a completed
  transition reads past participle -- "topic registered", "lease
  reclaimed"), consequence or next action after ` -- `. (checked)
- Nothing branches or filters on message text -- not code, not labs.
  Labs assert on log events by level and attributes through a counting
  Logger, never by matching message substrings.

### Declared events

The mirror of the error-declaration boundary: a Warn or Error event
earns a declaration (and a code) when it is operator-actionable enough
for a docs page -- a durable data consequence, a reclaim, a backstop, a
stopped mechanism. Debug/Info narration never declares, with ONE
exception: a lifecycle summary line whose attribute set needs a docs page
(the stopped line's session counters) declares despite being Info --
the code is the line's breadcrumb to its own explanation.

- Declare in the owning vocabulary package's events.go via
  diagnostic.NewDiagnosticEvent(code, message, consequence) -- the codes
  share the errors' VK serial space, next four-digit serial after the
  current max across both registries.
- Call sites log the declaration's Message() and attach `"code",
  Event.GetCode()` as the first attribute pair -- the message stays
  static, the code is the greppable pointer.
- Land the hand-written docs page (same /errors/ path) in the same
  change; `vulkan explain` lists events beside errors.
- The message follows the ### Messages grammar; the consequence clause
  is fixed at declaration, never at the call site.

### Attributes

One key per concept, flat snake_case, spelled from this table; a new
concept adds its row in the same change. A With key or a diagnose
placeholder not in this table is a bug. (checked)

| Key | Holds |
| --- | --- |
| `error` | the error value itself (never `err`, never stringified first -- .Error() defeats diagnostic.DiagnosticError.LogValue) |
| `code` | a declared log event's code (Event.GetCode()) |
| `alert` | a built-in alert's name (Alert.Name) |
| `metric_names` | original metric names omitted by an export collection |
| `alert_message` | the alert's own message clause -- never `message`, which is the log record's own field |
| `detail` | the alert's detail clause or metric-family rejection reason |
| `hint` | the alert's hint clause |
| `severity` | the alert's severity |
| `message` | a buffered record's own message, inside a `preceding` group attribute |
| `topic` | topic name |
| `topic_id` | topic id |
| `topics` | the topic names a guard names, comma-separated |
| `new_name` | a rename's target topic name |
| `declared_partition_size`, `existing_partition_size` | the PartitionSize a declaration carries against the one the topic row already holds |
| `version` | schema version (on VK0022/VK0023: the scope's current migration version) |
| `build_version` | the migration version a build defines for a scope |
| `min_compatible_version` | the strictest MinCompatibleVersion among the applied migration steps |
| `group` | consumer group name |
| `group_id` | consumer group id |
| `session` | consumer session id -- one Consume call's uuid |
| `system_id` | system id |
| `schema` | the Postgres schema vulkan's tables live in |
| `owner` | owner name (Owner.Name) |
| `owner_kind` | owner kind (Owner.Kind()) |
| `worker` | worker name |
| `worker_id` | worker row id |
| `metadata` | a worker row's stored config document; a replace logs "old -> new" |
| `payload_changed` | a schedule redeclaration changed the stored payload -- the document itself is never logged |
| `metadata_changed` | the same for a schedule's Metadata document |
| `target_instances` | the worker row's live-instance cap -- 0 is suspended, -1 is no cap |
| `message_id` | message id |
| `message_key` | message key |
| `schedule` | schedule name |
| `schedule_id` | schedule id |
| `low`, `high` | id range bounds |
| `committed` | the committed cursor id a new group's row was created at -- the registered (created) line |
| `attempt` | retry position (attempts = the row's own column; a cap spells its config field, max_retries) |
| `delay` | backoff delay |
| `rate` | worker poll rate |
| `duration` | elapsed wall time of the operation the line reports |
| `threshold` | the configured duration ceiling the line compares against |
| `vulkan_version` | module version (common.BuildVersion) -- start lines |
| `help` | plain words ending in the verbatim command that explains the line ("metrics explained: vulkan explain VK0041") -- summary lines only |
| `<verb>_count` | rows affected by the named action (swept_count, reclaimed_count, dead_count) |
| `suppressed_count` | repeats of the same Warn/Error line dropped inside the suppression window |

- Counts of affected rows end in `_count`; durations pass as
  time.Duration values (units render free); ids use their column's own
  name.
- A user document -- a message payload, a schedule's `Metadata` -- never
  reaches a log line: not as an attribute, not inside a row struct, not
  through `%v` of a struct that holds one, not through a wrapped driver
  error. A line says a document changed (`payload_changed`) and nothing of
  what it holds. A handler's own error text is stored in `last_error` and
  never logged. Keys (`message_key`, `idempotency_key`) are identifiers and
  do appear.

### The start line

A long-lived instance's "starting" line is its diagnosis snapshot: a
pasted log answers "what was your setup?" without a second question. It
carries the module version (common.BuildVersion), the instance identity,
and the resolved config facts an operator would ask for (poll rate,
timeouts, batch sizes) -- one line, attributes only. A config fact's key
spells its config field snake_cased (shutdown_timeout, batch_limit).

The paired "stopped" line is the session summary: bound identity, the
session's `duration`, and every lifetime counter the instance keeps as
`<verb>_count` attributes -- all printed, zeros included, so the line's shape
never varies. It is emitted on EVERY exit, fatal-error teardown included
(the error is still returned, never logged), and reads memory only --
never the database, which may be exactly what is down. A summary line
with counters is declared (the Declared-events exception) and carries a
trailing `help` attribute, so the line itself points at its explanation.

### The failure record

- A Warn or Error event names enough domain state to reconstruct the
  picture cold -- the ids, the range, the attempt, the durations: the
  operands, not just the verdict.
- Operations carry a debug buffer: a boundary (logging.WithLogBuffer)
  opens a small per-operation ring; Debug/Info/Warn records inside it
  are held as well as forwarded, and the operation's first Error record
  drains the ring into its `preceding` group attribute -- the failure line
  ships its own narration. Boundaries today: the produce call, the
  per-delivery dispatch, the worker tick; a new operation shape adds its
  boundary when built.

# Part 5 -- Outside the library

## Labs

- A lab that hand-copies a production query (EXPLAIN demos) goes silently
  stale when the real query changes -- grep labs for mirrors whenever a
  production query moves. Prefer driving the real datastore method.

## Playground examples

- Create each topic handle once and reuse it for registration and operations.
- Handles use domain names (`uploads`, `transcoder`); registered instances use
  activity names (`producer`, `consumer`, `scheduler`). Qualify instance names
  when several of the same kind exist (`uploadsProducer`, `usageProducer`).
- Always name the consumer handle separately before registering its instance.
- Consumer handlers are named functions below `run`, not inline closures.
- Every example starts with `LifecycleContext(nil)` and `defer stop()`.
  Simulated handler waits observe cancellation.
- Use `routines` and `routinesCtx` for errgroup orchestration.

## Documentation

Rules for the doc site (website/) and all user-facing prose.

- Docs describe the real API only: every code sample compiles against the
  shipped library. A capability that does not exist yet is marked as
  proposed, never shown as current; the process that gets it there (the
  doc page as the proposal) is in AGENTS.md.
- No performance number without a benchmark record behind it. The site
  cites bench/ records; a comparison table scores shipped behavior only --
  a proposed capability is never a checkmark.
- Docs speak the API's own nouns in their plainest form. The ## Vocabulary
  registry governs docs prose exactly as it governs code.
- The website/ tree carries its own rule file, website/CONVENTIONS.md --
  this file's sibling for frontend code. Its preamble names the sections
  here that bind there by reference; it never restates them.
- AI-drafted site prose writes against website/VOICE.md (samples, rules,
  and a revision checklist run as its own pass) -- read it before
  drafting any website/ prose, even when no file in that tree is open
  yet.
