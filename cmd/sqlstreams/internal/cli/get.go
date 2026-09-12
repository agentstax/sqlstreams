package cli

import (
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	"github.com/agentstax/sqlstreams/client"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/spf13/cobra"
)

func newStreamGetCmd(g *globalFlags) *cobra.Command {
	var quiet bool

	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show a stream's registration and config",
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
				writeJSON(out, toStreamGetDocument(name, nil))
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

			if g.jsonOutput() {
				writeJSON(out, toStreamGetDocument(name, found))
				return nil
			}

			fmt.Fprintf(out, "%s stream %q (id=%d)\n", glyphOK(), name, found.Id)
			printStreamDetail(out, found)
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
// (exists false, config null), the exit code stays 1.
type streamGetDocument struct {
	Stream string          `json:"stream"`
	Exists bool            `json:"exists"`
	Config *streamDocument `json:"config"`
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

func toStreamGetDocument(name string, found *stream.Stream) streamGetDocument {
	document := streamGetDocument{Stream: name, Exists: found != nil}
	if found != nil {
		config := toStreamDocument(found)
		document.Config = &config
	}
	return document
}

func printStreamDetail(w io.Writer, t *stream.Stream) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  PartitionSize\t%s\n", commaInt(t.PartitionSize))
	fmt.Fprintf(tw, "  RetentionTTL\t%s\n", retentionDetail(t.RetentionTTL))
	fmt.Fprintf(tw, "  AllowDropPastCommitted\t%t\n", t.AllowDropPastCommitted)
	fmt.Fprintf(tw, "  IdempotencyKeyTTL\t%s\n", t.IdempotencyKeyTTL)
	fmt.Fprintf(tw, "  EmptyCompactionHeadTTL\t%s\n", t.EmptyCompactionHeadTTL)
	fmt.Fprintf(tw, "  DeliveryLogMode\t%s\n", t.DeliveryLogMode)
	tw.Flush()
}
