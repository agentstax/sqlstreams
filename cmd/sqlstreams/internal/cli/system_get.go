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
	var quiet bool
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show the singleton system config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if quiet && g.jsonOutput() {
				return failUsage("--quiet and --output json cannot be combined")
			}

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
			if g.jsonOutput() {
				writeJSON(out, sys)
				if sys == nil {
					return failPrinted()
				}
				return nil
			}
			if quiet {
				if sys == nil {
					return failPrinted()
				}
				return nil
			}
			if sys == nil {
				return failOp("system not registered -- run `sqlstreams system register` first")
			}

			fmt.Fprintf(out, "%s system config\n", glyphOK())
			printSystemDetail(out, sys)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "no output; exit code is the answer (0 exists, 1 not)")
	return cmd
}

func printSystemDetail(w io.Writer, s *system.System) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  CreatedAt\t%s\n", timeCell(s.CreatedAt))
	fmt.Fprintf(tw, "  UpdatedAt\t%s\n", timeCell(s.UpdatedAt))
	tw.Flush()
}
