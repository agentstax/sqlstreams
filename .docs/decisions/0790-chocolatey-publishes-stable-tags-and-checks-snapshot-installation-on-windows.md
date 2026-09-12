---
status: superseded
date: 2026-09-12
phase: pre-v1
---

# Chocolatey publishes stable tags and checks snapshot installation on Windows

## Context

The user configured the Chocolatey account and Actions API key. GoReleaser
packaging has not yet run because releases skipped Chocolatey without the
key. The first community package needs a real installation check before
submission; test packages must not enter moderation.

## Decision

Keep one release workflow. Pushes skip Chocolatey when the key is absent
or the root tag has a prerelease suffix. Stable tags with a key use the
existing GoReleaser publisher. Homebrew retains its stable-only policy.

Add workflow_dispatch for a Windows snapshot check. It runs GoReleaser
with --snapshot --skip=publish and receives no package-manager secrets.
Select the latest reachable root v* tag explicitly so nested-module tags
cannot determine the snapshot version.

The snapshot URL template downloads from a loopback Python HTTP server
serving the run's .dist archives. The generated package keeps GoReleaser's
actual archive checksum. Published packages use the matching GitHub Release
URL. This tests fresh bytes without assuming a rebuilt ZIP matches an
older release or changing the generated install script after packaging.

Install the generated package from .dist, check the Chocolatey shim's
version against GoReleaser metadata, then uninstall. Any native command
failure or version mismatch fails the job. Use the existing Windows x64
runner and GoReleaser's supported amd64 Chocolatey archive selection.

## Consequences

About 30 net workflow lines plus one URL template add an on-demand check;
no separate workflow, permanent server, or package implementation is added.
The manual run proves packaging/install/uninstall, not community upload or
moderation. Publish the next stable tag only after the Windows check passes;
then verify submission, moderator feedback, and public-feed installation.

Reference: https://goreleaser.com/customization/package/chocolatey/

Superseded by [0791](0791-chocolatey-tests-the-generated-package-before-submission.md):
test against published release archives before submitting to Chocolatey.
