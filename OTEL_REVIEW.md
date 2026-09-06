# otelvulkan review

Reviewed 2026-09-06 against the current working tree and CONVENTIONS.md.
Review only; recommendations below are not settled design or shipped behavior.

The module serves a useful purpose, but its collection lifecycle and failure
semantics need correction before cosmetic cleanup. Keep the OTel adapter and
Prometheus integration together for now; delegate Vulkan measurement access
to the existing core path.

## Purpose and ownership

| Functionality | Purpose | Assessment |
| --- | --- | --- |
| Measurement-to-OTel instruments and attributes | Feed a caller's existing telemetry pipeline | Keep here. Gauges and cumulative counters have appropriate observable instrument types. |
| Built-in descriptions from the diagnostic registry | Preserve the same meaning in exported HELP text | Keep here. Reuses the declaration instead of inventing another catalog. |
| Prometheus provider, registry, handler, shutdown | Give users an endpoint without configuring their own OTel provider; used by `manager run` | Keep as a convenience over the same adapter. There is no demonstrated need for another package. |
| Latest retained measurement lookup and system-topic resolution | Find the measurements to export | Delegate to core. `MessageAdmin.ListMeasurements` and `System().Metrics().Latest` already own this operation. |
| Discovery and callback registration | Support custom metric names as well as built-ins | Necessary internally, but the current lifecycle is incomplete and unsafe under concurrency. |
| Collection, retention, history, custom production and consumption | Maintain and use Vulkan's stored observations | Correctly outside this module today. Keep in core; these features do not require OTel. |

The separate Go module remains justified: importing Vulkan should not require
the OTel SDK and Prometheus dependencies. A pool-taking constructor is also
consistent with the current entry-point convention; it is not itself a defect.
This integration exports metrics only. There is no tracing or logging bridge
here, and no evidence from this review that either should be added.

## Findings, in priority order

### 1. High: registration and observation can deadlock

`otelvulkan/metrics.go:101` holds `m.mutex` across instrument creation,
unregistration, and callback registration. `observe`, at line 176, takes that
same mutex. In the pinned OTel SDK v1.44.0, `pipeline.produce` holds its own
mutex while invoking callbacks; instrument creation and unregistration need
that SDK mutex too (`pipeline.go:93`, `118`, `140`).

Concurrent discovery and collection can therefore leave registration holding
the Vulkan mutex while waiting for OTel, and collection holding the OTel mutex
while waiting for Vulkan. This affects an application's periodic reader and
overlapping Prometheus scrapes when a new name appears. Context timeouts do
not interrupt either mutex wait.

This is established by the actual lock paths, not reproduced against a live
database in this review. Required invariant: registration must never enter
OTel while holding a lock its callback acquires.

### 2. High: unsuccessful collection can look like a successful scrape

`otelvulkan/exporter.go:80` logs registration errors and serves the handler
anyway. `metrics.go:182` returns observation read errors to OTel. The pinned
Prometheus exporter sends those callback errors to `otel.Handle` and continues
rendering, rather than necessarily failing the HTTP request.

A standalone probe against the pinned SDK/exporter reproduced HTTP 200 with
the source metric absent and one call to the global OTel error handler. Thus
Prometheus can report the endpoint as up while Vulkan observations disappear.
A missing system before initial registration has the same misleading-success
problem through the handler's explicit fallback.

The error also bypasses the integration's configured Logger on the observation
path. Define one explicit failure contract before changing this: either failed
scrapes, or a declared exporter-health signal if partial results are useful.
Logs alone do not give dashboards and alerts a reliable collection verdict.
This follows [Prometheus exporter guidance](https://prometheus.io/docs/instrumenting/writing_exporters/#failed-scrapes).

### 3. High: one unexportable custom name can prevent other names from appearing

`NewMeasurement` accepts any nonempty name (`pkg/metrics/measurement.go:94`);
OTel has a narrower instrument-name grammar. The pinned SDK rejects `bad name`.
`RegisterMetricInstruments` returns immediately on that failure at line 120,
before installing the callback for instruments it already added to its map.

With a first discovery ordered as `alpha`, `bad name`, `zeta`, `alpha` enters
the map, the pass stops at `bad name`, and no callback is installed. Subsequent
passes skip `alpha` and fail at the same name. Neither valid measurement is
exported. Removing the offending head still does not register `alpha` by
itself: `created` is false and the method returns at line 126.

Callback-registration failures have the same premature state-commit problem;
after an old callback is unregistered, a replacement failure can leave the
integration without observations and without a retry unless another name appears.

Required invariants: discovered instruments are not equivalent to successfully
registered observations, registration failures are retryable, and an
unexportable custom series has an explicit, observable disposition. Whether
OTel's restrictions belong in the general measurement API is a design choice,
not a reason to silently narrow core's accepted input.

### 4. Medium: the caller-owned meter has no corresponding detach lifecycle

`Metrics` retains a `metric.Registration` but exposes no close/unregister
operation. The caller cannot detach this adapter while retaining its provider.
Replacing an adapter leaves its callback and database access attached; closing
the pool first leaves callbacks querying a closed pool. `Exporter.Close`
can stop its private provider, which does not solve the caller-owned case.

Discovery also requires callers to schedule `RegisterMetricInstruments`
themselves. Calling it once on an empty system exports nothing when names
appear later; only the Prometheus wrapper automates discovery. This behavior
is documented in code, but is an incomplete convenience contract for the
advertised caller-owned pipeline. Decide the start/discover/stop contract
together. OTel explicitly provides callback unregistration for this lifetime:
[Metrics API](https://opentelemetry.io/docs/specs/otel/metrics/api/#asynchronous-gauge-operations).

### 5. Medium: retained measurements lose their freshness information

`observe` exports `Name`, `Value`, and `Attributes`, using kind/unit from the
instrument, but discards `Measurement.At` (`metrics.go:187`). A collector that
stops updating a gauge leaves the last retained value exported on every scrape.
The HTTP read is current; the underlying observation may be old. Core's
`Latest` API deliberately preserves `At` for this distinction.

This is a missing observability contract, not a recommendation to add an
arbitrary expiry cutoff. Decide how users detect a stopped collector or old
source observation before changing which measurements appear. A timestamp
metric is an established option; the appropriate granularity and semantics
need a proposal. Session counters intentionally skip unchanged flushes, so
measurement age alone is not proof that their producer has stopped.
See [Prometheus timestamp guidance](https://prometheus.io/docs/practices/instrumentation/#timestamps-not-time-since).

### 6. Medium: metric identity assumes consistent metadata without checking it

The map at `metrics.go:116` identifies instruments solely by name. Once the
first series creates an instrument, later measurements with that name cannot
change or validate its kind and unit. Two custom series sharing a name but
using seconds and milliseconds get exported with the first series' unit and
unconverted values; a later gauge/counter mismatch is likewise silently lost.

The name-to-kind/unit contract needs an owner and observable conflict behavior.
Built-ins already have that owner in the diagnostic declaration; custom
measurements currently do not. Do not solve this by silently converting or
renaming conflicting data in the exporter.

### 7. Medium: domain lookup is duplicated, including a permanent identity cache

`metrics.go:201` resolves the reserved topic itself and caches its id forever.
`pkg/admin/metrics.go:39` already resolves that topic and reads the same heads;
`pkg/vulkan/system_metrics.go:48` exposes the bare measurements publicly.
The integration reuses the compaction controller, so this is duplicated
orchestration rather than duplicate SQL.

The cache also leaves a surviving exporter addressing the old topic after an
installation is destroyed and registered again. Delegate through the existing
core operation; do not introduce another source abstraction or query solely
for this integration. The precise constructor composition belongs in the
proposal, given the repo's pool-taking constructor rule.

### 8. Medium/low: config ownership and housekeeping lag current conventions

Both constructors retain the caller's config pointer (`metrics.go:83`,
`exporter.go:65`). `Metrics` rereads CollectTimeout during operation, and Retry
is shared through the datastore. Caller edits can change behavior or race with
collection. `NewClient` already copies Retry to prevent this. Capture resolved
configuration and copy mutable configuration dependencies consistently.

Additional concrete cleanup: `Metrics.Logger` only forwards a value to Exporter;
`Exporter.Config` has no internal reader; schema defaults are repeated despite
the datastore owning resolution; `toAttributes` lacks the required helper
banner; the package comment is above rather than below the package clause;
both registration comments promise ErrTopicNotFound although the implementation
returns `migrate.ErrNotRegistered`. These are smaller than the lifecycle defects.

## Verification and next step

`go test -race -v ./otelvulkan/...` passed the constructor guards. The sole
database integration test skipped because VULKAN_OTEL_TEST_DATABASE_URL was
unset. That test covers one gauge's latest value and pool ownership; it does
not exercise concurrent discovery, failure recovery, counters, metadata
conflicts, freshness, or detach. `just verify` builds/vets this module but does
not run its tests.

The isolated upstream probe confirmed the HTTP-success-on-callback-error
behavior and rejection of a core-accepted name. No production files or existing
user changes were edited; no database suite was run.

Next, write the doc-site proposal around the retained-measurement contract,
collection failure visibility, and start/discover/stop behavior. Review those
choices before implementation, as AGENTS.md requires for public-surface work.
The package split does not need to change to address the findings above.
