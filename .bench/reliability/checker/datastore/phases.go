package datastore

import (
	"context"
	"fmt"

	"github.com/agentstax/sqlstreams/.bench/reliability/record"
)

// ReadPhases returns the run_phase rows in time order.
func (d *CheckerDatastore) ReadPhases(ctx context.Context) ([]record.PhaseRecord, error) {
	phasesSql := fmt.Sprintf(`
		-- lab: datastore.ReadPhases
		SELECT
			at,
			process,
			kind,
			name,
			status,
			detail
		FROM %[1]s
		ORDER BY at;
	`, runPhase)
	rows, err := d.pool.Query(ctx, phasesSql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	phases := []record.PhaseRecord{}
	for rows.Next() {
		var phase record.PhaseRecord
		if err := rows.Scan(&phase.At, &phase.Process, &phase.Kind, &phase.Name, &phase.Status, &phase.Detail); err != nil {
			return nil, err
		}
		phases = append(phases, phase)
	}
	return phases, rows.Err()
}
