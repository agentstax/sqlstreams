package cli

import (
	"context"
	"fmt"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"io"
	"log/slog"

	"github.com/allegedlyreliable/sqlstreams/client"
	migratecontroller "github.com/allegedlyreliable/sqlstreams/pkg/migrate/controller"
	"github.com/spf13/cobra"
)

func newMigrateSystemCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Migrate the shared control-plane tables",
	}
	cmd.AddCommand(newDirectionCmd(g, scopeSystem, dirUp))
	cmd.AddCommand(newDirectionCmd(g, scopeSystem, dirDown))
	return cmd
}

func newMigrateStreamsCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "streams",
		Short: "Migrate every registered stream's tables",
	}
	cmd.AddCommand(newDirectionCmd(g, scopeStreams, dirUp))
	cmd.AddCommand(newDirectionCmd(g, scopeStreams, dirDown))
	return cmd
}

func newMigrateStreamCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Migrate a single stream's tables, by name",
	}
	cmd.AddCommand(newDirectionCmd(g, scopeStream, dirUp))
	cmd.AddCommand(newDirectionCmd(g, scopeStream, dirDown))
	return cmd
}

// newDirectionCmd builds one up/down leaf. All six leaves (three scopes x two
// directions) share this body -- they differ only in the scope they resolve and
// the direction they guard. The stream scope alone takes a <name> positional.
func newDirectionCmd(g *globalFlags, s scope, dir direction) *cobra.Command {
	var targetVersion int64

	use := dir.verb()
	args := cobra.NoArgs
	if s == scopeStream {
		use = dir.verb() + " <name>"
		args = requireMigrateStreamName(dir)
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Migrate the %s tables %s to the required --target-version", scopeNoun(s), directionWord(dir)),
		Args:  args,
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			// --target-version is mandatory: no implicit "to latest" on up, no implicit
			// "one step back" on down. The message is direction-specific.
			if !cmd.Flags().Changed("target-version") {
				return errTargetVersionRequired(s, dir)
			}
			if ceiling := s.ceiling(); targetVersion < 1 || targetVersion > ceiling {
				return failUsage("--target-version %d is out of range [1, %d] for this binary -- run `sqlstreams migrate versions` to see what's available", targetVersion, ceiling)
			}

			name := ""
			if s == scopeStream {
				name = cmdArgs[0]
			}

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			targets, err := gatherTargets(ctx, client, s, name)
			if err != nil {
				return err
			}
			if s == scopeStreams && len(targets) == 0 {
				if g.jsonOutput() {
					writeJSON(out, toMigrateResultDocument(s, targets, targetVersion, 0))
					return nil
				}
				fmt.Fprintln(out, "no streams registered")
				return nil
			}

			moving, err := guardDirection(targets, dir, targetVersion)
			if err != nil {
				return err
			}
			if moving == 0 {
				if g.jsonOutput() {
					writeJSON(out, toMigrateResultDocument(s, targets, targetVersion, 0))
					return nil
				}
				printMigrateNoop(out, s, targets, targetVersion)
				return nil
			}

			// Fast pre-flight, not a guarantee -- see Controller.IsLocked. Catches the
			// common case (another migrate already running) before committing to a
			// call that would otherwise block silently until that one finishes.
			ds, err := datastore.NewPostgresDatastore(ctx, connection.pool, connection.config)
			if err != nil {
				return failOp("could not connect to database: %v", err)
			}
			controller, err := migratecontroller.NewController(ds, ds.Logger)
			if err != nil {
				return err
			}
			locked, err := controller.IsLocked(ctx)
			if err != nil {
				return translateAdminError(err)
			}
			if locked {
				return failOp("another migration is already in progress (advisory lock held) -- wait for it to finish, or confirm no other migrate process is actually running before retrying")
			}

			if err := runScopeMigrate(ctx, client, s, name, targetVersion); err != nil {
				return migrateError(err)
			}

			if g.jsonOutput() {
				writeJSON(out, toMigrateResultDocument(s, targets, targetVersion, moving))
				return nil
			}
			printMigrateResult(out, s, dir, targets, targetVersion, moving)
			return nil
		},
	}

	cmd.Flags().Int64Var(&targetVersion, "target-version", 0, "target migration version (required)")
	return cmd
}

// requireMigrateStreamName is the Args rule for `migrate stream up|down <name>` --
// its own validator, not the stream-command one, so the usage line names the right
// path (`sqlstreams migrate stream ...`, not `sqlstreams stream ...`).
func requireMigrateStreamName(dir direction) cobra.PositionalArgs {
	verb := dir.verb()
	return func(_ *cobra.Command, args []string) error {
		if len(args) < 1 {
			return failUsage("%s requires a stream name\nusage: sqlstreams migrate stream %s <name> --target-version N", verb, verb)
		}
		if len(args) > 1 {
			return failUsage("migrate stream %s takes exactly one stream name", verb)
		}
		return nil
	}
}

// errTargetVersionRequired is the direction-specific teaching error for a missing --target-version.
func errTargetVersionRequired(s scope, dir direction) error {
	if dir == dirDown {
		return failUsage("--target-version is required for %s down -- downgrades name an explicit target, there's no implicit \"down one step\"", scopeNoun(s))
	}
	return failUsage("--target-version is required (e.g. --target-version %d) -- run `sqlstreams migrate versions` to see what's available", s.ceiling())
}

func runScopeMigrate(ctx context.Context, client *sqlstreams.Client, s scope, name string, targetVersion int64) error {
	switch s {
	case scopeSystem:
		return client.System().Migrate(ctx, targetVersion)
	case scopeStream:
		return client.Stream[sqlstreams.RawPayload](name).Migrate(ctx, targetVersion)
	default:
		return client.System().MigrateStreams(ctx, targetVersion)
	}
}

// migrateResultDocument is a migrate up/down's json result; migrated_count 0
// means every target was already at the target version. The command names
// what it targeted, so the document reports only what happened: stream is
// present when one stream was named and absent otherwise.
type migrateResultDocument struct {
	Stream        string `json:"stream,omitempty"` // the named stream; absent for system and every-stream runs
	TargetVersion int64  `json:"target_version"`
	MigratedCount int    `json:"migrated_count"`
}

func toMigrateResultDocument(s scope, targets []migrateTarget, targetVersion int64, moving int) migrateResultDocument {
	document := migrateResultDocument{TargetVersion: targetVersion, MigratedCount: moving}
	if s == scopeStream {
		document.Stream = targets[0].name
	}
	return document
}

func printMigrateNoop(w io.Writer, s scope, targets []migrateTarget, targetVersion int64) {
	if s == scopeStreams {
		fmt.Fprintf(w, "%s all streams already at version %d, nothing to do\n", glyphOK(), targetVersion)
		return
	}
	fmt.Fprintf(w, "%s %s already at version %d, nothing to do\n", glyphOK(), singleLabel(s, targets), targetVersion)
}

func printMigrateResult(w io.Writer, s scope, dir direction, targets []migrateTarget, targetVersion int64, moving int) {
	if s == scopeStreams {
		fmt.Fprintf(w, "%s migrated %s %s to version %d\n", glyphOK(), pluralize(moving, "stream"), dir.verb(), targetVersion)
		return
	}
	fmt.Fprintf(w, "%s %s migrated %s to version %d\n", glyphOK(), singleLabel(s, targets), dir.verb(), targetVersion)
}

func singleLabel(s scope, targets []migrateTarget) string {
	if s == scopeStream {
		return fmt.Sprintf("stream %q", targets[0].name)
	}
	return "system"
}

func scopeNoun(s scope) string {
	switch s {
	case scopeSystem:
		return "system"
	case scopeStream:
		return "stream"
	default:
		return "streams"
	}
}

// directionWord is the human phrasing for a Short line -- "forward"/"back",
// where verb() gives the command word "up"/"down".
func directionWord(dir direction) string {
	if dir == dirDown {
		return "back"
	}
	return "forward"
}
