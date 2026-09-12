---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# SQLStreams module and distribution owner is allegedlyreliable

## Context

GitHub's canonical repository is allegedlyreliable/sqlstreams, but current
module paths and release destinations still name agentstax. The user chose
allegedlyreliable throughout before the first versioned publication.

## Decision

Use github.com/allegedlyreliable/sqlstreams for the root module and every
current nested module, import, convention-check path, and documentation
example. Current repository links target allegedlyreliable/sqlstreams.
GoReleaser releases target that repository; the Homebrew tap target is
allegedlyreliable/homebrew-tap, and Chocolatey metadata names the same owner.
This supersedes [0727] only on repository and module ownership.

Historical decision records and benchmark evidence retain their original
paths. The dormant compatibility harness retains its old Vulkan dependency
and imports: changing the prior build's module identity would defeat its
purpose. Its own dev-module path takes the new owner. Links to unrelated
repositories retain their verified owners.

## Consequences

The rename is mechanical and introduces no version pins or local replaces.
The reviewed source must be committed and pushed before canonical imports
resolve remotely. Root publication still precedes real root-version pins
and tags for the CLI and OTel modules. Package-manager accounts, credentials,
and installation checks remain separate release work; metadata creates none.
