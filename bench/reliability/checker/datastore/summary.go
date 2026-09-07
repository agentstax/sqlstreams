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

func (d *CheckerDatastore) ProduceSummary(ctx context.Context) (ProduceSummary, error) {
	summarySql := fmt.Sprintf(`
		-- lab: datastore.ProduceSummary
		SELECT
			count(*) FILTER (WHERE kind = 'attempted'),
			count(*) FILTER (WHERE kind = 'committed'),
			count(*) FILTER (WHERE kind = 'rejected'),
			count(*) FILTER (WHERE kind = 'unknown')
		FROM %[1]s;
	`, produceLedger)
	var summary ProduceSummary
	err := d.pool.QueryRow(ctx, summarySql).Scan(&summary.Attempted, &summary.Committed, &summary.Rejected, &summary.Unknown)
	return summary, err
}

func (d *CheckerDatastore) HandlerSummary(ctx context.Context) (HandlerSummary, error) {
	summarySql := fmt.Sprintf(`
		-- lab: datastore.HandlerSummary
		SELECT
			count(*) FILTER (WHERE outcome = 'success'),
			count(*) FILTER (WHERE outcome = 'error')
		FROM %[1]s;
	`, handlerLedger)
	var summary HandlerSummary
	err := d.pool.QueryRow(ctx, summarySql).Scan(&summary.Success, &summary.Error)
	return summary, err
}
