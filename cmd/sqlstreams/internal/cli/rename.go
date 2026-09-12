package cli

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/agentstax/sqlstreams/client"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/spf13/cobra"
)

func newStreamRenameCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <name> <new-name>",
		Short: "Change a stream's name (id, config, and messages are untouched)",
		Long: "Rename a stream. Everything but the name -- id, config, stored messages --\n" +
			"is untouched, since tables are addressed by id internally.\n\n" +
			"The old name is free the moment this returns. Running producers/consumers\n" +
			"keep working (they resolved the id at their Register), but anything still\n" +
			"configured with the old name fails its next restart -- or silently attaches\n" +
			"to a new stream later registered under the freed name. Update those configs.",
		Example: "sqlstreams stream rename orders.created orders.v2",
		Args:    requireRenameArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			oldName, newName := args[0], args[1]
			out := cmd.OutOrStdout()

			if newName == oldName {
				return failUsage("new name matches the current name -- nothing to rename")
			}

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			renamed, err := client.Stream[sqlstreams.RawPayload](oldName).Rename(ctx, newName)
			if err != nil {
				switch {
				case errors.Is(err, stream.ErrStreamNotFound):
					return errStreamNotFound(oldName)
				case errors.Is(err, stream.ErrStreamNameTaken):
					return failOp("stream %q already exists -- pick a name that's free, or destroy it first", newName)
				default:
					return translateAdminError(err)
				}
			}

			// echo the row as it now exists -- the get-shape
			if g.jsonOutput() {
				writeJSON(out, toStreamDocument(renamed))
				return nil
			}
			fmt.Fprintf(out, "%s renamed stream %q -> %q (id=%d)\n", glyphOK(), oldName, newName, renamed.Id)
			return nil
		},
	}

	return cmd
}

// requireRenameArgs is rename's Args rule: exactly two names, with a usage line
// naming the right path when either is missing (cobra's generic "accepts 2
// arg(s)" text names neither).
func requireRenameArgs(_ *cobra.Command, args []string) error {
	if len(args) < 2 {
		return failUsage("rename requires a stream name and a new name\nusage: sqlstreams stream rename <name> <new-name>")
	}
	if len(args) > 2 {
		return failUsage("rename takes exactly two names: <name> <new-name>")
	}
	return nil
}
