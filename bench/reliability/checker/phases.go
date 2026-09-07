package checker

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/bench/reliability/record"
)

// readPhases returns the run_phase rows in time order, for the results.
func (c *Checker) readPhases(ctx context.Context) ([]record.Phase, error) {
	phasesSql := fmt.Sprintf(`
		-- lab: checker.readPhases
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
	rows, err := c.pool.Query(ctx, phasesSql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	phases := []record.Phase{}
	for rows.Next() {
		var phase record.Phase
		if err := rows.Scan(&phase.At, &phase.Process, &phase.Kind, &phase.Name, &phase.Status, &phase.Detail); err != nil {
			return nil, err
		}
		phases = append(phases, phase)
	}
	return phases, rows.Err()
}
