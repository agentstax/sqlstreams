package cli

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	sqlstreams "github.com/agentstax/sqlstreams/client"
	"github.com/agentstax/sqlstreams/pkg/consume"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/spf13/cobra"
)

func newConsumerDestroyCmd(g *globalFlags) *cobra.Command {
	var (
		force bool
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "destroy <stream> <consumer>",
		Short: "Permanently delete a consumer and everything it owns",
		Long: `Permanently delete the named consumer's shared registration: its cursor,
bindings, leases, delivery rows, workers, and schedules. This affects every
instance using that registration. The stream and its messages are untouched.

Refused while any consumer instance is live or delivery rows remain
(failures awaiting retry, or dead-letters). --force overrides both guards:
running instances stop when their worker rows vanish, and delivery rows
are discarded.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streamName, consumerName := args[0], args[1]
			out := cmd.OutOrStdout()

			// the confirmation prompt would pollute the json document stream
			if g.jsonOutput() && !yes {
				return failUsage("refusing to destroy %q without confirmation -- pass --yes with --output json", consumerName)
			}

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			// Check order matters: a doomed call must never waste a prompt.
			found, err := client.Stream[sqlstreams.RawPayload](streamName).Get(ctx)
			if err != nil {
				return translateAdminError(err)
			}
			if found == nil {
				return errStreamNotFound(streamName)
			}

			if !yes {
				if !stdinIsTTY() {
					return failUsage("refusing to destroy %q without confirmation -- pass --yes in non-interactive contexts (e.g. CI)", consumerName)
				}
				if force {
					fmt.Fprintf(out, "%s --force deletes the shared registration for all instances and discards its delivery rows.\n", glyphWarn())
				}
				fmt.Fprintf(out, "This will PERMANENTLY delete consumer %q on stream %q.\n", consumerName, streamName)
				fmt.Fprintln(out, "This cannot be undone.")
				fmt.Fprintln(out)
				fmt.Fprint(out, "Type the consumer name to confirm: ")

				typed, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if strings.TrimSpace(typed) != consumerName {
					// No retry loop -- a piped wrong answer gets one shot, then out.
					fmt.Fprintln(out, "aborted: input did not match consumer name")
					return failPrinted()
				}
			}

			if !g.jsonOutput() {
				fmt.Fprintf(out, "destroying %q... ", consumerName)
			}
			if err := client.Stream[sqlstreams.RawPayload](streamName).Consumer(consumerName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: force}); err != nil {
				if !g.jsonOutput() {
					fmt.Fprintln(out) // end the dangling "destroying..." line
				}
				return consumerDestroyError(streamName, consumerName, err)
			}

			if g.jsonOutput() {
				writeJSON(out, consumerDestroyedDocument{
					Stream:    streamName,
					Consumer:  consumerName,
					Destroyed: true,
				})
				return nil
			}
			fmt.Fprintln(out, "done")
			fmt.Fprintf(out, "%s consumer %q on stream %q destroyed\n", glyphOK(), consumerName, streamName)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&force, "force", false, "destroy even while consumer instances are live or deliveries await an outcome")
	f.BoolVarP(&yes, "yes", "y", false, "skip the interactive confirmation (for non-interactive/CI use)")
	return cmd
}

// consumerDestroyedDocument is consumer destroy's json result: a small
// what-happened record, never the dead rows.
type consumerDestroyedDocument struct {
	Stream    string `json:"stream"`
	Consumer  string `json:"consumer"`
	Destroyed bool   `json:"destroyed"`
}

// consumerDestroyError maps a DestroyConsumer failure to CLI output.
func consumerDestroyError(streamName string, consumerName string, err error) error {
	switch {
	case errors.Is(err, consume.ErrConsumerNotFound):
		return failOp("consumer %q not found on stream %q", consumerName, streamName)
	case errors.Is(err, consume.ErrConsumerGroupLive):
		return failOp("consumer %q still has live instances -- stop them, or pass --force to destroy anyway", consumerName)
	case errors.Is(err, consume.ErrConsumerGroupDeliveriesPending):
		return failOp("consumer %q still has delivery rows (failures awaiting retry, or dead-letters) -- pass --force to discard them", consumerName)
	case errors.Is(err, stream.ErrStreamNotFound):
		return errStreamNotFound(streamName)
	default:
		return translateAdminError(err)
	}
}
