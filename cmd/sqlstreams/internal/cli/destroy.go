package cli

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"log/slog"
	"os"
	"strings"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	"github.com/spf13/cobra"
)

func newStreamDestroyCmd(g *globalFlags) *cobra.Command {
	var (
		force bool
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "destroy <name>",
		Short: "Permanently delete a stream and every message it holds",
		Args:  requireStreamName("destroy"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			out := cmd.OutOrStdout()

			// the confirmation prompt would pollute the json document stream
			if g.jsonOutput() && !yes {
				return failUsage("refusing to destroy %q without confirmation -- pass --yes with --output json", name)
			}

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client
			ds, err := datastore.NewPostgresDatastore(ctx, connection.pool, connection.config)
			if err != nil {
				return failOp("could not connect to database: %v", err)
			}

			// Check order matters: a doomed call must never waste a prompt.
			// 1. exists?
			found, err := client.Stream[sqlstreams.RawPayload](name).Get(ctx)
			if err != nil {
				return translateAdminError(err)
			}
			if found == nil {
				return errStreamNotFound(name)
			}

			// 2. emptiness -- the client doesn't expose this, so build a
			// stream controller over the same pool (public API, no pkg change).
			streamController, err := streamcontroller.NewStreamController(ds, logging.NewDefaultLogger(os.Stderr, slog.LevelError))
			if err != nil {
				return failOp("could not check whether stream is empty: %v", err)
			}
			empty, err := streamController.IsEmpty(ctx, found.Id)
			if err != nil {
				return translateAdminError(err)
			}
			if !empty && !force {
				return errStreamNotEmpty(name)
			}

			// 3. confirm, unless --yes.
			if !yes {
				if !stdinIsTTY() {
					return failUsage("refusing to destroy %q without confirmation -- pass --yes in non-interactive contexts (e.g. CI)", name)
				}
				if !empty { // implies --force by the gate above
					fmt.Fprintf(out, "%s stream %q still holds messages -- --force will delete them along with the stream.\n", glyphWarn(), name)
				}
				fmt.Fprintf(out, "This will PERMANENTLY delete stream %q (id=%d) and every message it holds.\n", name, found.Id)
				fmt.Fprintln(out, "This cannot be undone.")
				fmt.Fprintln(out)
				fmt.Fprint(out, "Type the stream name to confirm: ")

				typed, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if strings.TrimSpace(typed) != name {
					// No retry loop -- a piped wrong answer gets one shot, then out.
					fmt.Fprintln(out, "aborted: input did not match stream name")
					return failPrinted()
				}
			}

			// 4. destroy.
			if !g.jsonOutput() {
				fmt.Fprintf(out, "destroying %q... ", name)
			}
			if err := client.Stream[sqlstreams.RawPayload](name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: force}); err != nil {
				if !g.jsonOutput() {
					fmt.Fprintln(out) // end the dangling "destroying..." line
				}
				return destroyError(name, err)
			}

			if g.jsonOutput() {
				writeJSON(out, streamDestroyedDocument{
					Stream:    name,
					StreamId:  found.Id,
					Destroyed: true,
				})
				return nil
			}
			fmt.Fprintln(out, "done")
			fmt.Fprintf(out, "%s stream %q destroyed\n", glyphOK(), name)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&force, "force", false, "required to destroy a stream that still holds messages")
	f.BoolVarP(&yes, "yes", "y", false, "skip the interactive confirmation (for non-interactive/CI use)")
	return cmd
}

// streamDestroyedDocument is stream destroy's json result: a small
// what-happened record, never the dead row.
type streamDestroyedDocument struct {
	Stream    string `json:"stream"`
	StreamId  int64  `json:"stream_id"`
	Destroyed bool   `json:"destroyed"`
}

// errStreamNotFound / errStreamNotEmpty are the two operator-facing messages
// destroy raises from more than one place (pre-flight and the post-delete race
// map below). Single source each so the wording can never drift.
func errStreamNotFound(name string) error {
	return failOp("stream %q not found", name)
}

func errStreamNotEmpty(name string) error {
	return failOp("stream %q still holds messages -- pass --force to destroy anyway (this is unrecoverable data loss, not just a schema drop)", name)
}

// destroyError maps a DestroyStream failure to CLI output. Most cases are caught
// in pre-flight; these are the narrow races (a producer writes, or the stream is
// dropped, between our checks and the delete) plus anything unexpected.
func destroyError(name string, err error) error {
	switch {
	case errors.Is(err, stream.ErrStreamNotEmpty):
		return errStreamNotEmpty(name)
	case errors.Is(err, stream.ErrStreamNotFound):
		return errStreamNotFound(name)
	default:
		return translateAdminError(err)
	}
}
