package cli

import (
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	"github.com/agentstax/sqlstreams/pkg/system"
	"github.com/spf13/cobra"
)

func newSystemGetCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show the singleton system config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			sys, err := client.System().Get(ctx)
			if err != nil {
				return translateAdminError(err)
			}
			if sys == nil {
				return failOp("system not registered -- run `sqlstreams migrate init` first")
			}

			if g.jsonOutput() {
				writeJSON(out, sys)
				return nil
			}

			fmt.Fprintf(out, "%s system config\n", glyphOK())
			printSystemDetail(out, sys)
			return nil
		},
	}
}

func printSystemDetail(w io.Writer, s *system.System) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  CreatedAt\t%s\n", timeCell(s.CreatedAt))
	fmt.Fprintf(tw, "  UpdatedAt\t%s\n", timeCell(s.UpdatedAt))
	tw.Flush()
}
