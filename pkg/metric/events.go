package metric

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

var EventMeasurementsCannotBeExported = diagnostic.NewDiagnosticEvent("SQL0104",
	"measurements cannot be exported",
	"the current collection omits the rejected metric family; healthy families still export")

// EventGoRoutineEventsDropped means abandoned/cleared routine events were
// discarded -- the queue filled between flush ticks, or their batch could
// not land.
var EventGoRoutineEventsDropped = diagnostic.NewDiagnosticEvent("SQL0052",
	"abandoned-routine events dropped",
	"abandoned-routine snapshots undercount")
