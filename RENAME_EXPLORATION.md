# Project rename and topic → stream: exploration

2026-09-09. Initial inventory and recommendations, retained as exploration.
Subsequent decision [0725](.docs/decisions/0725-sqlstreams-is-the-project-name.md)
locks in SQLStreams and `topic` → `stream`; naming caveats below describe
the initial exploration, not an open shortlist. `NEW` remains a placeholder
where the exact technical identity or origin is still being settled.

The rename crosses source compatibility, stored data, operator interfaces, distribution, and visual identity. Most edits are mechanical; the deployment boundary and the meaning of “stream” need to be explicit first.

## What the repository actually contains

- Eight Go modules: root, `cmd/vulkan`, `otel`, `.e2e`, `.examples`, `.bench`, `.tools`, and `.tools/compat`. All need review; the compatibility module intentionally runs outside the workspace and can pin the old library.
- A tracked-text inventory found `vulkan` in 392 files under `pkg`, 43 under `cmd`, 8 under `otel`, 185 under `.website`, and 148 across `.tools`, `.e2e`, `.examples`, `.bench`, and `.github`. `topic` appears in 268 `pkg` files, 34 `cmd` files, and 166 site files. Counts are case-insensitive file matches, overlap, and include tests/comments; they measure search scope, not required edits. Historical docs are additional matches, not a replacement target.
- The roadmap's diagnostic pointers are stale: `docsBaseURL` and `isVKCode` now live in [pkg/common/diagnostic/registry.go](pkg/common/diagnostic/registry.go). Codes cover errors, events, metrics, and alerts in one serial space.
- [README.md](README.md) already displays SQLStreams artwork. Four `sqlstreams-*.svg` assets and a semicolon favicon live in `.website/public`; the site still contains Vulkan branding. This is evidence of an existing visual direction, not confirmation that SQLStreams is the selected name. No tracked logo-sheet file was found by filename.

## Blast radius

| Surface | Changes to inventory together | Main consequence |
| --- | --- | --- |
| Go identity | `github.com/agentstax/vulkan`, all eight module declarations and internal dependencies, imports, `pkg/vulkan` directory/package, `cmd/vulkan` directory, generated local `go.work`, tooling path constants | Downstream imports and install commands change. Changing only imports leaves module/version detection and source scanners stale. |
| Public API | `Client.Topic`, `Topics`, `TopicHandle`, `Topic`, `TopicConfig`, `TopicVersionHealth`, `TopicId`, `TopicName`, `ErrTopic*`, owner constructors, aliases, metric/alert declarations, table-name functions exposed through aliases | Every reachable declaration must use the same vocabulary. Rename at its owning declaration, then update the public alias surface. Keep the current package architecture. |
| Domain implementation | `pkg/topic` and its controller/datastore/janitor/migrations, references from admin, produce, consume, schedule, metrics, alerts, workers and system manager | This reaches almost every operational stack, even where the package name has no `topic` in it. |
| Database structure | `topic_config`, `topic_config_log`, `topic_id` columns, indexes, foreign-key references, row tags, SQL joins, diagnostic SQL, baseline DDL, migration scope names | Source changes alone cannot make an existing database conform. System and stream migrations must agree on the new catalog. |
| Stored identities | `OwnerTopic = "topic"`, `MetricScopeTopic = "topic"`, worker name `topic_janitor`, reserved-resource constant names, stored metric names/attributes and keys | These strings participate in identity and lookup, not just rendering. Old rows cannot be presumed readable by renamed code. |
| Defaults and connection configuration | `DefaultSchema = "vulkan"`, CLI environment variables, dev/test DSNs, database/container/volume names | A default-schema change selects a different installation. Never rename an operator's explicit schema or connection string as a branding operation. |
| CLI and scripts | Binary/root command, `topic` command, `--topic` flags, help/fix commands, JSON keys, completion output, examples, `.bin/vulkan`, Justfile recipes | Shell scripts, jq expressions, saved commands and operator instructions break together. |
| Diagnostics and observability | `VK` declarations/validator, `topic`/`topic_id`/`topics` attributes, `vulkan_version`, `vulkan.*` metric names, units containing `topic`, Prometheus reserved-name validation, OTel meter scope | Dashboards and alert rules can go empty while the application continues working. Codes and URLs must stay traceable. |
| Release/distribution | `.goreleaser.yaml`, GitHub workflows, release repository, archive/binary/package names, Homebrew cask, Chocolatey metadata, nested-module tag paths | Updating the repository does not complete package-manager or release identity changes. External publication status needs checking. |
| Site and discovery | `.website/src/site.ts`, titles/navigation, `why-vulkan` and topic-related slugs, error-code URLs, version manifest, canonical URLs, sitemap, robots, repository links, Cloudflare project in Justfile | Old links and old binaries' diagnostic URLs need a deliberate destination. Frozen version sites also fetch the live version manifest. |
| Site functionality | PGlite sandbox SQL and schemas, examples, diagnostic data/types, code fixtures, Shiki theme names, browser-storage keys | The site is executable documentation. Updating prose while leaving its SQL or data vocabulary old produces a split interface. |
| Brand assets | README wordmark, site header, favicon, SVG text/path artwork, introductory illustrations, accessible names, share/repository images where used | A text search cannot establish that all visible old branding is gone. Asset review is separate work. |
| Rules and records | Current `CONVENTIONS.md` vocabulary, frontend rules/voice references as applicable, convention tests, live roadmap/TODO, migration guide and eventual history entry | The current rule explicitly bans “stream” as a synonym. Update that rule as part of the accepted terminology change. Preserve historical records and the user's writing. |

Key implementation anchors: [public handle](pkg/vulkan/topic.go), [owner identity](pkg/common/owner.go), [catalog DDL](pkg/system/controller/datastore/tables.go), [table names](pkg/topic/tables.go), [CLI connection settings](cmd/vulkan/internal/cli/conn.go), [default schema](pkg/datastore/datastore_config.go), [module version lookup](pkg/common/version.go), [OTel validation](otel/validation.go), [site identity](.website/src/site.ts), [release configuration](.goreleaser.yaml).

## The changes that are easy to underestimate

**Database cutover.** A caller upgrades and leaves `Schema` unset. If the new default replaces `vulkan`, their client selects a different namespace; depending on the operation, they see an unregistered installation or initialize separate state. Their original messages have not moved.

Worked example: an `orders` resource and its `billing` consumer currently live in schema `vulkan`. New code defaulting to `NEW` does not select those rows. Explicitly passing `Schema: "vulkan"` preserves namespace selection, but still does not convert `topic_config` into `stream_config` or its columns. These are two separate changes.

A. Use the repository's existing pre-v1 baseline policy: change DDL in place and recreate disposable development databases after stopping old processes.

B. Preserve a populated installation: separately design and verify a data-preserving cutover, including catalog names, stored identities, metrics, locks and compatibility behavior.

Recommendation: A for this task under the current pre-v1 convention; its cost is losing disposable database contents. If any populated installation must survive, B is a prerequisite, not a follow-up patch. Exploration does not authorize deleting any database.

**Lock identity.** [pkg/common/lock.go](pkg/common/lock.go) uses `0x56554C4B` (VULK) as the advisory-lock namespace. Topic registration also hashes the literal `"topic"`. Renaming either changes lock keys; old and new binaries can stop contending for the same logical operation. Treat these as coordination protocol values. Recommendation: preserve the numeric namespace; change the resource literal only within the chosen incompatible cutover. Do not change the number for visual consistency.

**Metric identity.** `vulkan.topic.state.partitions` becomes a new metric name, and `topic="orders"` becomes a new attribute. [MeasurementKey](pkg/metric/measurement.go) incorporates metric names and attributes, so stored measurements also change identity. Update built-ins, reserved prefixes, OTel translation validation, alert readers, examples and dashboard queries together. Verify actual exported series and an active alert after cutover; a compiled exporter is insufficient evidence.

**Diagnostic continuity.** Preserve every four-digit serial while changing the prefix. Include events, metrics and alerts, not just `Err*` declarations. Update registry validation, test-only codes, CLI explain parsing/rendering, fix placeholders, exported site data, pages and links. Decide the prefix independently of the logo abbreviation; keeping two uppercase letters retains the current format. Keep old code URLs resolvable if already published so an old log remains useful.

**Checks that can stop checking.** `.tools/conventions` hard-codes the public entry package, declaring roots, `pkg/topic/tables.go`, CLI tree and `-- vulkan:` SQL markers. The site's SQL parity tests also recognize owner markers. Update the discovery paths with the code they inspect, and feed deliberately wrong input to affected discovery checks before trusting green results.

**External state.** Cloudflare deployment identity, domains, GitHub repository settings, package-manager listings and existing releases were not queried. Inventory configuration here is not proof that those names are published, available or transferable. Local ignored configuration also needs a name-only review during implementation; do not dump environment secrets into the inventory. Browser preferences/read tracking may reset on an origin or storage-key change; that does not justify a migration subsystem by itself.

## Vocabulary recommendation

Use the product name as the brand and **stream** as the resource noun. Avoid building a second metaphor vocabulary around the eventual logo.

| Today | Proposed decision | Reason |
| --- | --- | --- |
| Topic / topics | Stream / streams, everywhere this resource is named | One noun across API, CLI, SQL, JSON, logs and docs. Includes `StreamConfig`, `StreamHandle`, `StreamId`, `StreamName`, `OwnerStream`, `MetricScopeStream`, `ErrStream*`, `stream_janitor`, and `stream_config[_log]`. |
| Message | Keep | It names the stored/handled unit without implying event sourcing or changing handler semantics. A stream contains messages. |
| Produce / producer; consume / consumer | Keep | They already describe writing and processing a stream. Publish/subscribe or append/read would introduce another simultaneous API redesign. |
| Consumer group / consumer instance | Keep | A durable shared consumption identity and a running process remain different concepts. Calling either a “subscription” hides that distinction. |
| Binding / routing key | Keep | Bindings remain routing rules within a stream; they do not become streams or subscriptions. |
| Message log | Keep | It names the physical append history. `stream_log` would compete with stream configuration history and blur resource versus payload storage. |
| Partition | Keep; clarify as a storage partition in prose | `stream` must not imply a new delivery-ordering or partition-assignment guarantee. No algorithm change is part of the noun rename. |
| Message id / cursor / lease | Keep | Each names an existing mechanism; “offset” or “checkpoint” would imply a new position model. |
| System / schema | Keep | The installed system and its Postgres namespace are already distinct. “Cluster” would invent a topology concept. |
| Schedule / scheduler; maintenance worker / janitor | Keep | Their responsibilities do not change with stream terminology. Rename only topic-qualified names. |
| `MetricTopicName`, `AlertTopicName`, `ScheduleTopicName`, `SystemTopicPrefix` | Rename to their `Stream` forms; keep `__system.metrics`, `__system.alerts`, `__system.schedules` | The identifiers contain the old noun; the persisted resource names already do not. |
| `message_log_42`, `claim_lease_42`, `binding_config_42`, other per-resource family names | Keep their strings; rename the owning package and parameter vocabulary | The numeric suffix already identifies the resource without saying topic. Renaming these tables buys no consistency. |

One adjacent inconsistency worth a separate decision: consumer-group identity is spelled `group`, `group_id`, and `consumer_group` across current JSON/log/read models. `Binding.ConsumerGroupName` uses `consumer_group`, while `Owner.ConsumerGroupId` uses `group_id`. Recommendation: leave this outside the brand rename; if wanted before v1, settle one wire/log vocabulary as its own small proposal rather than allowing opportunistic edits.

The resulting public sentence is: “Create a stream, produce messages, and consume them with a consumer group.” The final docs must spell out existing retention, replay, concurrency and ordering behavior beneath that sentence. The word “stream” alone supplies none of those guarantees.

## Logo sheet and name selection

The existing SQLStreams wordmarks should be reference material until the name is confirmed. First settle a compact identity table: display spelling/capitalization, repository/module slug, Go package, CLI binary, environment prefix, diagnostic prefix, metric prefix, default schema and canonical docs origin. Check candidate availability and discoverability before final artwork; that external research has not been done here.

The logo sheet should be a reviewable comparison of:

1. Primary wordmark, horizontal mark-plus-name, standalone symbol and small-size favicon.
2. Light/dark, single-color and reversed versions, with named colors and typography.
3. Actual placements: README at its current 56px image height, site header at desktop/mobile widths, 16/32px favicon, repository avatar and social preview.
4. Minimum size, clear space, accessible labels, source/licensing information, and the chosen direction alongside the rejected candidates.

Deliver the selected identity as editable vector masters plus required PNG exports, and a complete logo sheet. Existing SVG lettering is outlined into paths, so replacing an SVG label does not change the visible wordmark. A logo refresh does not require redesigning the site's board layout or syntax colors.

## Proposed sequence and completion evidence

1. **Settle identity and cutover scope.** Confirm the name and spelling family, final diagnostic prefix, and whether any existing data must survive. `topic` → `stream` is already the requested direction.
2. **Review the public proposal.** Draft the doc-site proposal labeled Proposed with the resulting API/CLI vocabulary and deployment implications; review it before implementing. Review the logo sheet alongside it. Record decisions only when they settle.
3. **Implement as separable review chunks.** Stream terminology through source/storage/observability; brand/module/CLI/distribution; diagnostics/URLs; site/assets. Coordinate them into one coherent cutover rather than releasing an intermediate mixed vocabulary. Add no compatibility aliases by default before v1.
4. **Verify the changed contracts.** Build/vet and `go fmt ./...` in each affected module, targeted race tests, alias/convention checks, CLI help/JSON/explain, site build and sandbox parity. Check a consuming module and module-version reporting without relying solely on the local workspace. Reuse the existing code export and checks; no new rename infrastructure is warranted.
5. **At the review-ready checkpoint**, run the fresh-DB e2e recipe, verify exported metrics and alert evaluation, inspect logos in context, and check code-page/old-route handling. At a release checkpoint also run the pinned compatibility lab against the declared verdict, update the migration table and cite e2e outcomes in HISTORY. Do not rewrite the old side of the compatibility harness into the new API and accidentally stop testing the boundary.
6. **Coordinate external publication last.** Update repository/domain/package distribution as applicable, with concrete local artifacts ready for review. `just site-deploy` still requires approval. Close out this root exploration document by folding accepted work into the fixed record-keeping surface.

## Scope of this exploration

Read-only repository inspection plus this report; no production code, rules, live planning files, database, deployment or external account changed. No builds or tests were needed for the inventory. Existing working-tree edits were present before inspection and remain untouched. `.docs/archive/explain-it-back.md` is absent in this checkout and could not be consulted.
