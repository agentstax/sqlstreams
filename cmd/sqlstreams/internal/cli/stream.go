package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStreamCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Inspect, rename, and destroy streams",
	}

	cmd.AddCommand(newStreamListCmd(g))
	cmd.AddCommand(newStreamGetCmd(g))
	cmd.AddCommand(newStreamConfigCmd(g))
	cmd.AddCommand(newStreamKeyCmd(g))
	cmd.AddCommand(newStreamRenameCmd(g))
	cmd.AddCommand(newStreamDestroyCmd(g))

	return cmd
}

// requireStreamName is the shared Args rule for every single-stream command
// (get/rename/destroy): exactly one name, with a verb-specific usage line when
// it's missing so they fail identically instead of leaking cobra's generic
// "accepts 1 arg(s)" text. extraLines are appended to the missing-name error.
func requireStreamName(verb string, extraLines ...string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) < 1 {
			msg := fmt.Sprintf("%s requires a stream name\nusage: sqlstreams stream %s <name> [flags]", verb, verb)
			for _, line := range extraLines {
				msg += "\n" + line
			}
			return failUsage("%s", msg)
		}
		if len(args) > 1 {
			return failUsage("%s takes exactly one stream name", verb)
		}
		return nil
	}
}
