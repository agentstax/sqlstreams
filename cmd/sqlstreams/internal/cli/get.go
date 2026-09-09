package cli

import (
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/spf13/cobra"
)

func newStreamGetCmd(g *globalFlags) *cobra.Command {
	var quiet bool

	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show a stream, every payload version in its log, and each version's retire state",
		Args:  requireStreamName("get"),
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
			client := connection.client

			found, err := client.Stream[sqlstreams.RawPayload](name).Get(ctx)
			if err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() && found == nil {
				writeJSON(out, toStreamGetDocument(name, nil, nil))
				return failPrinted()
			}

			// -q is the scriptable form: no output at all, the exit code IS the
			// answer (`if sqlstreams stream get -q X; then ...`).
			if quiet {
				if found == nil {
					return failPrinted()
				}
				return nil
			}

			if found == nil {
				fmt.Fprintf(out, "%s stream %q does not exist\n", glyphNo(), name)
				return failPrinted()
			}

			health, err := client.Stream[sqlstreams.RawPayload](name).Health(ctx)
			if err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() {
				writeJSON(out, toStreamGetDocument(name, found, health))
				return nil
			}

			fmt.Fprintf(out, "%s stream %q -- %s\n", glyphOK(), name, pluralize(len(health), "payload version"))
			printStreamDetail(out, found)
			for _, h := range health {
				fmt.Fprintln(out)
				printVersionHealth(out, h)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&quiet, "quiet", "q", false, "no output; exit code is the answer (0 exists, 1 not)")
	return cmd
}

// streamDocument is one stream row's json shape -- the get-shape every
// stream-echoing command shares. Durations render with units.
type streamDocument struct {
	StreamId               int64  `json:"stream_id"`
	SystemId               int64  `json:"system_id"`
	Stream                 string `json:"stream"`
	PartitionSize          int64  `json:"partition_size"`
	RetentionTTL           string `json:"retention_ttl"` // "0s" keeps messages forever
	AllowDropPastCommitted bool   `json:"allow_drop_past_committed"`
	IdempotencyKeyTTL      string `json:"idempotency_key_ttl"`
	EmptyCompactionHeadTTL string `json:"empty_compaction_head_ttl"`
	DeliveryLogMode        string `json:"delivery_log_mode"`
}

// streamGetDocument is stream get's json result; the not-found case is data
// (exists false, stream null, versions empty), the exit code stays 1.
type streamGetDocument struct {
	Stream   string                  `json:"stream"`
	Exists   bool                    `json:"exists"`
	Config   *streamDocument         `json:"config"`
	Versions []versionHealthDocument `json:"versions"`
}

// versionHealthDocument is one payload version present in the log with its
// retire verdict.
type versionHealthDocument struct {
	Version         int64                     `json:"version"`
	Messages        int64                     `json:"messages"`
	CompactionHeads int64                     `json:"compaction_heads"`
	Groups          []groupVersionLagDocument `json:"groups"`
	Safe            bool                      `json:"safe"`
	Reason          string                    `json:"reason"`
}

type groupVersionLagDocument struct {
	Group                string `json:"group"`
	Unconsumed           int64  `json:"unconsumed"`
	UnresolvedExceptions int64  `json:"unresolved_exceptions"`
}

func toStreamDocument(found *stream.Stream) streamDocument {
	return streamDocument{
		StreamId:               found.Id,
		SystemId:               found.SystemId,
		Stream:                 found.Name,
		PartitionSize:          found.PartitionSize,
		RetentionTTL:           found.RetentionTTL.String(),
		AllowDropPastCommitted: found.AllowDropPastCommitted,
		IdempotencyKeyTTL:      found.IdempotencyKeyTTL.String(),
		EmptyCompactionHeadTTL: found.EmptyCompactionHeadTTL.String(),
		DeliveryLogMode:        string(found.DeliveryLogMode),
	}
}

func toStreamDocuments(streams []*stream.Stream) []streamDocument {
	documents := make([]streamDocument, 0, len(streams))
	for _, found := range streams {
		documents = append(documents, toStreamDocument(found))
	}
	return documents
}

func toStreamGetDocument(name string, found *stream.Stream, health []*sqlstreams.StreamVersionHealth) streamGetDocument {
	document := streamGetDocument{Stream: name, Exists: found != nil, Versions: make([]versionHealthDocument, 0, len(health))}
	if found != nil {
		config := toStreamDocument(found)
		document.Config = &config
	}
	for _, versionHealth := range health {
		document.Versions = append(document.Versions, toVersionHealthDocument(versionHealth))
	}
	return document
}

func toVersionHealthDocument(versionHealth *sqlstreams.StreamVersionHealth) versionHealthDocument {
	groups := make([]groupVersionLagDocument, 0, len(versionHealth.Groups))
	for _, group := range versionHealth.Groups {
		groups = append(groups, groupVersionLagDocument{
			Group:                group.ConsumerGroup,
			Unconsumed:           group.Unconsumed,
			UnresolvedExceptions: group.UnresolvedExceptions,
		})
	}
	return versionHealthDocument{
		Version:         int64(versionHealth.Version),
		Messages:        versionHealth.Messages,
		CompactionHeads: versionHealth.CompactionHeads,
		Groups:          groups,
		Safe:            versionHealth.Safe,
		Reason:          versionHealth.Reason,
	}
}

// printStreamDetail shows the columns fixed at creation; the declared config
// lives under stream config get.
func printStreamDetail(w io.Writer, t *stream.Stream) {
	fmt.Fprintf(w, "\n(id=%d)\n", t.Id)

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  PartitionSize\t%s\n", commaInt(t.PartitionSize))
	tw.Flush()
}

// printVersionHealth is one payload version's picture: how many rows sit at
// it, how many compaction heads point at it, each group's lag against it,
// and the resulting retire verdict.
func printVersionHealth(w io.Writer, h *sqlstreams.StreamVersionHealth) {
	fmt.Fprintf(w, "  v%d\n", h.Version)

	ctw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(ctw, "    Messages\t%s\n", commaInt(h.Messages))
	fmt.Fprintf(ctw, "    CompactionHeads\t%s\n", commaInt(h.CompactionHeads))
	ctw.Flush()

	if len(h.Groups) > 0 {
		fmt.Fprintln(w)
		tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "    GROUP\tUNCONSUMED\tUNRESOLVED")
		for _, group := range h.Groups {
			fmt.Fprintf(tw, "    %s\t%s\t%d\n", group.ConsumerGroup, commaInt(group.Unconsumed), group.UnresolvedExceptions)
		}
		tw.Flush()
	}

	fmt.Fprintln(w)
	verdict := glyphNo()
	if h.Safe {
		verdict = glyphOK()
	}
	fmt.Fprintf(w, "    retire: %s %s\n", verdict, h.Reason)
}
