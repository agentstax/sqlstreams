# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
.docs/decisions/.

The idle-fleet fix shipped 2026-09-12; see HISTORY.md and [0780] [0781] [0783].

## Release pipeline, dependency scanning, and package managers

Root, OTel, and CLI v0.1.0-rc.1 publication and standalone consumption are
proved [0785] [0786]; release/CI and Dependabot evidence is in ROADMAP.md
and HISTORY.md.

- [x] User confirmed grouped security updates are enabled on 2026-09-12.
- [x] Prepare seven-module Dependabot coverage [0787]. .bench, .tests,
  .tools, and examples pin root v0.1.0-rc.1 and pass standalone
  tidy/fmt/build/vet; .bench and .tools race tests pass. go.work still
  resolves development source locally. YAML checks preserve the existing
  monthly schedule, seven-day cooldown, Go group, and Actions/npm entries.
- [x] Standalone .tests integration suite passes against disposable Docker
  Postgres with GOWORK=off and -race -count=1; no tests were changed.
- [ ] User commits/pushes the prepared module and Dependabot files; trigger a
  Go update check and verify seven-directory coverage, no resolution errors,
  and zero or one grouped version-update PR.
- [ ] Provision allegedlyreliable/homebrew-tap and the Chocolatey account;
  settle prerelease publication policy, configure credentials, verify
  Chocolatey packaging on Windows, and prove package-manager installs.
