package alert

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// EventAlertConditionHolds means a Register-time pass measured the stream
// against the built-in alert conditions and one of them held. The pass is
// log-only -- Register never fails on it.
var EventAlertConditionHolds = diagnostic.NewDiagnosticEvent("SS0063",
	"alert condition holds",
	"nothing was published; the scheduled check is what publishes and resolves an alert")
