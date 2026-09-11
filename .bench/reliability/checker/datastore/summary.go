package datastore

import (
	"context"
	"fmt"
)

// ProduceSummary counts the records' produce outcomes.
type ProduceSummary struct {
	Attempted int64 `json:"attempted"`
	Committed int64 `json:"committed"`
	Rejected  int64 `json:"rejected"`
	Unknown   int64 `json:"unknown"`
}

// HandlerSummary counts the records' handler invocations by outcome.
type HandlerSummary struct {
	Success int64 `json:"success"`
	Error   int64 `json:"error"`
}

func (d *CheckerDatastore) ReadProduceSummary(ctx context.Context) (ProduceSummary, error) {
	if d.Config.DisableMessageRecording {
		readSql := fmt.Sprintf(`SELECT COALESCE(sum(attempted),0),COALESCE(sum(committed),0),COALESCE(sum(rejected),0),COALESCE(sum(unknown),0) FROM (SELECT DISTINCT ON (process,stream) attempted,committed,rejected,unknown FROM %s WHERE "group" = '' ORDER BY process,stream,at DESC) latest`, messageProgress)
		var summary ProduceSummary
		err := d.pool.QueryRow(ctx, readSql).Scan(&summary.Attempted, &summary.Committed, &summary.Rejected, &summary.Unknown)
		return summary, err
	}
	summarySql := fmt.Sprintf(`
		-- lab: datastore.ReadProduceSummary
		SELECT
			count(*) FILTER (WHERE kind = 'attempted'),
			count(*) FILTER (WHERE kind = 'committed'),
			count(*) FILTER (WHERE kind = 'rejected'),
			count(*) FILTER (WHERE kind = 'unknown')
		FROM %[1]s;
	`, produceRecord)
	var summary ProduceSummary
	err := d.pool.QueryRow(ctx, summarySql).Scan(&summary.Attempted, &summary.Committed, &summary.Rejected, &summary.Unknown)
	return summary, err
}

func (d *CheckerDatastore) ReadHandlerSummary(ctx context.Context) (HandlerSummary, error) {
	if d.Config.DisableMessageRecording {
		readSql := fmt.Sprintf(`SELECT COALESCE(sum(success),0),COALESCE(sum(error),0) FROM (SELECT DISTINCT ON (process,stream,"group") success,error FROM %s WHERE "group" <> '' ORDER BY process,stream,"group",at DESC) latest`, messageProgress)
		var summary HandlerSummary
		err := d.pool.QueryRow(ctx, readSql).Scan(&summary.Success, &summary.Error)
		return summary, err
	}
	summarySql := fmt.Sprintf(`
		-- lab: datastore.ReadHandlerSummary
		SELECT
			count(*) FILTER (WHERE outcome = 'success'),
			count(*) FILTER (WHERE outcome = 'error')
		FROM %[1]s;
	`, handlerRecord)
	var summary HandlerSummary
	err := d.pool.QueryRow(ctx, summarySql).Scan(&summary.Success, &summary.Error)
	return summary, err
}
