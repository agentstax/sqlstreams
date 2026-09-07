# Website documentation review

The documentation has a useful structure and strong operational detail, but it needs a focused editorial pass before its onboarding and explanations are consistently dependable.

## Scope and limits

Reviewed the board order and heading structure across all 48 non-error documentation pages; read representative onboarding, concept, guide, reference, and troubleshooting pages in depth. Inspected the shared layouts and CSS. This is a high-level documentation review, not an exhaustive API, SQL, or competitor fact audit.

Browser access was unavailable, and the local Astro preview exited before becoming ready. Visual observations below are source-based; desktop/mobile appearance, contrast, keyboard behavior, and rendered code formatting remain unverified. No website files were changed.

## 1. Make the Quickstart a reproducible first run — high priority

The reader gets two complete programs but no filenames or commands for running them. The CLI uses `VULKAN_ADMIN_DATABASE_URL`, while both programs read `PGPASSWORD`, which the instructions never set. An alert-schedule configuration example appears before the reader has constructed `client`. The final transaction inserts into a `users` table that the tutorial never creates.

Worked case: a reader follows the `signup.welcome-email` instructions with the example password `secret`. The CLI receives that password, but the Go program receives an empty password unless the reader independently sets `PGPASSWORD`. After fixing that, the reader must decide where the two `package main` programs belong and how to run them.

Options: **A.** Keep the present tutorial and supply the missing setup, filenames, run commands, and expected output. **B.** Make first produce/consume the complete tutorial and link to inspection and transactional-produce follow-ups.

**Pick B:** a shorter path to a visible result, at the cost of one follow-up click. Use actual section headings for the steps; currently the only H2 is “Where next.” Keep cancellation and at-least-once handling notes beside the consumer example. Move alert policy and shutdown opt-outs to their owning reference pages.

Source: [Quickstart](website/src/content/docs/quickstart.mdx).

## 2. Correct the broad mental models before polishing prose — high priority

The Lifecycle page says state appears only when something goes wrong and its diagram shows only `ready`, `inflight`, and `dead`. The Ordered Delivery page already explains `deferred` rows for messages whose handlers never ran. Fan-out calls lag “the one health metric,” while Lifecycle explicitly says dead rows do not block the cursor.

Worked case: `apply-balance` message 101 fails; 102–104 become `deferred` without running. Those are normal ordering behavior, not handler failures. Separately, a `fraud-screening` message can be dead while the group's cursor continues toward the head: small cursor lag does not establish successful processing.

Options: **A.** Qualify the existing descriptions and add the missing states and health signals. **B.** Replace these introductions with a detailed account of every persistence path.

**Pick A:** preserve the useful fast-path explanation, with a compact state/outcome table and one worked case. Describe lag as cursor progress; show pending retries and dead deliveries alongside it. Put retention limits beside claims that messages remain queryable. The cost is a few extra rows, without another concept page.

Sources: [Lifecycle](website/src/content/docs/concepts/lifecycle.mdx), [Ordered Delivery](website/src/content/docs/guides/ordered-delivery.mdx), [Fan-out](website/src/content/docs/concepts/fan-out.mdx).

## 3. Order the reader journey by prerequisite — medium priority

The Concepts board places Architecture and Table Design before Lifecycle, Handler Outcomes, and Fan-out. A reader following “next” reaches physical tables before understanding what the delivery states represent. Getting Started places Why Vulkan and Demo before Quickstart, though Quickstart is also sticky.

Worked case: someone who has just consumed a welcome email follows the Concepts board and meets `compaction_head` and `message_key_lease` before learning the simpler relationship between a topic and independent consumer groups.

Options: **A.** Reorder existing entries. **B.** Build a second navigation or learning-path system.

**Pick A:** Getting Started → Quickstart, Why Vulkan, Demo, Roadmap. Concepts → Queue and Log, API Shape, Fan-out, Lifecycle, Handler Outcomes, Consumer Group Config, Routing, Message Key, Ordering, Architecture, Table Design, then observability and reliability. The cost is a small board-order change; no new navigation machinery is needed.

Source: [Board definitions](website/src/pages/_board/boards.ts).

## 4. Keep depth, but put it where the reader asks for it — medium priority

Several guides mix instructions with implementation rationale. Ordered Delivery moves from configuration into indexes and claim predicates; Where a New Group Starts ends in cursor-fence SQL; Transactional Produce makes readers pass a long persuasion section before the working API example. These are useful topics, but the entry sequence makes task completion slower.

Worked case: a reader choosing `ConcurrencyOrdered` needs the group declaration, message key, retry behavior, compaction restriction, and waiting cost. The index shape explains implementation rather than how to configure the guarantee.

Options: **A.** Lead guides with the action and observable result; move owned mechanisms to the corresponding concept pages. **B.** Shorten every page equally.

**Pick A:** preserve the named examples and SQL in the explanation layer. Move the transaction verb-selection table before the extended rationale. Delete change-history phrases such as “gains columns” and “a refusing form was built and reverted”; decision records already own that history. Cost: moving and relinking sections, while retaining their useful depth.

Sources: [Ordered Delivery](website/src/content/docs/guides/ordered-delivery.mdx), [New Group Start](website/src/content/docs/guides/new-group-start.mdx), [Transactional Produce](website/src/content/docs/guides/transactional-produce.mdx), [API Shape](website/src/content/docs/concepts/api-shape.mdx).

## 5. Apply the concise voice selectively — medium priority

Why Vulkan uses absolute claims such as “The dead end never comes,” “the only safe queue,” and “Operational surface area of zero.” They obscure the database capacity and upkeep costs discussed elsewhere. Reference and tuning pages are generally more direct and useful.

Worked case: a team evaluating a producer-only deployment reads “zero” operational surface, then discovers in Architecture that it needs a running manager. The product benefit is fewer systems to operate; the docs should state the remaining responsibility immediately.

Options: **A.** Replace absolute claims with the concrete benefit and its boundary. **B.** Keep the promotional voice in orientation and qualify it elsewhere.

**Pick A:** “Vulkan runs inside your Go processes against Postgres. Consumers run upkeep; a producer-only deployment also runs the manager.” Keep longer explanation for ambiguous commits, retention, timeouts, and ordering. Cost: less sales rhetoric, more trustworthy evaluation copy. Competitor comparison tables need a separate source-backed accuracy pass before their broad claims are endorsed.

Sources: [Why Vulkan](website/src/content/docs/why-vulkan.mdx), [Architecture](website/src/content/docs/concepts/architecture.mdx), [Consumer Tuning](website/src/content/docs/guides/consumer-tuning.mdx).

## 6. Use visuals to expose a relationship, and headings to support scanning — medium priority

Architecture's text diagram spans 100 characters; Lifecycle's main state diagram spans 79. The shared code style caps blocks at 680px and scrolls horizontally. This makes narrow-screen horizontal reading likely, though it could not be visually verified. Long reference tables already have local horizontal scrolling, and the author column collapses on narrow screens: retain those existing provisions.

Worked case: on a phone, the reader needs to see the cause and destination of a lifecycle transition together. A wide text diagram can put the transition label and destination on opposite sides of the scroll area.

Options: **A.** Use existing Markdown tables and numbered steps for these relationships. **B.** Add custom diagram components or interactive controls.

**Pick A:** Lifecycle gets `trigger → state → next action`; Architecture gets a short produce/claim/resolve sequence plus its existing table map; Ordering keeps its useful policy comparison and numbered-message example. Cost: a small content edit, no dependency or new component. A graphical diagram should follow only if this still leaves the mechanism hard to read.

Normalize code indentation during that pass, and replace `must()` in Replay, New Group Start, and Schema Versions with visible error handling. Check the Quickstart's nested code fences in the rendered page before approving formatting.

Sources: [Global styles](website/src/styles/global.css), [Post frame](website/src/components/post-frame/post-frame.css), [Architecture](website/src/content/docs/concepts/architecture.mdx), [Lifecycle](website/src/content/docs/concepts/lifecycle.mdx).

## What to preserve and how to close the review

Consumer Tuning's setting/measurement/cost pattern, Consumer Timeouts' concrete timing example, the reference field/default tables, contextual VK links, and explicit Proposed labels are good foundations. None of the guide/concept pages exceeded six H2s in the inventory; the main problem is purpose and sequencing, not indiscriminate length.

First repair Quickstart and the core mental-model contradictions; then reorder and relocate content; finish with voice and formatting. Verify the complete first-run instructions against the library and database, then inspect the changed pages on desktop and phone in both board styles. No new documentation infrastructure is warranted by this review.
