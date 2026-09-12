package cli

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"text/tabwriter"

	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/spf13/cobra"
)

func newMetricListCmd(g *globalFlags) *cobra.Command {
	var (
		quiet   bool
		builtin bool
		user    bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the current measurement per (name, attributes) series",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if builtin && user {
				return failUsage("--builtin and --user exclude everything together; pass one or neither")
			}
			if quiet && g.jsonOutput() {
				return failUsage("--quiet and --output json cannot be combined")
			}

			connection, err := newConnection(ctx, g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()
			client := connection.client

			measurements, err := client.System().Metrics().Latest(ctx)
			if err != nil {
				return translateAdminError(err)
			}

			filtered := make([]*metric.Measurement, 0, len(measurements))
			for _, measurement := range measurements {
				isBuiltin := strings.HasPrefix(measurement.Name, metric.MetricNameReservedPrefix)
				if builtin && !isBuiltin {
					continue
				}
				if user && isBuiltin {
					continue
				}
				filtered = append(filtered, measurement)
			}

			if g.jsonOutput() {
				writeJSON(out, filtered)
				return nil
			}

			if quiet {
				printMeasurementKeys(out, filtered)
			} else {
				printMeasurementsTable(out, filtered)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&quiet, "quiet", "q", false, "series keys only, one per line (for scripts)")
	f.BoolVar(&builtin, "builtin", false, "only SQLStreams's built-in metrics (names starting sqlstreams.)")
	f.BoolVar(&user, "user", false, "only user-produced measurements")
	return cmd
}

func printMeasurementKeys(w io.Writer, measurements []*metric.Measurement) {
	for _, measurement := range measurements {
		fmt.Fprintln(w, metric.MeasurementKey(measurement.Name, measurement.Attributes))
	}
}

func printMeasurementsTable(w io.Writer, measurements []*metric.Measurement) {
	if len(measurements) == 0 {
		fmt.Fprintln(w, "no measurements published")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tKIND\tVALUE\tATTRIBUTES\tAT")
	for _, measurement := range measurements {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			measurement.Name, measurement.Kind, measurementValueCell(measurement),
			measurementAttributesCell(measurement.Attributes), timeCell(measurement.At.Local()))
	}
	tw.Flush()

	fmt.Fprintf(w, "\n%d series\n", len(measurements))
}
