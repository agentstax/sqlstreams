package cli

import (
	"fmt"
	"log/slog"
	"text/tabwriter"

	"github.com/allegedlyreliable/sqlstreams/client"
	"github.com/spf13/cobra"
)

func newConsumerListCmd(g *globalFlags) *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "list <stream>",
		Short: "List every consumer registered on a stream",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if quiet && g.jsonOutput() {
				return failUsage("--quiet and --output json cannot be combined")
			}

			connection, err := newConnection(cmd.Context(), g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()

			consumers, err := connection.client.Stream[sqlstreams.RawPayload](args[0]).Consumers(cmd.Context())
			if err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() {
				if consumers == nil {
					consumers = make([]*sqlstreams.Consumer, 0)
				}
				writeJSON(cmd.OutOrStdout(), consumers)
				return nil
			}

			out := cmd.OutOrStdout()
			if quiet {
				for _, consumer := range consumers {
					fmt.Fprintln(out, consumer.Name)
				}
				return nil
			}
			if len(consumers) == 0 {
				fmt.Fprintln(out, "no consumers registered")
				return nil
			}

			table := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
			fmt.Fprintln(table, "NAME\tID")
			for _, consumer := range consumers {
				fmt.Fprintf(table, "%s\t%d\n", consumer.Name, consumer.Id)
			}
			table.Flush()
			fmt.Fprintf(out, "\n%s\n", pluralize(len(consumers), "consumer"))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "names only, one per line (for scripts)")
	return cmd
}
