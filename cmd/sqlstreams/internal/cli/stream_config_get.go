package cli

import (
	"fmt"
	"log/slog"

	"github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/spf13/cobra"
)

func newStreamConfigGetCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name> [key]",
		Short: "Show the stream's config keys: default and current value",
		Example: `  sqlstreams stream config get orders.created
  sqlstreams stream config get orders.created retention_ttl`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			key := ""
			if len(args) == 2 {
				key = args[1]
			}
			out := cmd.OutOrStdout()

			entries := streamConfigKeys
			if key != "" {
				entry, ok := findStreamConfigKey(key)
				if !ok {
					return errUnknownStreamConfigKey(key)
				}
				entries = []streamConfigKey{entry}
			}

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			found, err := client.Stream[sqlstreams.RawPayload](name).Get(ctx)
			if err != nil {
				return translateAdminError(err)
			}
			if found == nil {
				return errStreamNotFound(name)
			}

			if g.jsonOutput() {
				writeJSON(out, toStreamConfigDocument(found, entries))
				return nil
			}

			fmt.Fprintf(out, "%s stream %q (id=%d)\n", glyphOK(), name, found.Id)
			printStreamConfigLines(out, found, entries)
			return nil
		},
	}

	return cmd
}
