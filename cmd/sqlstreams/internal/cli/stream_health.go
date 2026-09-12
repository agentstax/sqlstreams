package cli

import (
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	"github.com/allegedlyreliable/sqlstreams/client"
	"github.com/spf13/cobra"
)

func newStreamHealthCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "health <name>",
		Short: "Show each payload version's retirement verdict",
		Args:  requireStreamName("health"),
		RunE: func(cmd *cobra.Command, args []string) error {
			connection, err := newConnection(cmd.Context(), g.databaseURL, g.schema, slog.LevelError)
			if err != nil {
				return err
			}
			defer connection.Close()

			health, err := connection.client.Stream[sqlstreams.RawPayload](args[0]).Health(cmd.Context())
			if err != nil {
				return translateAdminError(err)
			}

			if g.jsonOutput() {
				versions := make([]versionHealthDocument, 0, len(health))
				for _, version := range health {
					versions = append(versions, toVersionHealthDocument(version))
				}
				writeJSON(cmd.OutOrStdout(), versions)
				return nil
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s stream %q -- %s\n", glyphOK(), args[0], pluralize(len(health), "payload version"))
			for _, version := range health {
				fmt.Fprintln(out)
				printVersionHealth(out, version)
			}
			return nil
		},
	}
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
