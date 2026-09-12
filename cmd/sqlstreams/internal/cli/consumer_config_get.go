package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	sqlstreams "github.com/agentstax/sqlstreams/client"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/spf13/cobra"
)

func newConsumerConfigGetCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <stream> <consumer> [key]",
		Short: "Show the consumer's config",
		Long: `Show each config key the consumer's workers declare, and its stored
value. Pass a key to show just that key; message shows one line per
field.`,
		Example: `  sqlstreams consumer config get orders billing
  sqlstreams consumer config get orders billing claim_poll_rate
  sqlstreams consumer config get orders billing message`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streamName, consumerName := args[0], args[1]
			key := ""
			if len(args) == 3 {
				key = args[2]
			}
			out := cmd.OutOrStdout()

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			workers, err := client.Stream[sqlstreams.RawPayload](streamName).Consumer(consumerName).Workers(ctx)
			if err != nil {
				return consumerError(streamName, consumerName, err)
			}

			lines := consumerConfigLines(workers)
			if key != "" && len(lines) > 0 {
				lines = filterConsumerConfigLines(lines, key)
				if len(lines) == 0 {
					return failOp("no worker of consumer %q declares config key %q", consumerName, key)
				}
			}

			if g.jsonOutput() {
				writeJSON(out, toConsumerConfigDocument(streamName, consumerName, lines))
				return nil
			}

			if len(lines) == 0 {
				fmt.Fprintf(out, "%s consumer %q on stream %q\n", glyphOK(), consumerName, streamName)
				fmt.Fprintln(out, "  (no config -- worker rows appear at the consumer's first Consume)")
				return nil
			}

			fmt.Fprintf(out, "%s consumer %q on stream %q\n", glyphOK(), consumerName, streamName)
			printConsumerConfigLines(out, lines)
			return nil
		},
	}

	return cmd
}

// consumerConfigLine is one config key on one worker row, ready to print.
type consumerConfigLine struct {
	key    string
	worker string
	value  string
}

// consumerConfigDocument is consumer config get's json result: each declared key
// with the worker row it came from, as the table renders them.
type consumerConfigDocument struct {
	Stream   string                       `json:"stream"`
	Consumer string                       `json:"consumer"`
	Keys     []consumerConfigLineDocument `json:"keys"`
}

type consumerConfigLineDocument struct {
	Key    string `json:"key"`
	Worker string `json:"worker"`
	Value  string `json:"value"`
}

func toConsumerConfigDocument(streamName string, consumerName string, lines []consumerConfigLine) consumerConfigDocument {
	keys := make([]consumerConfigLineDocument, 0, len(lines))
	for _, line := range lines {
		keys = append(keys, consumerConfigLineDocument{Key: line.key, Worker: line.worker, Value: line.value})
	}
	return consumerConfigDocument{Stream: streamName, Consumer: consumerName, Keys: keys}
}

// consumerConfigLines flattens the rows' metadata into print lines: one per key,
// and one per message field so the KEY column names the field itself.
func consumerConfigLines(workers []*worker.Worker) []consumerConfigLine {
	var lines []consumerConfigLine
	for _, row := range workers {
		metadata, ok := row.Metadata.(map[string]any)
		if !ok {
			continue
		}
		for key, value := range metadata {
			if key == "message" {
				lines = append(lines, messageConfigLines(row.Name, value)...)
				continue
			}
			lines = append(lines, consumerConfigLine{
				key:    key,
				worker: row.Name,
				value:  formatMetadataValue(key, value),
			})
		}
	}
	slices.SortFunc(lines, func(a, b consumerConfigLine) int {
		if c := strings.Compare(a.key, b.key); c != 0 {
			return c
		}
		return strings.Compare(a.worker, b.worker)
	})
	return lines
}

// messageConfigLines is one row's message document expanded to a line per
// field. A document that doesn't decode prints as raw JSON rather than
// dropping out of the table.
func messageConfigLines(workerName string, document any) []consumerConfigLine {
	options, err := decodeMessageOptions(document)
	if err != nil {
		return []consumerConfigLine{{
			key:    "message",
			worker: workerName,
			value:  formatMetadataValue("message", document),
		}}
	}

	var lines []consumerConfigLine
	for _, field := range messageFieldKeys {
		lines = append(lines, consumerConfigLine{
			key:    field.path,
			worker: workerName,
			value:  field.read(options),
		})
	}
	return lines
}

// filterConsumerConfigLines keeps one key's lines -- message matches all its
// fields.
func filterConsumerConfigLines(lines []consumerConfigLine, key string) []consumerConfigLine {
	var kept []consumerConfigLine
	for _, line := range lines {
		if line.key == key || strings.HasPrefix(line.key, key+".") {
			kept = append(kept, line)
		}
	}
	return kept
}

func printConsumerConfigLines(w io.Writer, lines []consumerConfigLine) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "  KEY\tWORKER\tVALUE")
	for _, line := range lines {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", line.key, line.worker, cellOrDash(line.value))
	}
	tw.Flush()
}

// formatMetadataValue renders one metadata value for the table -- a duration
// key arrives from JSONB as float64 nanoseconds, indistinguishable from a
// plain count, so the key's name is what tells them apart.
func formatMetadataValue(key string, value any) string {
	if value == nil {
		return ""
	}
	if nanoseconds, ok := value.(float64); ok && isDurationKey(key) {
		return time.Duration(int64(nanoseconds)).String()
	}
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(raw)
}

// durationKeySuffixes are how every worker kind names a time.Duration field:
// poll_rate, repeat_interval, exception_initial_backoff. A kind naming one
// some other way prints its raw nanoseconds until the name joins this list.
var durationKeySuffixes = []string{"_rate", "_interval", "_backoff", "_ttl", "_timeout", "_delay", "_margin"}

func isDurationKey(key string) bool {
	for _, suffix := range durationKeySuffixes {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

func cellOrDash(cell string) string {
	if cell == "" {
		return "-"
	}
	return cell
}

// durationCell renders a decoded duration field; zero means absent.
func durationCell(duration time.Duration) string {
	if duration == 0 {
		return ""
	}
	return duration.String()
}

// intCell renders a decoded int field; zero means absent.
func intCell(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}
