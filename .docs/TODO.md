# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
.docs/decisions/.

The idle-fleet fix shipped 2026-09-12; see HISTORY.md and [0780] [0781] [0783].

## Release pipeline, dependency scanning, and package managers

- [x] Publish canonical source and prove main-push CI on 9772deee.
- [x] Prove v0.1.0-rc.1 release on Windows with package-manager secrets empty;
  verify published asset digests, downloaded macOS checksum, version output,
  and build identity. Root module resolves outside the workspace.
- [x] Verify all three Dependabot version-update ecosystems; alerts and
  automatic security updates enabled; root, CLI, and OTel manifests indexed
  with parseable dependency entries. Evidence in ROADMAP.md.
- [ ] Confirm grouped security updates in GitHub settings; the available API
  does not expose a verified value for that setting.
- [x] Prepare root v0.1.0-rc.1 requirements and checksums in OTel and CLI.
  OTel passes GOWORK=off tidy/fmt/build/vet/race tests against the downloaded
  root version. Tidy corrects direct/indirect OTel SDK/metric classifications
  without upgrading dependencies.
- [x] Let Fang read the installed CLI module version when GoReleaser has not
  supplied one [0786]. CLI workspace build/vet/race tests pass; local source
  reports unknown (built from source), and the linker override reports
  0.1.0-rc.1. Installed-version reporting still needs the real CLI tag.
- [x] Publish otel/v0.1.0-rc.1 at c99bd5e8ab38c82311e36695e596f30dd3a66356.
  Public Go download resolves that tag; a fresh GOWORK=off consumer outside
  the repo builds/runs with root and OTel v0.1.0-rc.1 and no replaces.
- [x] Add OTel v0.1.0-rc.1 to CLI and remove its workspace note. Standalone
  GOWORK=off tidy/fmt/build/vet/race tests pass against downloaded root/OTel.
  Tidy records OTel's transitive requirements/checksums without upgrading
  the CLI's existing dependency versions.
- [ ] User commits/pushes cmd/sqlstreams/go.mod and go.sum; agent publishes
  cmd/sqlstreams/v0.1.0-rc.1 from that exact commit.
- [ ] Prove CLI go install and version output outside the workspace before
  changing installation docs. OTel consumer proof is complete.
- [ ] Add remotely resolvable nested/dev modules to Dependabot version updates.
- [ ] Provision allegedlyreliable/homebrew-tap and the Chocolatey account;
  settle prerelease publication policy, configure credentials, verify
  Chocolatey packaging on Windows, and prove package-manager installs.
