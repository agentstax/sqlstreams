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

OTel v0.1.1 is published at fe29a31f and verified with an external consumer
of stable root/OTel. All six active nested modules' root pins are published
in that commit. The compatibility harness retains its tested RC pin.

- [x] Prepare CLI's OTel v0.1.1 require, tidy its sums, and pass standalone
  tidy -diff, build, vet, and race tests with GOWORK=off. Both READMEs now
  prepare the stable go install command for publication with that tag.
- [ ] User commits/pushes the CLI preparation; publish cmd/sqlstreams/v0.1.1
  from that commit. Verify versioned go install outside the workspace,
  --version, and build metadata for stable root/OTel with no replacements.
  Record the final publication in HISTORY and remove this checklist.
- [ ] Provision the Chocolatey account, settle its prerelease policy,
  configure its API key, and verify Windows packaging and installation.

Dependabot version-update PRs #1 (Actions) and #11 (website) are open as of
2026-09-12; security alerts are zero. Routine version bumps are separate
from this release publication task. The prior website dependency review
found existing formatting, CSS-token, SQL-comment, and browser-flow failures;
its exact outcomes and unchanged-test evidence are recorded in HISTORY.
