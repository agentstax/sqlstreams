package cli

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"text/tabwriter"

	"github.com/agentstax/sqlstreams/pkg/consume"
	"github.com/spf13/cobra"
)

func newSystemBindingListCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every consumer's declared binding set",
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

			bindings, err := client.System().Bindings(ctx)
			if err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() {
				if bindings == nil {
					bindings = make([]*consume.Binding, 0)
				}
				writeJSON(out, bindings)
				return nil
			}

			printBindingsTable(out, bindings)
			return nil
		},
	}
	return cmd
}

func printBindingsTable(w io.Writer, bindings []*consume.Binding) {
	if len(bindings) == 0 {
		fmt.Fprintln(w, "no binding declarations -- every consumer receives every message on its stream")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "CONSUMER\tSTREAM\tSTATUS\tPATTERNS\tDECLARED BY\tDECLARED AT\tLAST ATTEMPT")
	for _, binding := range bindings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			binding.ConsumerGroupName,
			binding.StreamName,
			string(binding.Status),
			patternsCell(binding.Patterns),
			binding.DeclaredBy,
			timeCell(binding.DeclaredAt),
			timeCell(binding.AttemptedAt))
	}
	tw.Flush()

	fmt.Fprintf(w, "\n%s\n", pluralize(len(bindings), "declaration"))
}

func patternsCell(patterns []string) string {
	if len(patterns) == 0 {
		return "(whole stream)"
	}
	return strings.Join(patterns, ",")
}
