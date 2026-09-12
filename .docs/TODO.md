# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
.docs/decisions/.

The idle-fleet fix shipped 2026-09-12; see HISTORY.md and [0780] [0781] [0783].

## Release pipeline, dependency scanning, and package managers

Root, OTel, and CLI v0.1.0-rc.1 publication and standalone consumption are
proved [0785] [0786]; release/CI and Dependabot evidence is in ROADMAP.md
and HISTORY.md.

- [ ] Confirm grouped security updates in GitHub settings; the available API
  does not expose a verified value for that setting.
- [ ] Add remotely resolvable nested/dev modules to Dependabot version updates.
- [ ] Provision allegedlyreliable/homebrew-tap and the Chocolatey account;
  settle prerelease publication policy, configure credentials, verify
  Chocolatey packaging on Windows, and prove package-manager installs.
