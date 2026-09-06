package cli

import (
	"fmt"
	"log/slog"

	"github.com/agentstax/vulkan/pkg/consume"
	"github.com/agentstax/vulkan/pkg/vulkan"
	"github.com/spf13/cobra"
)

func newConsumerBindingGetCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <topic> <consumer>",
		Short: "Show the consumer's effective binding set",
		Long: `Show the consumer's effective binding set -- its newest installed declaration.
A consumer that never declared a set prints none and receives every message
on its topic.`,
		Example: `  vulkan consumer binding get orders billing`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			topicName, consumerName := args[0], args[1]
			out := cmd.OutOrStdout()

			client, closeClient, err := openClient(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer closeClient()

			// Binding().Get collapses an absent consumer into nil; the command
			// reports absence as not-found like consumer config get does
			found, err := client.Topic[vulkan.RawPayload](topicName).Get(ctx)
			if err != nil {
				return consumerError(topicName, consumerName, err)
			}
			if found == nil {
				return errTopicNotFound(topicName)
			}
			consumer, err := client.Topic[vulkan.RawPayload](topicName).Consumer(consumerName).Get(ctx)
			if err != nil {
				return consumerError(topicName, consumerName, err)
			}
			if consumer == nil {
				return failOp("consumer %q not found on topic %q", consumerName, topicName)
			}

			binding, err := client.Topic[vulkan.RawPayload](topicName).Consumer(consumerName).Binding().Get(ctx)
			if err != nil {
				return consumerError(topicName, consumerName, err)
			}

			if g.jsonOutput() {
				writeJSON(out, binding)
				return nil
			}

			fmt.Fprintf(out, "%s consumer %q on topic %q\n", glyphOK(), consumerName, topicName)
			if binding == nil {
				fmt.Fprintln(out, "  (no binding declared -- the consumer receives every message on its topic)")
				return nil
			}
			printBindingsTable(out, []*consume.Binding{binding})
			return nil
		},
	}

	return cmd
}
