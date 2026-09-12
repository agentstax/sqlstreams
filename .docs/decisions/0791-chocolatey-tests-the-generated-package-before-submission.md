---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# Chocolatey tests the generated package before submission

## Context

The snapshot proposal added a local HTTP server and alternate package URLs
because it tried to test installation before any release assets existed.
The actual requirement is to test before submitting to Chocolatey. The
user approved the documented build, install/test, then push sequence.

Supersedes [0790](0790-chocolatey-publishes-stable-tags-and-checks-snapshot-installation-on-windows.md).

## Decision

Keep the tag-triggered Windows release workflow. Stable tags with a key
include Chocolatey packaging; prereleases and missing keys skip it.
GoReleaser uses chocolateys.skip_publish: true to generate the nupkg while
publishing the normal GitHub release archives and Homebrew cask.

After GoReleaser succeeds, install that version from .dist on the same
Windows runner. Its unchanged generated installer downloads the published
GitHub archive and verifies GoReleaser's checksum. Check the Chocolatey
shim's version against release metadata, then uninstall. Every native
command must succeed; any version mismatch fails the job.

Only then use choco push to submit the same .dist nupkg with the Actions
secret. Keep the publication API key in this final workflow step. Use the
normal generated download URL; remove manual dispatch, snapshot branching,
root-tag discovery, and the Python server.

## Consequences

One boolean and about 25 workflow lines implement the check with the
existing tools. No independent package format or test download route exists.
GitHub assets and Homebrew are already published if Chocolatey checks fail;
the workflow reports failure and does not submit the Chocolatey package.
Community moderation and later public-feed installation remain separate
proofs. The first hosted Windows run is still required.

References:
https://goreleaser.com/customization/package/chocolatey/
https://docs.chocolatey.org/en-us/create/create-packages-quick-start/
