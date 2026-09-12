package cli

import (
	"fmt"
	"log/slog"

	"github.com/allegedlyreliable/sqlstreams/client"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/spf13/cobra"
)

func newConsumerBindingGetCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <stream> <consumer>",
		Short: "Show the consumer's effective binding set",
		Long: `Show the consumer's effective binding set -- its newest installed declaration.
A consumer that never declared a set prints none and receives every message
on its stream.`,
		Example: `  sqlstreams consumer binding get orders billing`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streamName, consumerName := args[0], args[1]
			out := cmd.OutOrStdout()

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			// Binding().Get collapses an absent consumer into nil; the command
			// reports absence as not-found like consumer worker list does
			found, err := client.Stream[sqlstreams.RawPayload](streamName).Get(ctx)
			if err != nil {
				return consumerError(streamName, consumerName, err)
			}
			if found == nil {
				return errStreamNotFound(streamName)
			}
			consumer, err := client.Stream[sqlstreams.RawPayload](streamName).Consumer(consumerName).Get(ctx)
			if err != nil {
				return consumerError(streamName, consumerName, err)
			}
			if consumer == nil {
				return failOp("consumer %q not found on stream %q", consumerName, streamName)
			}

			binding, err := client.Stream[sqlstreams.RawPayload](streamName).Consumer(consumerName).Binding().Get(ctx)
			if err != nil {
				return consumerError(streamName, consumerName, err)
			}

			if g.jsonOutput() {
				writeJSON(out, binding)
				return nil
			}

			fmt.Fprintf(out, "%s consumer %q on stream %q\n", glyphOK(), consumerName, streamName)
			if binding == nil {
				fmt.Fprintln(out, "  (no binding declared -- the consumer receives every message on its stream)")
				return nil
			}
			printBindingsTable(out, []*consume.Binding{binding})
			return nil
		},
	}

	return cmd
}
