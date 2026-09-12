package cli

import (
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/spf13/cobra"
)

func newStreamListCmd(g *globalFlags) *cobra.Command {
	var quiet bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every registered stream",
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

			streams, err := client.Streams(ctx)
			if err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() {
				writeJSON(out, toStreamDocuments(streams))
				return nil
			}

			if quiet {
				printStreamNames(out, streams)
			} else {
				printStreamsTable(out, streams)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&quiet, "quiet", "q", false, "names only, one per line (for scripts)")
	return cmd
}

func printStreamNames(w io.Writer, streams []*stream.Stream) {
	for _, t := range streams {
		fmt.Fprintln(w, t.Name)
	}
}

func printStreamsTable(w io.Writer, streams []*stream.Stream) {
	if len(streams) == 0 {
		fmt.Fprintln(w, "no streams registered")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tID")
	for _, t := range streams {
		fmt.Fprintf(tw, "%s\t%d\n", t.Name, t.Id)
	}
	tw.Flush()

	fmt.Fprintf(w, "\n%s\n", pluralize(len(streams), "stream"))
}
