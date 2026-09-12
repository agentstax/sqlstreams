package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/spf13/cobra"
)

func newStreamConfigCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read a registered stream's config",
		Long: `Stream config comes from the cfg your code passes to Stream(name).Register, and
is applied every time that runs. Changing a value means changing that code
and redeploying; this command only reads.

Running producers and consumers read stream config once, at their Register,
so a change takes effect on their next restart, not live.`,
	}

	cmd.AddCommand(newStreamConfigGetCmd(g))

	return cmd
}

// streamConfigKey is one key of a stream's config: the column name config get
// prints, its value shape for help and errors, and how get reads it off a
// stream.
type streamConfigKey struct {
	key   string
	value string
	read  func(found *stream.Stream) string
}

// streamConfigKeys holds the keys config get prints. PartitionSize is absent --
// it is fixed at creation and shown by stream get.
var streamConfigKeys = []streamConfigKey{
	{
		key:   "retention_ttl",
		value: "duration, e.g. 720h (0 keeps messages forever)",
		read: func(found *stream.Stream) string {
			return retentionDetail(found.RetentionTTL)
		},
	},
	{
		key:   "allow_drop_past_committed",
		value: "true or false",
		read: func(found *stream.Stream) string {
			return strconv.FormatBool(found.AllowDropPastCommitted)
		},
	},
	{
		key:   "idempotency_key_ttl",
		value: "duration, e.g. 1h",
		read: func(found *stream.Stream) string {
			return found.IdempotencyKeyTTL.String()
		},
	},
	{
		key:   "empty_compaction_head_ttl",
		value: "duration, e.g. 1h",
		read: func(found *stream.Stream) string {
			return found.EmptyCompactionHeadTTL.String()
		},
	},
	{
		key:   "delivery_log_mode",
		value: "off, failures, or all",
		read: func(found *stream.Stream) string {
			return string(found.DeliveryLogMode)
		},
	},
}

func findStreamConfigKey(key string) (streamConfigKey, bool) {
	for _, entry := range streamConfigKeys {
		if entry.key == key {
			return entry, true
		}
	}
	return streamConfigKey{}, false
}

// errUnknownStreamConfigKey rejects a key the table doesn't know, listing the
// vocabulary.
func errUnknownStreamConfigKey(key string) error {
	var known []string
	for _, entry := range streamConfigKeys {
		known = append(known, fmt.Sprintf("  %-26s %s", entry.key, entry.value))
	}
	return failUsage("unknown config key %q -- known keys:\n%s", key, strings.Join(known, "\n"))
}

// streamConfigDocument is stream config get's json result: each key with its
// compiled-in default and the stream's current value, as the table renders
// them.
type streamConfigDocument struct {
	Stream   string                    `json:"stream"`
	StreamId int64                     `json:"stream_id"`
	Keys     []streamConfigKeyDocument `json:"keys"`
}

type streamConfigKeyDocument struct {
	Key     string `json:"key"`
	Default string `json:"default"`
	Value   string `json:"value"`
}

func toStreamConfigDocument(found *stream.Stream, entries []streamConfigKey) streamConfigDocument {
	defaults := (&stream.StreamConfig{}).WithDefaults().ToStream(0, 0, "")

	keys := make([]streamConfigKeyDocument, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, streamConfigKeyDocument{
			Key:     entry.key,
			Default: entry.read(defaults),
			Value:   entry.read(found),
		})
	}
	return streamConfigDocument{
		Stream:   found.Name,
		StreamId: found.Id,
		Keys:     keys,
	}
}

func printStreamConfigLines(w io.Writer, found *stream.Stream, entries []streamConfigKey) {
	defaults := (&stream.StreamConfig{}).WithDefaults().ToStream(0, 0, "")
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "  KEY\tDEFAULT\tVALUE")
	for _, entry := range entries {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", entry.key, entry.read(defaults), entry.read(found))
	}
	tw.Flush()
}

// retentionDetail is the retention cell: raw Go duration string, plus a day
// parenthetical when it's whole days ("720h0m0s (30d)"); "forever" for
// keep-indefinitely.
func retentionDetail(retention time.Duration) string {
	if retention == 0 {
		return "forever"
	}
	const day = 24 * time.Hour
	if retention%day == 0 {
		return fmt.Sprintf("%s (%dd)", retention, retention/day)
	}
	return retention.String()
}
