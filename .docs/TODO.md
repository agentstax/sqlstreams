# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
.docs/decisions/.

## Release pipeline, dependency scanning, and package managers

Root/CLI archives and Homebrew v0.1.1 are published and verified. The
fresh-DB release checkpoint, RC compatibility verdict, Dependabot coverage,
and security remediation are recorded in HISTORY [0785] [0787] [0788] [0789].

Stable nested module publication follows the existing root -> OTel -> CLI
order [0786]. The user authorized both tag pushes; agents never commit.

- [x] Prepare OTel, CLI, .bench, .tests, .tools, and examples with a real
  root v0.1.1 require and tidied sums. All six pass standalone tidy -diff,
  build, vet, and race tests with GOWORK=off, including Docker integration
  tests. The compatibility driver retains v0.1.0-rc.1 as its tested prior
  release; it must not advance with routine dependency updates.
- [ ] User commits/pushes this preparation; publish otel/v0.1.1 from that
  commit and verify it resolves through the public Go module path.
- [ ] Once OTel resolves remotely, pin CLI to OTel v0.1.1, tidy, and verify
  standalone. Update both READMEs' Go install commands to the stable tag
  in that publication change. User commits/pushes; publish
  cmd/sqlstreams/v0.1.1 and verify versioned go install outside the workspace.
- [ ] Verify a fresh consumer of published OTel/root v0.1.1, record both
  stable module publications in HISTORY, and remove the completed checklist.
- [ ] Provision the Chocolatey account, settle its prerelease policy,
  configure its API key, and verify Windows packaging and installation.

Dependabot version-update PRs #1 (Actions) and #11 (website) are open as of
2026-09-12; security alerts are zero. Routine version bumps are separate
from this release publication task. The prior website dependency review
found existing formatting, CSS-token, SQL-comment, and browser-flow failures;
its exact outcomes and unchanged-test evidence are recorded in HISTORY.
