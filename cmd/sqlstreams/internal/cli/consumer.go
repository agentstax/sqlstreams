package cli

import (
	"github.com/spf13/cobra"
)

func newConsumerCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consumer",
		Short: "Inspect and destroy consumers",
	}

	cmd.AddCommand(newConsumerListCmd(g))
	cmd.AddCommand(newConsumerGetCmd(g))
	cmd.AddCommand(newConsumerWorkerCmd(g))
	cmd.AddCommand(newConsumerBindingCmd(g))
	cmd.AddCommand(newConsumerDestroyCmd(g))

	return cmd
}
