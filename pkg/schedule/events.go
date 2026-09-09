package schedule

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// EventMessageAlreadyProduced means a schedule producer tick found its
// message already in the stream: an earlier tick's commit confirmation was
// lost after the produce landed, so this tick produces nothing.
var EventMessageAlreadyProduced = diagnostic.NewDiagnosticEvent("SQL0037",
	"schedule message was already produced by an earlier ambiguous commit", "")

// EventTargetKeepsNoSuccessRows means the schedule's target stream keeps
// failure rows only, so ScheduleStatus can never count a success.
var EventTargetKeepsNoSuccessRows = diagnostic.NewDiagnosticEvent("SQL0058",
	"schedule target stream keeps no success rows",
	"Client.Scheduler(name).Status counts no successes for it; set DeliveryLogMode all on the stream to count them")

// EventScheduleConfigReplaced means a declaration overwrote a schedule row's
// differing config -- two declarers disagree about the schedule.
//
// Diagnose queries: sqlstreams explain SQL0062
var EventScheduleConfigReplaced = diagnostic.NewDiagnosticEvent("SQL0062",
	"schedule config replaced",
	"the newest declaration wins; if this is unexpected or repeats on every restart, two services declare this schedule with different configs and overwrite each other",

	diagnostic.NewDiagnosticQuery("the schedule row as stored now (schedule_config keeps no declaration trail)", `
SELECT
	name,
	expression,
	concurrency,
	timeout_ns,
	suspended
FROM {schema}.schedule_config
WHERE id = {schedule_id};`),
)
