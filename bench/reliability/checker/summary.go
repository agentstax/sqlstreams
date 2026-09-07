package checker

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/bench/reliability/record"
)

// RecordSummary counts the rows loaded from the record files, per kind.
type RecordSummary struct {
	Produce int64 `json:"produce"`
	Handler int64 `json:"handler"`
	Phase   int64 `json:"phase"`
}

// ProduceSummary counts the records' produce outcomes; Duplicate is the
// committed replies the library flagged as an idempotency-key repeat.
type ProduceSummary struct {
	Attempted int64 `json:"attempted"`
	Committed int64 `json:"committed"`
	Rejected  int64 `json:"rejected"`
	Unknown   int64 `json:"unknown"`
	Duplicate int64 `json:"duplicate"`
}

// HandlerSummary counts the records' handler invocations by outcome.
type HandlerSummary struct {
	Success int64 `json:"success"`
	Error   int64 `json:"error"`
}

func (c *Checker) produceSummary(ctx context.Context) (ProduceSummary, error) {
	summarySql := fmt.Sprintf(`
		-- lab: checker.produceSummary
		SELECT
			count(*) FILTER (WHERE kind = 'attempted'),
			count(*) FILTER (WHERE kind = 'committed'),
			count(*) FILTER (WHERE kind = 'rejected'),
			count(*) FILTER (WHERE kind = 'unknown'),
			count(*) FILTER (WHERE kind = 'committed' AND duplicate)
		FROM %[1]s;
	`, produceLedger)
	var summary ProduceSummary
	err := c.pool.QueryRow(ctx, summarySql).Scan(&summary.Attempted, &summary.Committed, &summary.Rejected, &summary.Unknown, &summary.Duplicate)
	return summary, err
}

func (c *Checker) handlerSummary(ctx context.Context) (HandlerSummary, error) {
	summarySql := fmt.Sprintf(`
		-- lab: checker.handlerSummary
		SELECT
			count(*) FILTER (WHERE outcome = 'success'),
			count(*) FILTER (WHERE outcome = 'error')
		FROM %[1]s;
	`, handlerLedger)
	var summary HandlerSummary
	err := c.pool.QueryRow(ctx, summarySql).Scan(&summary.Success, &summary.Error)
	return summary, err
}

func (c *Checker) readPhases(ctx context.Context) ([]record.Phase, error) {
	phasesSql := fmt.Sprintf(`
		-- lab: checker.readPhases
		SELECT
			at,
			role,
			kind,
			name,
			status,
			detail
		FROM %[1]s
		ORDER BY at;
	`, runPhase)
	rows, err := c.pool.Query(ctx, phasesSql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	phases := []record.Phase{}
	for rows.Next() {
		var phase record.Phase
		if err := rows.Scan(&phase.At, &phase.Role, &phase.Kind, &phase.Name, &phase.Status, &phase.Detail); err != nil {
			return nil, err
		}
		phases = append(phases, phase)
	}
	return phases, rows.Err()
}

// synchronousCommit is the server setting the verdict records: a run with
// it off proves less about durability than one with it on.
func (c *Checker) synchronousCommit(ctx context.Context) (string, error) {
	var setting string
	err := c.pool.QueryRow(ctx, "SHOW synchronous_commit;").Scan(&setting)
	return setting, err
}
