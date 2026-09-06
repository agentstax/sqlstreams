package cli

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/agentstax/vulkan/pkg/consume"
	"github.com/agentstax/vulkan/pkg/topic"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
	"github.com/spf13/cobra"
)

func newConsumerDestroyCmd(g *globalFlags) *cobra.Command {
	var (
		force bool
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "destroy <topic> <consumer>",
		Short: "Permanently delete a consumer and everything it owns",
		Long: `Permanently delete the named consumer's shared registration: its cursor,
bindings, leases, delivery rows, workers, and schedules. This affects every
instance using that registration. The topic and its messages are untouched.

Refused while any consumer instance is live or delivery rows remain
(failures awaiting retry, or dead-letters). --force overrides both guards:
running instances stop when their worker rows vanish, and delivery rows
are discarded.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			topicName, consumerName := args[0], args[1]
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
			found, err := client.Topic[vulkan.RawPayload](topicName).Get(ctx)
			if err != nil {
				return translateAdminError(err)
			}
			if found == nil {
				return errTopicNotFound(topicName)
			}

			if !yes {
				if !stdinIsTTY() {
					return failUsage("refusing to destroy %q without confirmation -- pass --yes in non-interactive contexts (e.g. CI)", consumerName)
				}
				if force {
					fmt.Fprintf(out, "%s --force deletes the shared registration for all instances and discards its delivery rows.\n", glyphWarn())
				}
				fmt.Fprintf(out, "This will PERMANENTLY delete consumer %q on topic %q.\n", consumerName, topicName)
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
			if err := client.Topic[vulkan.RawPayload](topicName).Consumer(consumerName).Destroy(ctx, &vulkan.DestroyOptions{Force: force}); err != nil {
				if !g.jsonOutput() {
					fmt.Fprintln(out) // end the dangling "destroying..." line
				}
				return consumerDestroyError(topicName, consumerName, err)
			}

			if g.jsonOutput() {
				writeJSON(out, consumerDestroyedDocument{
					Topic:     topicName,
					Consumer:  consumerName,
					Destroyed: true,
				})
				return nil
			}
			fmt.Fprintln(out, "done")
			fmt.Fprintf(out, "%s consumer %q on topic %q destroyed\n", glyphOK(), consumerName, topicName)
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
	Topic     string `json:"topic"`
	Consumer  string `json:"consumer"`
	Destroyed bool   `json:"destroyed"`
}

// consumerDestroyError maps a DestroyConsumer failure to CLI output.
func consumerDestroyError(topicName string, consumerName string, err error) error {
	switch {
	case errors.Is(err, consume.ErrConsumerNotFound):
		return failOp("consumer %q not found on topic %q", consumerName, topicName)
	case errors.Is(err, consume.ErrConsumerGroupLive):
		return failOp("consumer %q still has live instances -- stop them, or pass --force to destroy anyway", consumerName)
	case errors.Is(err, consume.ErrConsumerGroupDeliveriesPending):
		return failOp("consumer %q still has delivery rows (failures awaiting retry, or dead-letters) -- pass --force to discard them", consumerName)
	case errors.Is(err, topic.ErrTopicNotFound):
		return errTopicNotFound(topicName)
	default:
		return translateAdminError(err)
	}
}
