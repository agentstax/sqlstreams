package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"text/tabwriter"

	"github.com/allegedlyreliable/sqlstreams/client"
	"github.com/spf13/cobra"
)

func newStreamMaintenanceCmd(g *globalFlags, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Suspend, unsuspend, and inspect a stream's " + name,
	}

	for _, operation := range []struct {
		verb        string
		description string
	}{
		{"suspend", "Prevent new claims and request running work to stop at its next heartbeat. This command does not wait for work to stop."},
		{"unsuspend", "Permit one instance to run. A manager must be running to claim it."},
		{"status", "Show the operational target, live claims, and consecutive failures."},
	} {
		cmd.AddCommand(&cobra.Command{
			Use:   operation.verb + " <name>",
			Short: operation.description,
			Args:  requireStreamName(name + " " + operation.verb),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				streamName := args[0]
				connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
				if err != nil {
					return err
				}
				defer connection.Close()

				stream := connection.client.Stream[sqlstreams.RawPayload](streamName)
				var handle *sqlstreams.MaintenanceHandle
				switch name {
				case "janitor":
					handle = stream.Janitor()
				case "vacuum":
					handle = stream.Vacuum()
				}

				switch operation.verb {
				case "status":
					snapshot, err := handle.Status(ctx)
					if err != nil {
						return translateAdminError(err)
					}
					if g.jsonOutput() {
						document, err := newMaintenanceStatusDocument(snapshot)
						if err != nil {
							return err
						}
						writeJSON(cmd.OutOrStdout(), document)
						return nil
					}
					out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
					fmt.Fprintf(out, "Stream\t%s\nWorker\t%s\nStatus\t%s\n", streamName, snapshot.Name, snapshot.Status)
					fmt.Fprintf(out, "Target instances\t%d\nLive instances\t%d\nConsecutive failures\t%d\nUnclaimed for\t%s\n",
						snapshot.TargetInstances, snapshot.LiveInstances, snapshot.Attempts, snapshot.UnclaimedFor)
					return out.Flush()
				case "suspend":
					err = handle.Suspend(ctx)
				case "unsuspend":
					err = handle.Unsuspend(ctx)
				}
				if err != nil {
					return translateAdminError(err)
				}
				if g.jsonOutput() {
					writeJSON(cmd.OutOrStdout(), maintenanceSuspendedDocument{
						Stream: streamName, Worker: "stream_" + name, Suspended: operation.verb == "suspend",
					})
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s stream %q %s %sed\n", glyphOK(), streamName, name, operation.verb)
				return nil
			},
		})
	}
	return cmd
}

type maintenanceStatusDocument struct {
	*sqlstreams.WorkerSnapshot
	UnclaimedFor string `json:"unclaimed_for"`
}

func newMaintenanceStatusDocument(snapshot *sqlstreams.WorkerSnapshot) (*maintenanceStatusDocument, error) {
	if snapshot == nil {
		return nil, errors.New("snapshot must not be nil")
	}

	return &maintenanceStatusDocument{
		WorkerSnapshot: snapshot,
		UnclaimedFor:   snapshot.UnclaimedFor.String(),
	}, nil
}

// Suspended reports the applied target; live instances may still be stopping.
type maintenanceSuspendedDocument struct {
	Stream    string `json:"stream"`
	Worker    string `json:"worker"`
	Suspended bool   `json:"suspended"`
}
