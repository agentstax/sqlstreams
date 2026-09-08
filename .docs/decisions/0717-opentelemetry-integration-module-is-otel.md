---
status: accepted
date: 2026-09-08
phase: pre-v1
---

# 0717 -- The OpenTelemetry integration module is otel

## Context

The optional OpenTelemetry integration used `otelvulkan` for its directory,
module path, and package name. The repository and import path already identify
Vulkan, so callers had to repeat that name in `otelvulkan.NewMetrics` and
`otelvulkan.NewExporter`.

## Decision

The integration lives at `github.com/agentstax/vulkan/otel` and declares
`package otel`. Its instrumentation scope uses the same module path. The old
module path and package name are removed rather than kept as a second public
entry point.

## Consequences

Callers import `github.com/agentstax/vulkan/otel` and use `otel.NewMetrics` or
`otel.NewExporter`. This is a breaking import-path change accepted before v1.
The release remains three modules: root, `cmd/vulkan`, and `otel`.
