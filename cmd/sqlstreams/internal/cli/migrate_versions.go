package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newMigrateVersionsCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "versions",
		Short: "List the migration versions this binary knows how to reach",
		Long: "List every migration version compiled into THIS binary, per scope. The step\n" +
			"registry is the source of truth here -- nothing is read from a database.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			if g.jsonOutput() {
				writeJSON(out, migrateVersionsDocument{
					System: availableSystemVersion(),
					Stream: availableStreamVersion(),
				})
				return nil
			}

			printScopeVersions(out, "system migration versions (this binary):", availableSystemVersion())
			fmt.Fprintln(out)
			printScopeVersions(out, "stream migration versions (this binary):", availableStreamVersion())
			return nil
		},
	}
}

// migrateVersionsDocument is the compiled-in version ceiling per scope; v1
// (the baseline) through the ceiling is reachable.
type migrateVersionsDocument struct {
	System int64 `json:"system"`
	Stream int64 `json:"stream"`
}

// printScopeVersions lists v1 (the baseline) through the compiled ceiling. Steps
// carry no description in the registry, so the number is all there is to show;
// v1 is annotated because it's created by `system register`, not a versioned step.
func printScopeVersions(w io.Writer, title string, ceiling int64) {
	fmt.Fprintln(w, title)
	for v := int64(1); v <= ceiling; v++ {
		if v == 1 {
			fmt.Fprintf(w, "  %d  baseline\n", v)
			continue
		}
		fmt.Fprintf(w, "  %d\n", v)
	}
	if ceiling == 1 {
		fmt.Fprintln(w, "  (no versioned steps compiled in yet)")
	}
}
