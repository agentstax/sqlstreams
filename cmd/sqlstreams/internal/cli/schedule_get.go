package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"
	"time"

	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/spf13/cobra"
)

func newScheduleGetCmd(g *globalFlags) *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show a schedule's expression and config",
		Args:  requireScheduleName("get"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			out := cmd.OutOrStdout()
			if quiet && g.jsonOutput() {
				return failUsage("--quiet and --output json cannot be combined")
			}

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			row, err := connection.client.Scheduler(name).Get(ctx)
			if err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() {
				if row == nil {
					writeJSON(out, nil)
				} else {
					writeJSON(out, toScheduleDocument(row))
				}
			} else if !quiet {
				if row == nil {
					fmt.Fprintf(out, "%s schedule %q does not exist\n", glyphNo(), name)
				} else {
					fmt.Fprintf(out, "%s schedule %q (id=%d)\n", glyphOK(), name, row.Id)
					printScheduleDetail(out, row)
				}
			}
			if row == nil {
				return failPrinted()
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "no output; exit code is the answer (0 exists, 1 not)")
	return cmd
}

// scheduleDocument is one schedule's json shape -- the get-shape every
// schedule-echoing command shares. Durations render with units.
type scheduleDocument struct {
	ScheduleId      int64           `json:"schedule_id"`
	SystemId        int64           `json:"system_id"`
	StreamId        int64           `json:"stream_id"`
	Schedule        string          `json:"schedule"`
	Expression      string          `json:"expression"`
	SchemaVersion   int             `json:"schema_version"`
	Concurrency     string          `json:"concurrency"`
	Timeout         string          `json:"timeout"`
	Suspended       bool            `json:"suspended"`
	Payload         json.RawMessage `json:"payload"`
	Metadata        json.RawMessage `json:"metadata"`
	NextScheduledAt time.Time       `json:"next_scheduled_at"`
	LastScheduledAt *time.Time      `json:"last_scheduled_at"` // null until the scheduler first produces the schedule
}

func toScheduleDocument(row *schedule.Schedule) scheduleDocument {
	return scheduleDocument{
		ScheduleId:      row.Id,
		SystemId:        row.SystemId,
		StreamId:        row.StreamId,
		Schedule:        row.Name,
		Expression:      row.Expression,
		SchemaVersion:   row.SchemaVersion,
		Concurrency:     string(row.Concurrency),
		Timeout:         row.Timeout.String(),
		Suspended:       row.Suspended,
		Payload:         row.Payload,
		Metadata:        row.Metadata,
		NextScheduledAt: row.NextScheduledAt,
		LastScheduledAt: row.LastScheduledAt,
	}
}

func toScheduleDocuments(schedules []*schedule.Schedule) []scheduleDocument {
	documents := make([]scheduleDocument, 0, len(schedules))
	for _, row := range schedules {
		documents = append(documents, toScheduleDocument(row))
	}
	return documents
}

func printScheduleDetail(w io.Writer, row *schedule.Schedule) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  Expression\t%s\n", row.Expression)
	fmt.Fprintf(tw, "  Concurrency\t%s\n", row.Concurrency)
	fmt.Fprintf(tw, "  Timeout\t%s\n", row.Timeout)
	fmt.Fprintf(tw, "  Suspended\t%t\n", row.Suspended)
	fmt.Fprintf(tw, "  StreamId\t%d\n", row.StreamId)
	fmt.Fprintf(tw, "  Payload\t%s\n", row.Payload)
	fmt.Fprintf(tw, "  Metadata\t%s\n", row.Metadata)
	fmt.Fprintf(tw, "  NextScheduledAt\t%s\n", scheduleNextCell(row))
	fmt.Fprintf(tw, "  LastScheduledAt\t%s\n", scheduleLastCell(row))
	tw.Flush()
}

// printScheduleStatuses is one line per consumer group whose binding matches
// the schedule's name -- message outcomes over the target stream's retention window.
func printScheduleStatuses(w io.Writer, statuses []*schedule.ScheduleConsumerGroupSummary) {
	fmt.Fprintln(w)
	if len(statuses) == 0 {
		fmt.Fprintln(w, "  no consumer group is bound to this row's name")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "  GROUP\tRAN\tSUCCEEDED\tFAILED\tSUPERSEDED")
	for _, status := range statuses {
		fmt.Fprintf(tw, "  %s\t%d\t%d\t%d\t%d\n", status.ConsumerGroup, status.Ran, status.Succeeded, status.Failed, status.Superseded)
	}
	tw.Flush()
}

// printScheduleMessages is one line per (message, consumer group), newest
// message first -- messages older than the retention window are gone.
func printScheduleMessages(w io.Writer, statuses []*schedule.ScheduleMessageStatus) {
	fmt.Fprintln(w)
	if len(statuses) == 0 {
		fmt.Fprintln(w, "  no messages in the retention window")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "  MESSAGE\tSCHEDULED\tPRODUCED\tGROUP\tOUTCOME")
	for _, status := range statuses {
		// ScheduledAt is stored in UTC -- render it in the driver's zone like
		// the columns beside it
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\n",
			status.MessageId, timeCell(status.ScheduledAt.Local()), timeCell(status.ProducedAt), status.ConsumerGroup, messageOutcomeCell(status))
	}
	tw.Flush()
}

// messageOutcomeCell names the replacing message inline, where the reader is
// already looking for it.
func messageOutcomeCell(status *schedule.ScheduleMessageStatus) string {
	if status.Outcome == schedule.ScheduleMessageSuperseded && status.SupersededBy != nil {
		return fmt.Sprintf("superseded by %d at %s", *status.SupersededBy, timeCell(*status.SupersededAt))
	}
	return string(status.Outcome)
}

// scheduleNextCell - a suspended schedule's next_scheduled_at is stale by design
// (unsuspend re-seeds it), so show the state instead of a misleading time.
func scheduleNextCell(row *schedule.Schedule) string {
	if row.Suspended {
		return "suspended"
	}
	return timeCell(row.NextScheduledAt)
}

// scheduleLastCell - NULL until the scheduler first produces this schedule.
func scheduleLastCell(row *schedule.Schedule) string {
	if row.LastScheduledAt == nil {
		return "never"
	}
	return timeCell(*row.LastScheduledAt)
}
