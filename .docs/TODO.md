# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
.docs/decisions/.

## Release pipeline, dependency scanning, and package managers

Root/CLI archives and Homebrew v0.1.1 are published and verified. The
fresh-DB release checkpoint, RC compatibility verdict, Dependabot coverage,
and security remediation are recorded in HISTORY [0785] [0787] [0788] [0789].

Root, OTel, and CLI v0.1.1 are published. Fresh external OTel/root use
and versioned CLI installation are verified [0786]. Module pins, READMEs,
and release documentation are current; the compatibility driver retains
its tested RC pin. Only Chocolatey remains in this release-pipeline item.

Chocolatey publication [0791] uses the normal tag-triggered Windows job.
GoReleaser generates the package with skip_publish: true and publishes
its GitHub archive. One subsequent step installs that local package against
the real release URL/checksum, verifies the shim version, uninstalls, and
pushes the same nupkg. Any failed check blocks Chocolatey submission;
GitHub assets and Homebrew are already published at that point.

- [x] User created the Chocolatey account and configured CHOCOLATEY_API_KEY;
  GitHub secret-name readback confirms the key exists (contents unread).
- [x] Prepare the stable-only gate and install/test/push sequence. Remove
  the earlier snapshot server and alternate URL. GoReleaser 2.18.1 config
  validation, actionlint 1.7.12, and git diff --check pass locally.
- [ ] User commits/pushes the workflow; publish the next stable root tag
  (for example v0.1.2). Verify Windows packaging, installation, exact
  version, uninstall, and submission. Record the actual outcome in HISTORY.
- [ ] Address first-package moderation and verify public-feed installation
  before documenting Chocolatey as available.

Dependabot version-update PRs #1 (Actions) and #11 (website) are open as of
2026-09-12; security alerts are zero. Routine version bumps are separate
from this release publication task. The prior website dependency review
found existing formatting, CSS-token, SQL-comment, and browser-flow failures;
its exact outcomes and unchanged-test evidence are recorded in HISTORY.
