package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	"github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/spf13/cobra"
)

func newAlertGetCmd(g *globalFlags) *cobra.Command {
	var (
		streamName   string
		consumerName string
		limit        int
	)

	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show one alert's current state, or its history with --limit",
		Long: `Show the current retained alert under a name for one owner: the system by
default, a stream with --stream, a consumer group with --stream and --consumer.
With --limit the newest retained alerts are listed instead, newest first.
An owner that is not registered exits non-zero with its not-found code; an
owner nothing was published for prints "no alert published".`,
		Example: `  sqlstreams alert get partition_count --stream orders.created
  sqlstreams alert get worker_liveness --stream orders.created --limit 5
  sqlstreams alert get disk_pressure --stream orders.created --consumer billing`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 1 {
				return failUsage("get requires an alert name\nusage: sqlstreams alert get <name> [flags]")
			}
			if len(args) > 1 {
				return failUsage("get takes exactly one alert name")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			out := cmd.OutOrStdout()

			if consumerName != "" && streamName == "" {
				return failUsage("--consumer requires --stream")
			}
			if cmd.Flags().Changed("limit") && limit <= 0 {
				return failUsage("--limit must be > 0, got %d", limit)
			}

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			handle := alertHandle(client, name, streamName, consumerName)
			var alerts []*sqlstreams.Alert
			if cmd.Flags().Changed("limit") {
				alerts, err = handle.History(ctx, limit)
			} else {
				var current *sqlstreams.Alert
				current, err = handle.Latest(ctx)
				if current != nil {
					alerts = []*sqlstreams.Alert{current}
				}
			}
			if err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() {
				if alerts == nil {
					alerts = make([]*sqlstreams.Alert, 0)
				}
				writeJSON(out, alertGetDocument{Name: name, Exists: len(alerts) > 0, Alerts: alerts})
				if len(alerts) == 0 {
					return failPrinted()
				}
				return nil
			}

			if len(alerts) == 0 {
				fmt.Fprintf(out, "%s no alert published under %q on %s\n", glyphNo(), name, ownerFlagsCell(streamName, consumerName))
				return failPrinted()
			}

			fmt.Fprintf(out, "%s alert %q on %s\n", glyphOK(), name, ownerCell(alerts[0].Owner))
			for _, published := range alerts {
				fmt.Fprintln(out)
				printAlert(out, published)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&streamName, "stream", "", "the stream that owns the alert")
	f.StringVar(&consumerName, "consumer", "", "the consumer group that owns the alert; needs --stream")
	f.IntVar(&limit, "limit", 10, "list the newest retained alerts instead of the current one")
	return cmd
}

// alertGetDocument is alert get's json result; the not-found case is data
// (exists false, alerts empty), the exit code stays 1. Alerts holds the one
// current alert, or the history newest first under --limit.
type alertGetDocument struct {
	Name   string              `json:"name"`
	Exists bool                `json:"exists"`
	Alerts []*sqlstreams.Alert `json:"alerts"`
}

// alertHandle picks the scope the flags address: none is the system, a
// stream name is that stream, both names is that consumer group.
func alertHandle(client *sqlstreams.Client, name string, streamName string, consumerName string) *sqlstreams.AlertHandle {
	switch {
	case consumerName != "":
		return client.Stream[sqlstreams.RawPayload](streamName).Consumer(consumerName).Alerts().Alert(name)
	case streamName != "":
		return client.Stream[sqlstreams.RawPayload](streamName).Alerts().Alert(name)
	default:
		return client.System().Alerts().Alert(name)
	}
}

// ownerFlagsCell renders the owner the flags addressed, for the line that
// has no alert row to read an owner from.
func ownerFlagsCell(streamName string, consumerName string) string {
	switch {
	case consumerName != "":
		return fmt.Sprintf("consumer_group/%s", consumerName)
	case streamName != "":
		return fmt.Sprintf("stream/%s", streamName)
	default:
		return "system/system"
	}
}

// printAlert is the alert's facts one per line, then its evidence as JSON
// under its own label.
func printAlert(w io.Writer, published *sqlstreams.Alert) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  Status\t%s\n", published.Status)
	fmt.Fprintf(tw, "  Severity\t%s\n", published.Severity)
	fmt.Fprintf(tw, "  At\t%s\n", timeCell(published.At))
	fmt.Fprintf(tw, "  Message\t%s\n", published.Message)
	if published.Detail != "" {
		fmt.Fprintf(tw, "  Detail\t%s\n", published.Detail)
	}
	if published.Hint != "" {
		fmt.Fprintf(tw, "  Hint\t%s\n", published.Hint)
	}
	tw.Flush()

	if len(published.Data) > 0 {
		fmt.Fprintln(w, "  Data")
		fmt.Fprintln(w, indentedDocument(published.Data, "    "))
	}
}

// indentedDocument renders a document as indented JSON under prefix.
func indentedDocument(document map[string]any, prefix string) string {
	encoded, err := json.MarshalIndent(document, prefix, "  ")
	if err != nil {
		return prefix + fmt.Sprint(document)
	}
	return prefix + string(encoded)
}
