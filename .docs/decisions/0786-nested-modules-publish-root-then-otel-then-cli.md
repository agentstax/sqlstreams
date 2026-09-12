---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# Nested modules publish root, then OTel, then CLI

## Context

The root v0.1.0-rc.1 and CLI binary archives are published. The nested
modules still rely on go.work, and the CLI imports both the root and OTel.
The user authorized publishing both nested modules and proving standalone
installation. The repository rule still prohibits agents from committing.

## Decision

Pin OTel to the existing root v0.1.0-rc.1, tidy and verify with GOWORK=off,
then publish otel/v0.1.0-rc.1 from the user's committed preparation. Only
after that version resolves remotely, pin the CLI to root and OTel
v0.1.0-rc.1, tidy and verify standalone, then publish
cmd/sqlstreams/v0.1.0-rc.1 from its prepared commit. Root tags remain fixed.
No published nested module carries a local replace or a placeholder pin.

The CLI leaves its linker-injected version empty by default, allowing
Fang's existing build-info mechanism to read the installed module version.
GoReleaser still supplies the release version. Local source builds without
an installed module version use Fang's unknown-built-from-source output.

## Consequences

OTel publication is a prerequisite for CLI standalone resolution. Each
module is checked against downloaded dependencies without the workspace;
the final proof is a CLI go install and an OTel consumer outside this repo.
Module-version reporting from a real installed CLI remains part of that
final proof. Agents prepare and verify files, then wait for the user's
commit before tagging the source. Package managers remain separate work.
