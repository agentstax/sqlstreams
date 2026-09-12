package cli

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	"github.com/allegedlyreliable/sqlstreams/client"
	"github.com/spf13/cobra"
)

func newMigrateStatusCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Compare the system's and each stream's current version against what this binary offers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			sysCurrent, err := client.System().MigrationVersion(ctx)
			if err != nil {
				if errors.Is(err, sqlstreams.ErrNotRegistered) {
					return migrateStatusNotRegistered(out, g)
				}
				return translateAdminError(err)
			}

			streams, err := client.Streams(ctx)
			if err != nil {
				return translateAdminError(err)
			}

			sysAvail := availableSystemVersion()
			streamAvail := availableStreamVersion()

			// Read every current version up front so the behind-summary can be
			// computed before anything prints.
			type row struct {
				name      string
				current   int64
				available int64
			}
			rows := []row{{name: "system", current: sysCurrent, available: sysAvail}}
			for _, t := range streams {
				current, err := client.Stream[sqlstreams.RawPayload](t.Name).MigrationVersion(ctx)
				if err != nil {
					return translateAdminError(err)
				}
				rows = append(rows, row{name: t.Name, current: current, available: streamAvail})
			}

			if g.jsonOutput() {
				document := migrateStatusDocument{
					Registered:      true,
					SystemAvailable: sysAvail,
					StreamAvailable: streamAvail,
					System: &migrateSystemDocument{
						Current:   sysCurrent,
						Available: sysAvail,
						Behind:    sysCurrent < sysAvail,
					},
					Streams: make([]migrateStreamDocument, 0, len(rows)-1),
				}
				for _, r := range rows[1:] {
					document.Streams = append(document.Streams, migrateStreamDocument{
						Stream:    r.name,
						Current:   r.current,
						Available: r.available,
						Behind:    r.current < r.available,
					})
				}
				writeJSON(out, document)
				return nil
			}

			fmt.Fprintf(out, "latest available: system %d, stream %d\n\n", sysAvail, streamAvail)

			fmt.Fprintf(out, "system: current %d, available %d\n\n", sysCurrent, sysAvail)

			tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
			fmt.Fprintln(tw, "STREAM\tCURRENT\tAVAILABLE")
			for _, r := range rows[1:] {
				fmt.Fprintf(tw, "%s\t%d\t%d\n", r.name, r.current, r.available)
			}
			tw.Flush()

			// A binary older than the DB (current > available) is fine, not behind
			// -- only current < available is actionable.
			systemBehind := sysCurrent < sysAvail
			streamsBehind := 0
			for _, r := range rows[1:] {
				if r.current < r.available {
					streamsBehind++
				}
			}
			if systemBehind || streamsBehind > 0 {
				fmt.Fprintln(out)
			}
			if systemBehind {
				fmt.Fprintf(out, "system behind (%d < %d) -- run `sqlstreams migrate system up --target-version %d`\n", sysCurrent, sysAvail, sysAvail)
			}
			if streamsBehind > 0 {
				fmt.Fprintf(out, "%s behind -- run `sqlstreams migrate streams up --target-version %d`\n", pluralize(streamsBehind, "stream"), streamAvail)
			}
			return nil
		},
	}
}

// migrateStatusDocument is migrate status's json result. Registered false
// means the control-plane tables were never created; system is then null and
// streams empty.
type migrateStatusDocument struct {
	Registered      bool                    `json:"registered"`
	SystemAvailable int64                   `json:"system_available"`
	StreamAvailable int64                   `json:"stream_available"`
	System          *migrateSystemDocument  `json:"system"`
	Streams         []migrateStreamDocument `json:"streams"`
}

// migrateSystemDocument is the control-plane tables' versions. It carries no
// name: a database holds exactly one system, so a name could only repeat the
// key it already sits under -- and a stream may itself be named "system".
type migrateSystemDocument struct {
	Current   int64 `json:"current"`
	Available int64 `json:"available"`
	Behind    bool  `json:"behind"`
}

// migrateStreamDocument is one registered stream's versions.
type migrateStreamDocument struct {
	Stream    string `json:"stream"`
	Current   int64  `json:"current"`
	Available int64  `json:"available"`
	Behind    bool   `json:"behind"`
}

// migrateStatusNotRegistered is the shared never-registered result: still
// exit 0 -- an unregistered database is an answer, not a failure.
func migrateStatusNotRegistered(w io.Writer, g *globalFlags) error {
	if g.jsonOutput() {
		writeJSON(w, migrateStatusDocument{
			SystemAvailable: availableSystemVersion(),
			StreamAvailable: availableStreamVersion(),
			Streams:         make([]migrateStreamDocument, 0),
		})
		return nil
	}
	fmt.Fprintln(w, "system not registered -- run `sqlstreams system register`")
	return nil
}
