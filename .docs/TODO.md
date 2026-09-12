# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
.docs/decisions/.

The idle-fleet fix shipped 2026-09-12; see HISTORY.md and [0780] [0781] [0783].

## Release pipeline, dependency scanning, and package managers

Release checkpoint for v0.1.0 passed on 2026-09-12 [0789]. Fresh-DB
signal-e2e and the complete just verify suite pass at source 65efd3a1.
The compatibility driver now pins published root v0.1.0-rc.1 with
GOWORK=off and no replace. Against system/stream tables created by the
current build, GOFLAGS=-race just compat-lab round-trip passes: five
distinct payloads consumed and stream destruction verified. Missing-stream
and deliberately wrong-verdict checks fail as intended. Both registries
remain v1 with no steps. HISTORY and the migration guide record this exact
pair. The checkpoint is pushed as b5bcded6 and tagged v0.1.0;
Homebrew publication failed after the GitHub assets were uploaded (below).

Root, OTel, and CLI v0.1.0-rc.1 publication and standalone consumption are
proved [0785] [0786]. Seven-module Dependabot version updates are proved
on pushed commit 5e6ae93c [0787]: run 34718872371 succeeds and opens one
Go group, PR #10, replacing #4. Evidence is in ROADMAP.md and HISTORY.md.

- [ ] Confirm a later dependency graph refresh stores .tools' published root
  requirement. Run 34718871790 parsed all four dev manifests but received
  HTTP 500 submitting .tools' snapshot; GitHub refuses a workflow rerun.
  The graph still shows the old six-dependency .tools entry.
- [x] Review Go PR #10 at 9dc040c3 and apply its dependency changes locally.
  Its hosted CI passes; the combined local change passes just verify,
  including Docker integration tests and race tests. All seven modules
  also pass standalone go mod tidy -diff, build, and vet with GOWORK=off.
  .tests additionally pins moby/go-archive v0.3.0 and its required transitives;
  the other six modules match PR #10 exactly. No test files changed.
- [x] Prepare fixes for all 21 observed alerts: Astro 7.3.2, Sharp 0.35.4,
  compatible transitive npm fixes, and moby/go-archive v0.3.0. npm audit
  reports zero vulnerabilities; every affected website package version is
  outside the 20 reported advisory ranges. Declare @astrojs/markdown-remark
  directly because astro.config.mjs already imports its unified processor;
  the updated Astro no longer provides that package at the top level.
- [x] Website dependency verification: build, Astro/Svelte type checks,
  ESLint, remark, Vale, edited-manifest formatting, and Wrangler startup
  pass. Browser flows pass 33/45; the same 12 member-profile animation
  failures reproduce with committed source and the original npm lockfile.
  The original lockfile also reproduces all five formatting failures,
  board-banner.css:26's token lint violation, and the SQL-comment drift
  assertion (131/132 unit tests pass). No tests or unrelated source changed.
  Evidence: /private/tmp/sqlstreams-dependency-*.log and
  /private/tmp/sqlstreams-site-before-*.log. Astro's agent auto-backgrounding
  required ASTRO_PREVIEW_BACKGROUND=1 for Playwright to own its preview server.
- [ ] User commits/pushes the reviewed dependency changes; verify hosted
  checks, alert closure, PR supersession, and the .tools graph refresh.
  Grouped security updates are user-confirmed enabled. No PR was merged,
  no alert dismissed, and no site deployed during local verification.
- [x] Confirm public allegedlyreliable/homebrew-tap exists with main as its
  default branch and HOMEBREW_TAP_TOKEN exists in SQLStreams' Actions secrets.
  Token permissions remain to be proved by publication.
- [x] Prepare stable-only Homebrew publication [0788]: skip_upload renders
  auto with a token, true without one. Prereleases keep their GitHub archives.
  GoReleaser 2.18.1 check and git diff --check pass.
- [x] Diagnose v0.1.0 release run 34721726307: all six archives and checksums
  are published, but Homebrew fails while evaluating its repository token.
  GoReleaser 2.18.1's restricted credential evaluator accepts only a direct
  .Env reference; envOrDefault is available in general templates, not here.
  Replace the Homebrew token with {{ .Env.HOMEBREW_TAP_TOKEN }} and use the
  same direct-reference form for Chocolatey's API key. GoReleaser check
  passes. An isolated check using upstream ApplySingleEnvOnly rejects the
  original template and accepts both replacements with empty/dummy values;
  the existing skip gate gives true for missing/empty tokens and auto for
  a present token. Actual tap write access remains unproved.
- [ ] Commit/push the credential-template fix, then publish v0.1.1 to prove
  cask publication and brew installation/version output. Preserve the
  published v0.1.0 tag/assets; rerunning its workflow uses the broken config.
- [ ] Provision the Chocolatey account, settle its prerelease policy,
  configure its API key, and verify Windows packaging and installation.
