package cli

import (
	"github.com/spf13/cobra"
)

func newStreamKeyCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Read what a stream holds under one message key",
		Long: `A message key is set per message at produce time (MessageOptions.MessageKey).
These commands read what the stream holds under one: its compaction head, the
message that currently wins under the key, and its retained messages. The CLI
has no message type in scope, so a payload prints as the JSON the row stores.`,
	}

	cmd.AddCommand(newStreamKeyCompactionHeadCmd(g))
	cmd.AddCommand(newStreamKeyMessagesCmd(g))

	return cmd
}
