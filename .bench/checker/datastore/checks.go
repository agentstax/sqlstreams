package datastore

import (
	"context"
	"fmt"
)

// exampleLimit is how many example ids or keys a check keeps columns its
// count: enough to look up, few enough to print on one line.
const exampleLimit = 5

// example labels: what a check's example values are
const (
	exampleKey       = "key"
	exampleMessageId = "message_id"
	exampleSecond    = "second"
	examplePhase     = "phase"
	exampleSample    = "sample"
	exampleError     = "error"
)

// Measurement is what every check query returns: how many rows matched and
// the first few, as text, so the report can name them -- ExampleOf says
// whether they are keys or message ids.
type Measurement struct {
	Count     int64
	ExampleOf string
	Examples  []string
}

// measure runs a check query shaped `SELECT count(*), <array of examples>`.
func (d *CheckerDatastore) measure(ctx context.Context, exampleOf string, sql string, args ...any) (Measurement, error) {
	measured := Measurement{ExampleOf: exampleOf}
	err := d.pool.QueryRow(ctx, sql, args...).Scan(&measured.Count, &measured.Examples)
	return measured, err
}

func (d *CheckerDatastore) CountErrors(ctx context.Context) (Measurement, error) {
	if d.Config.DisableMessageRecording {
		errorsSql := fmt.Sprintf(`SELECT COALESCE(sum(rejected+unknown+error),0), ARRAY[]::text[] FROM (SELECT DISTINCT ON (process,stream,"group") rejected,unknown,error FROM %s ORDER BY process,stream,"group",at DESC) latest`, messageProgress)
		return d.measure(ctx, exampleError, errorsSql)
	}
	errorsSql := fmt.Sprintf(`
        -- lab: datastore.CountErrors
        SELECT count(*), COALESCE((array_agg(detail))[1:5], ARRAY[]::text[])
        FROM (
            SELECT error AS detail FROM %[1]s WHERE kind IN ('rejected', 'unknown')
            UNION ALL
            SELECT outcome FROM %[2]s WHERE outcome <> 'success'
        ) failures;
	`, produceRecord, handlerRecord)
	return d.measure(ctx, exampleError, errorsSql)
}
