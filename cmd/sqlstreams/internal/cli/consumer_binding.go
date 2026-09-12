package cli

import (
	"github.com/spf13/cobra"
)

func newConsumerBindingCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "binding",
		Short: "Read a consumer's binding declaration",
		Long: `A consumer's binding set comes from ConsumerConfig.Bindings, declared on every
consumer Register. Changing it means changing that code and redeploying;
these commands only read.

A consumer that never declared a set receives every message on its stream.`,
	}

	cmd.AddCommand(newConsumerBindingGetCmd(g))

	return cmd
}
