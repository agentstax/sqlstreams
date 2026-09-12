package cli

import "github.com/spf13/cobra"

func newSystemBindingCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "binding",
		Short: "Read binding declarations across the installation",
	}

	cmd.AddCommand(newSystemBindingListCmd(g))

	return cmd
}
