package printer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nxtrace/NTrace-core/trace"
)

// MTRColumn selects a human-readable statistic; it never changes probe data.
type MTRColumn uint8

const (
	MTRColumnLoss MTRColumn = iota
	MTRColumnSnt
	MTRColumnReceived
	MTRColumnLast
	MTRColumnAvg
	MTRColumnBest
	MTRColumnWrst
	MTRColumnStDev
	MTRColumnDropped
	MTRColumnGMean
	MTRColumnJitter
	MTRColumnJitterAvg
	MTRColumnJitterMax
	MTRColumnJitterInterarrival
	MTRColumnSpace
)

var mtrColumnDefinitions = [...]struct {
	name, title string
	code        byte
	description string
}{
	{"loss", "Loss%", 'L', "Loss Ratio"},
	{"snt", "Snt", 'S', "Sent Packets"},
	{"received", "Rcv", 'R', "Received Packets"},
	{"last", "Last", 'N', "Newest RTT(ms)"},
	{"avg", "Avg", 'A', "Average RTT(ms)"},
	{"best", "Best", 'B', "Min/Best RTT(ms)"},
	{"wrst", "Wrst", 'W', "Max/Worst RTT(ms)"},
	{"stdev", "StDev", 'V', "Standard Deviation"},
	{"dropped", "Drop", 'D', "Dropped Packets"},
	{"gmean", "Gmean", 'G', "Geometric Mean"},
	{"jitter", "Jttr", 'J', "Current Jitter"},
	{"javg", "Javg", 'M', "Jitter Mean/Avg."},
	{"jmax", "Jmax", 'X', "Worst Jitter"},
	{"jint", "Jint", 'I', "Interarrival Jitter"},
	{"space", "", ' ', "Space between fields"},
}

func DefaultMTRColumns() []MTRColumn {
	return []MTRColumn{MTRColumnLoss, MTRColumnSnt, MTRColumnLast, MTRColumnAvg, MTRColumnBest, MTRColumnWrst, MTRColumnStDev}
}

// ParseMTRColumns accepts comma-separated CLI names or TUI codes; spaces add display spacing.
func ParseMTRColumns(input string, codes bool) ([]MTRColumn, error) {
	tokens := strings.Split(strings.ToLower(input), ",")
	if codes {
		tokens = nil
		for _, c := range strings.ToUpper(input) {
			tokens = append(tokens, string(c))
		}
	}
	var columns []MTRColumn
	for _, token := range tokens {
		if !codes {
			token = strings.TrimSpace(token)
		}
		if token == "" {
			return nil, fmt.Errorf("empty MTR column entry")
		}
		found := false
		for i, def := range mtrColumnDefinitions {
			if (!codes && token == def.name) || (codes && token == string(def.code)) {
				columns = append(columns, MTRColumn(i))
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown MTR column %q", token)
		}
	}
	if err := ValidateMTRColumns(columns); err != nil {
		return nil, err
	}
	return columns, nil
}

func ValidateMTRColumns(columns []MTRColumn) error {
	if len(columns) == 0 {
		return fmt.Errorf("select at least one MTR column")
	}
	seen := make(map[MTRColumn]bool)
	for _, c := range columns {
		if int(c) >= len(mtrColumnDefinitions) {
			return fmt.Errorf("unknown MTR column %d", c)
		}
		if c == MTRColumnSpace {
			continue
		}
		if seen[c] {
			return fmt.Errorf("duplicate MTR column %s", mtrColumnDefinitions[c].name)
		}
		seen[c] = true
	}
	if len(seen) == 0 {
		return fmt.Errorf("select at least one MTR column")
	}
	return nil
}

func MTRColumnCodes(columns []MTRColumn) string {
	var b strings.Builder
	for _, c := range columns {
		if int(c) < len(mtrColumnDefinitions) {
			b.WriteByte(mtrColumnDefinitions[c].code)
		}
	}
	return b.String()
}

func isDefaultMTRColumns(columns []MTRColumn) bool { return slices.Equal(columns, DefaultMTRColumns()) }

func mtrColumnValue(c MTRColumn, s trace.MTRHopStat) string {
	if isWaitingHopStat(s) {
		return ""
	}
	switch c {
	case MTRColumnLoss:
		return formatLoss(s.Loss)
	case MTRColumnSnt:
		return fmt.Sprint(s.Snt)
	case MTRColumnReceived:
		return fmt.Sprint(s.Received)
	case MTRColumnLast:
		return formatMs(s.Last)
	case MTRColumnAvg:
		return formatMs(s.Avg)
	case MTRColumnBest:
		return formatMs(s.Best)
	case MTRColumnWrst:
		return formatMs(s.Wrst)
	case MTRColumnStDev:
		return formatMs(s.StDev)
	case MTRColumnDropped:
		return fmt.Sprint(s.Dropped)
	case MTRColumnGMean:
		return formatMs(s.GMean)
	case MTRColumnJitter:
		return formatMs(s.Jitter)
	case MTRColumnJitterAvg:
		return formatMs(s.JitterAvg)
	case MTRColumnJitterMax:
		return formatMs(s.JitterMax)
	case MTRColumnJitterInterarrival:
		return formatMs(s.JitterInterarrival)
	default:
		return ""
	}
}

func mtrColumnWidths(columns []MTRColumn, stats []trace.MTRHopStat) []int {
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(mtrColumnDefinitions[c].title)
		for _, s := range stats {
			widths[i] = max(widths[i], len(mtrColumnValue(c, s)))
		}
	}
	return widths
}

func mtrSelectedMetrics(columns []MTRColumn, widths []int, stat *trace.MTRHopStat, colorize bool) string {
	cells := make([]string, len(columns))
	for i, c := range columns {
		value := mtrColumnDefinitions[c].title
		if stat != nil {
			value = mtrColumnValue(c, *stat)
		}
		cells[i] = padLeft(value, widths[i])
		if colorize && stat != nil && (c == MTRColumnLoss || c == MTRColumnSnt || c == MTRColumnReceived || c == MTRColumnDropped) {
			cells[i], _ = mtrColorPacketsByLoss(cells[i], "", stat.Loss, isWaitingHopStat(*stat))
		}
	}
	return strings.Join(cells, " ")
}
