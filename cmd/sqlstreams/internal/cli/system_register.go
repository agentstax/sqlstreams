package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

func newSystemRegisterCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "register",
		Short: "Register the system with default config",
		Long: "Create the shared control-plane tables if absent and apply the default\n" +
			"system config, including built-in streams, alert schedules, and collector rate.\n" +
			"An existing system is redeclared with those defaults. Use System().Register\n" +
			"in application code to declare custom config. Migrate existing tables with\n" +
			"sqlstreams migrate system up --target-version N.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			if err := client.System().Register(ctx, nil); err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() {
				writeJSON(out, systemRegisterDocument{Registered: true})
				return nil
			}

			fmt.Fprintf(out, "%s system registered\n", glyphOK())
			return nil
		},
	}
}

// Registration applies the default system declaration.
type systemRegisterDocument struct {
	Registered bool `json:"registered"`
}
