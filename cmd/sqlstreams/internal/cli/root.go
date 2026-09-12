package cli

import (
	"context"
	"io"

	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// Execute builds the command tree, runs it through fang, and returns the
// process exit code.
func Execute(ctx context.Context, version string) int {
	root, g := newRootCmd()

	err := fang.Execute(
		ctx,
		root,
		fang.WithVersion(version),
		fang.WithErrorHandler(func(w io.Writer, _ fang.Styles, err error) {
			errorHandler(w, g, err)
		}),
	)
	if err != nil {
		return exitCode(err)
	}
	return 0
}

// configureCommands shows flags in definition order instead of fang's default
// alphabetical, giving every command the same shape: its own flags first (in
// the order they're declared), then the inherited globals, which cobra merges
// in last. The canonical trailing block is therefore always
//
//	... --database-url, --schema, --output, --help
//
// (--help last) -- keep it that way by declaring any new global on the ROOT's
// persistent flags before --help gets merged, and any new per-command flag
// before the merge. fang renders a single FLAGS block with no sub-grouping, so
// ordering is the only lever. (Root alone also shows fang's --version, which
// cobra appends after --help; that ordering isn't ours to control.)
func configureCommands(cmd *cobra.Command) {
	// Cobra skips argument validation on non-runnable command groups.
	if !cmd.Runnable() {
		cmd.Args = func(command *cobra.Command, args []string) error {
			if len(args) > 0 {
				return failUsage("unrecognized command %q for %q", args[0], command.CommandPath())
			}
			return nil
		}
		cmd.RunE = func(command *cobra.Command, _ []string) error { return command.Help() }
	}

	cmd.Flags().SortFlags = false
	cmd.PersistentFlags().SortFlags = false
	for _, sub := range cmd.Commands() {
		configureCommands(sub)
	}
}

// persisted global flags, read by subcommands off the root.
type globalFlags struct {
	databaseURL string
	schema      string
	output      string
}

// jsonOutput reports whether --output json was passed. Commands branch on it
// once, after computing their result; the error handler branches on it too.
func (g *globalFlags) jsonOutput() bool {
	return g.output == "json"
}

func newRootCmd() (*cobra.Command, *globalFlags) {
	g := &globalFlags{}

	root := &cobra.Command{
		Use:   "sqlstreams",
		Short: "Admin CLI for SQLStreams deployments",
		Long: "sqlstreams is the privileged admin tool for a SQLStreams deployment: manage\n" +
			"streams, schema migrations, and maintenance against the control-plane\n" +
			"database.",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if g.output != "text" && g.output != "json" {
				return failUsage("unrecognized output format: %q -- pass text or json", g.output)
			}
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&g.databaseURL, "database-url", "",
		"postgres:// connection URL (or set "+databaseURLEnv+")")
	pf.StringVar(&g.schema, "schema", "",
		"postgres schema sqlstreams's tables live in (or set "+schemaEnv+"; default "+datastore.DefaultSchema+")")
	pf.StringVar(&g.output, "output", "text",
		"output format: text or json (one document on stdout, errors as json on stderr)")

	root.AddCommand(newStreamCmd(g))
	root.AddCommand(newConsumerCmd(g))
	root.AddCommand(newScheduleCmd(g))
	root.AddCommand(newAlertCmd(g))
	root.AddCommand(newMetricCmd(g))
	root.AddCommand(newSystemCmd(g))
	root.AddCommand(newMigrateCmd(g))
	root.AddCommand(newManagerCmd(g))
	root.AddCommand(newExplainCmd(g))

	configureCommands(root)
	return root, g
}
