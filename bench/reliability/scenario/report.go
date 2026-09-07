package scenario

import (
	"fmt"
	"strings"
)

// Report is String with each [expect] line extended by the columns in
// beside, keyed by check -- "lost\t0\tactual 0\tPASS". A check with no
// entry prints as in String.
func (s *Scenario) Report(beside map[Check]string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n", s.Summary)
	writeSection(&out, "[input]", s.inputLines())
	writeSection(&out, "[shape]", s.producerLines())
	writeSection(&out, "", s.consumerLines())
	writeSection(&out, "[expect]", s.expectLines(beside))
	return out.String()
}
