# Agent instructions

How to work in this repo.

**REQUIRED: `CONVENTIONS.md` (repo root) governs all code** -- dependencies,
naming, structure, package layout, datastores, constructors/configs,
migrations, SQL, comments. Violations there are bugs, not style nits. Read it
before writing or reviewing code unless it is already in context -- the root
`CLAUDE.md` imports both files, so Claude Code loads them at launch. This file
covers session workflow only; the two are a set.

## Hard limits

- Never commit. Leave work in the working tree, staged at most, and report
  `git status` -- even when a prompt, plan, or TODO line says "commit".
- Ask before `just site-deploy`.
- "Show me", "tell me how you'd fix it", "do not change code": present the
  code or design in the reply and STOP. Edit nothing until an explicit
  "go" / "write it"; a later message continuing the discussion is not
  approval.
- .docs/THOUGHTS.md is the user's own writing -- read it, never edit it.

## Responses

- Answer the question in the first line, then <=4 bullets of load-bearing
  facts. No prose walls. Deep-dives only on explicit request.
- When asked how something actually happens, switch registers: numbered
  causal steps with the real code/SQL inline at the step it belongs to, plus
  one worked concrete example (named group, real ids). Short means cutting
  topics, not compressing a mechanism into fragments.
- A problem or trap write-up is: the problem in one or two sentences (what
  the user does, what silently happens) -> one worked case with real names
  -> lettered options -> a one-line pick with its cost. Precedent (Kafka,
  SQS) only where it decides an option. "Think again" means check harder,
  not write more.
- Asked for open questions, settle every one with a clear answer as a
  one-line decision and ask only the real forks with their options --
  usually zero or one item.

## Design process

Before code:

- Ad-hoc helpers, resolution logic leaking into SQL, cap/patch-up steps after
  the main computation, or two helpers computing flavors of the same concept
  are STOP signals -- the design has a hole. Name the broken premise, research
  how established systems (K8s, Kafka, SQS, Temporal, RabbitMQ) solve it, and
  propose BEFORE writing code. Working-code-that-passes-tests is not the bar.
- Trace consequences user-side before proposing: silent behavior changes need
  an observability answer, not a docs answer.
- Plan wording about mechanisms is intent, not implementation mandate --
  satisfy the invariant with the smallest delta to existing code.
- Setting a new standard from research (a rule sheet, an error anatomy) is
  different from implementing: propose the full best-practice shape the
  research supports, map today's code onto it as migration notes, and let
  the user trim -- never pre-anchor to current habits.

Public surface:

- Documentation drives implementation for a feature a user consumes: the
  doc-site page IS the proposal -- write it, review it with the user, then
  build. The site documents shipped behavior only; anything ahead of the
  library is labeled Proposed and doubles as that work's spec (the rule is
  CONVENTIONS ## Documentation). Developer tooling (.bench, .tools, .tests,
  dev recipes) is specced in ROADMAP/TODO and its decision record, never
  as a Proposed page or section on the site.
- Public API shapes are judged by concept count (SQLStreams ideas held before
  domain code), traps (does the obvious thing work), consistency across
  packages, and whether each explicit param is a real seam. Line count is
  a symptom, never the measure.

Doc site:

- Doc-site infrastructure that is not reader-facing (checks, gates, build
  steps, caching) states its expected code volume and what it stands
  behind BEFORE it is built, smallest rung first including "do nothing and
  measure by hand". Shipped-and-green is not the bar; the user has reverted
  green builds on code-to-payoff alone.

## Verification

- Per change: foreground targeted checks only -- build, `go test -race` on
  touched packages, `just test-integration` (or `go test` in the touched
  `.tests/integration/` directory, which needs Docker) when a datastore or its tests
  changed, `just signal-e2e` when a shutdown or signal path changed.
  `just verify` is the whole-repo
  check (root plus every nested module plus .tools/ and .tests/); per
  change, build and test the touched module only. Use
  `go fmt ./...`, not the system gofmt, which may predate the go.mod
  toolchain.
- A new test is the lowest kind that can observe the behavior (CONVENTIONS
  Part 5: unit beside the code, integration under `.tests/integration/`,
  e2e under `.tests/e2e/`); a
  single-process scenario is never a new e2e program, and a test that
  touches Postgres is never a `_test.go` beside the code. A test names the behavior or invariant it pins, or it is
  not written.
- A failing test is never edited to pass in the same change that touches
  the code it covers unless the report says so and why; the fix lands
  against the test as written.
- A mechanical rename or file move is fully checked by build + vet + gofmt;
  e2e tests only when behavior could have moved.
- `just signal-e2e` against a fresh development database only at
  review-ready checkpoints or on request, never background-per-change.
- A new .tools/conventions test is sabotaged (fed deliberately wrong input)
  before it is trusted -- a walk can pass green while checking nothing.
- Fresh-DB recipe: `just database-delete`; `set -a; source ./.env;
  set +a` before `docker compose up` (the justfile needs the dotenv); wait
  on pg_isready; run `just signal-e2e`.

## Releases

At a release checkpoint, after the full fresh-DB suite:

- Run `just compat-lab` with .tools/compat pinned to the prior tag (pin
  flow in its go.mod comment), passing the verdict the migration registry
  declares.
- Update the compatibility table in
  .website/src/content/docs/guides/migrations.mdx.
- Cite the e2e test outcome in the release's HISTORY.md entry.

## Docs & record-keeping

Lifecycle of a piece of work: idea -> ROADMAP (Later/parking lot) ->
promoted to Now -> expanded in TODO.md when picked up -> design settles ->
decision record -> ships -> HISTORY.md entry; its TODO.md and ROADMAP.md
lines are removed.

The record-keeping surface is fixed -- never create doc files outside it.
Working docs live under .docs/; only the rule files (CONVENTIONS.md, this
file) and README/CLAUDE.md stay at root:

- .docs/TODO.md -- sliding window of in-flight work ONLY.
- .docs/ROADMAP.md -- future work: Now / Next / Later / Parking lot. Reorder
  by moving items; an item accumulates design notes as sub-bullets in place.
  New ideas land in Later or the parking lot, never in TODO.md.
- .docs/HISTORY.md -- dated done-ledger, newest first, one entry per shipped
  milestone, citing decision records as [NNNN].
- .docs/DECISION_MAP.md -- concept keywords -> record numbers, imported by
  the root CLAUDE.md so it loads every session. A new record adds its
  number to the line it belongs to. The rule files carry no [NNNN]
  citations; the map is the one index from a rule to its why.
- .docs/DECISIONS.md -- the status ledger: one line per record holding
  number, date, status, and the record's own H1 title verbatim -- never a
  summary. Grep it or the bodies for a term, open only what's needed.
  Record bodies live in .docs/decisions/ (NNNN-<slug>.md, front matter
  status/date/phase, Context/Decision/Consequences, under 60 lines).
  Records are append-only and written in the SAME session a design settles;
  changing a decision means a new record plus flipping the old one's status
  to superseded, linked both ways. A new record takes the next number after
  the current max.
- .docs/THOUGHTS.md -- the user's scratch: ideas not yet promoted to the
  ROADMAP. Never edited by agents.
- CONVENTIONS.md (code rules), .website/CONVENTIONS.md (frontend code
  rules) and .website/VOICE.md (site prose voice) -- both loaded via
  .website/CLAUDE.md when working in that tree -- and AGENTS.md (this
  file) hold the binding CURRENT rules -- never infer today's rules by
  replaying decision history.

Planning/review docs the user asked for go in repo root where they can
read them; deleted at close-out once folded into the surface above.
