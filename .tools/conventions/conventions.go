package conventions

// Package conventions enforces the machine-checkable rules of the repo-root
// CONVENTIONS.md as tests. It is developer tooling, not production code:
// nothing imports it, and it ships in the dev-only tools module so its
// dependencies never enter the library's graph. Run it via `just verify`.
//
// The import block below links every package that declares coded errors,
// log events, metrics, or alerts, so the walks see the complete registry
// through diagnostic.Errors(), Events(), Metrics(), and Alerts(). A new
// file that declares codes gets its package added here (the completeness
// test fails until it is).

import (
	_ "github.com/agentstax/sqlstreams/pkg/alert"
	_ "github.com/agentstax/sqlstreams/pkg/common"
	_ "github.com/agentstax/sqlstreams/pkg/compaction"
	_ "github.com/agentstax/sqlstreams/pkg/consume"
	_ "github.com/agentstax/sqlstreams/pkg/metric"
	_ "github.com/agentstax/sqlstreams/pkg/migrate"
	_ "github.com/agentstax/sqlstreams/pkg/produce"
	_ "github.com/agentstax/sqlstreams/pkg/schedule"
	_ "github.com/agentstax/sqlstreams/pkg/stream"
	_ "github.com/agentstax/sqlstreams/pkg/system"
	_ "github.com/agentstax/sqlstreams/pkg/worker"
)
