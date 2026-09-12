package cli

import (
	"log/slog"

	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/spf13/cobra"
)

func newScheduleMessagesCmd(g *globalFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "messages <name>",
		Short: "List a schedule's newest messages",
		Args:  requireScheduleName("messages"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				return failUsage("--limit must be > 0, got %d", limit)
			}
			ctx := cmd.Context()
			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()

			rows, err := connection.client.Scheduler(args[0]).Messages(ctx, limit)
			if err != nil {
				return translateAdminError(err)
			}
			if g.jsonOutput() {
				if rows == nil {
					rows = make([]*schedule.ScheduleMessageStatus, 0)
				}
				writeJSON(cmd.OutOrStdout(), rows)
			} else {
				printScheduleMessages(cmd.OutOrStdout(), rows)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "how many of the newest messages to list")
	return cmd
}
