package cli

import (
	"fmt"
	"log/slog"
	"text/tabwriter"

	"github.com/allegedlyreliable/sqlstreams/client"
	"github.com/spf13/cobra"
)

func newConsumerGetCmd(g *globalFlags) *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "get <stream> <consumer>",
		Short: "Show a consumer's registration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if quiet && g.jsonOutput() {
				return failUsage("--quiet and --output json cannot be combined")
			}

			connection, err := newConnection(cmd.Context(), g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()

			found, err := connection.client.Stream[sqlstreams.RawPayload](args[0]).Consumer(args[1]).Get(cmd.Context())
			if err != nil {
				return consumerError(args[0], args[1], err)
			}

			if g.jsonOutput() {
				writeJSON(cmd.OutOrStdout(), found)
			} else if !quiet {
				if found == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%s consumer %q on stream %q does not exist\n", glyphNo(), args[1], args[0])
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s consumer %q on stream %q (id=%d)\n", glyphOK(), found.Name, args[0], found.Id)
					out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
					fmt.Fprintf(out, "  StreamId\t%d\n  CreatedAt\t%s\n", found.StreamId, timeCell(found.CreatedAt))
					out.Flush()
				}
			}
			if found == nil {
				return failPrinted()
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "no output; exit code is the answer (0 exists, 1 not)")
	return cmd
}
