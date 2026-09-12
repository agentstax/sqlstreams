---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# Homebrew cask publishes stable releases only

## Context

The CLI prerelease pipeline is proved. The public
allegedlyreliable/homebrew-tap repository and HOMEBREW_TAP_TOKEN secret now
exist. The previous token-only gate would let a release candidate replace
the normal sqlstreams cask.

## Decision

Set the Homebrew skip_upload template to auto when HOMEBREW_TAP_TOKEN is
set and true otherwise. GoReleaser's auto policy skips prerelease tags;
stable tags may update the normal cask. Missing credentials still skip
publication. Use the existing publisher without another workflow or cask.

GitHub prereleases continue to receive their archives. This decision sets
Homebrew policy only; Chocolatey's publication policy remains to be settled.

## Consequences

Installing the normal cask follows stable releases. Release candidates do
not change that listing. A stable release and a real brew installation
remain necessary to prove tap write access and the installed binary.

Reference: https://goreleaser.com/customization/publish/homebrew_casks/
