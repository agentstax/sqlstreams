package cli

import (
	"log/slog"

	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/spf13/cobra"
)

func newScheduleStatusCmd(g *globalFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show a schedule's per-consumer-group run outcomes",
		Args:  requireScheduleName("status"),
		RunE: func(cmd *cobra.Command, args []string) error {

			ctx := cmd.Context()
			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()

			rows, err := connection.client.Scheduler(args[0]).Status(ctx)
			if err != nil {
				return translateAdminError(err)
			}
			if g.jsonOutput() {
				if rows == nil {
					rows = make([]*schedule.ScheduleConsumerGroupSummary, 0)
				}
				writeJSON(cmd.OutOrStdout(), rows)
			} else {
				printScheduleStatuses(cmd.OutOrStdout(), rows)
			}
			return nil
		},
	}

	return cmd
}
