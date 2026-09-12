package cli

import (
	"github.com/spf13/cobra"
)

func newSystemCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Register, inspect, and destroy the singleton system",
	}

	cmd.AddCommand(newSystemRegisterCmd(g))
	cmd.AddCommand(newSystemGetCmd(g))
	cmd.AddCommand(newSystemDestroyCmd(g))

	return cmd
}
