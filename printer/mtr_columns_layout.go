package printer

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/rodaine/table"
)

// MTRColumnEditor is an immutable frame snapshot supplied by the TUI controller.
type MTRColumnEditor struct {
	Active       bool
	Draft, Error string
}

func renderMTRColumnEditor(b *strings.Builder, header MTRTUIHeader, width, height int) {
	editor := header.ColumnEditor
	prefix := "Fields: "
	if width < len(prefix)+2 {
		prefix = ""
	}
	available := max(1, width-len(prefix)-1)
	draft := editor.Draft
	if len(draft) > available {
		draft = draft[len(draft)-available:]
	}
	fields := prefix + draft + "_"
	// Leave the last terminal row unused: CRLF there would scroll the page.
	remaining := max(0, height-1)
	line := func(text string) {
		if remaining > 0 {
			tuiLine(b, "%s", text)
			remaining--
		}
	}
	// Header builders already fit visible width before adding ANSI colors.
	if height >= 9 {
		line(buildMTRTUITitleLine(header, width))
		if width < 40 {
			line(truncateByDisplayWidth(buildMTRTUIRouteText(header), width))
		} else {
			line(buildMTRTUIRouteLine(header, width, mtrTUIClock(header)))
		}
	}
	line(fields)
	if editor.Error != "" {
		line(truncateByDisplayWidth(editor.Error, width))
	}
	// Keep editing controls visible even when the field list does not fit.
	controls := []string{"Enter: apply Esc: cancel", "Backspace: delete", "Ctrl-U: clear  Ctrl-C: quit"}
	if width >= 72 {
		controls = []string{"Enter: apply  Esc: cancel  Backspace: delete  Ctrl-U: clear  Ctrl-C: quit"}
	}
	order := []MTRColumn{MTRColumnSpace, MTRColumnLoss, MTRColumnDropped, MTRColumnReceived,
		MTRColumnSnt, MTRColumnLast, MTRColumnBest, MTRColumnAvg, MTRColumnWrst,
		MTRColumnStDev, MTRColumnGMean, MTRColumnJitter, MTRColumnJitterAvg,
		MTRColumnJitterMax, MTRColumnJitterInterarrival}
	if remaining >= len(order)+len(controls)+2 {
		line("")
	}
	count := min(len(order), max(0, remaining-len(controls)-1))
	for _, column := range order[:count] {
		def := mtrColumnDefinitions[column]
		code := string(def.code)
		if column == MTRColumnSpace {
			code = "<sp>"
		}
		line(truncateByDisplayWidth("  "+code+": "+def.description, width))
	}
	if count < len(order) && remaining > len(controls) {
		line(truncateByDisplayWidth("Enlarge terminal for help", width))
	}
	for _, help := range controls {
		line(truncateByDisplayWidth(help, width))
	}
}

func renderMTRSelectedColumns(b *strings.Builder, header MTRTUIHeader, stats []trace.MTRHopStat, width int) {
	columns := header.Columns
	widths := mtrColumnWidths(columns, stats)
	maxTTL, _ := scanMTRTUIStats(stats)
	prefixW := max(tuiPrefixW, len(fmt.Sprint(maxTTL))+2)
	metricsW := len(mtrSelectedMetrics(columns, widths, nil, false))
	hostW := width - prefixW - 1 - metricsW
	if hostW < tuiHostMin {
		for _, line := range []string{"Select fewer columns", "or widen terminal (O)."} {
			tuiLine(b, "%s", truncateByDisplayWidth(line, width))
		}
		return
	}
	prefix := strings.Repeat(" ", prefixW)
	if isDefaultMTRColumns(columns) {
		packetsW := widths[0] + 1 + widths[1]
		tuiLine(b, "%s", prefix+padRight("Host", hostW)+" "+mtrTUIHeaderColor(centerIn("Packets", packetsW)+" "+centerIn("Pings", metricsW-packetsW-1)))
		tuiLine(b, "%s", prefix+strings.Repeat(" ", hostW)+" "+mtrTUIHeaderColor(mtrSelectedMetrics(columns, widths, nil, false)))
	} else {
		tuiLine(b, "%s", prefix+mtrTUIHeaderColor(padRight("Host", hostW)+" "+mtrSelectedMetrics(columns, widths, nil, false)))
	}
	parts := buildTUIHostPartSet(stats, header)
	asnW := computeTUIASNWidthFromParts(parts)
	prevTTL := 0
	for i, stat := range stats {
		host := fitMTRHostWithMarker(formatTUIHost(parts[i], asnW), mtrResponseMarker(stat), hostW)
		hostStyle := mtrTUIHostColor
		if isWaitingHopStat(stat) {
			hostStyle = mtrTUIWaitColor
		}
		tuiLine(b, "%s", mtrTUIHopColor(formatTUIHopPrefix(stat.TTL, prevTTL, prefixW))+hostStyle(host)+" "+mtrSelectedMetrics(columns, widths, &stat, true))
		renderMTRTUIMPLSRows(b, mtrTUILayout{prefixW: prefixW, hostW: hostW}, stat.MPLS, header.DisableMPLS)
		prevTTL = stat.TTL
	}
}

// MTRTablePrinterWithColumns preserves Hop/Host placement and the default wrapper.
func MTRTablePrinterWithColumns(stats []trace.MTRHopStat, iteration, mode, nameMode int, lang string, showIPs bool, columns []MTRColumn) {
	if columns == nil {
		MTRTablePrinter(stats, iteration, mode, nameMode, lang, showIPs)
		return
	}
	fmt.Print("\033[H\033[2J")
	headers := []any{"Hop"}
	headers = append(headers, mtrTableColumnCells(columns, nil)...)
	headers = append(headers, "Host")
	tbl := table.New(headers...).WithWriter(os.Stdout)
	tbl.WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())
	prevTTL := 0
	for _, s := range stats {
		hop := fmt.Sprint(s.TTL)
		if s.TTL == prevTTL {
			hop = ""
		}
		prevTTL = s.TTL
		row := []any{hop}
		row = append(row, mtrTableColumnCells(columns, &s)...)
		row = append(row, appendMTRResponseMarker(formatMTRHostWithMPLS(s, mode, nameMode, lang, showIPs), s))
		tbl.AddRow(row...)
	}
	tbl.Print()
}

func printMTRSelectedReport(stats []trace.MTRHopStat, opts MTRReportOptions, hosts []string, hostW int) {
	widths := mtrColumnWidths(opts.Columns, stats)
	hostHeader := opts.SrcHost
	if !opts.Wide {
		hostHeader = reportTruncateToWidth(hostHeader, hostW)
	}
	fmt.Printf("HOST: %s %s\n", reportPadRight(hostHeader, hostW), mtrSelectedMetrics(opts.Columns, widths, nil, false))
	prevTTL := 0
	for i, s := range stats {
		fmt.Printf("%s%s %s\n", mtrReportPrefix(s.TTL, prevTTL), reportPadRight(hosts[i], hostW), mtrSelectedMetrics(opts.Columns, widths, &s, false))
		prevTTL = s.TTL
	}
}

// Keep custom frames within the terminal even when the legacy shortcut bar
// would wrap over the data. O remains visible in the narrowest supported view.
func renderMTRSelectedHeader(b *strings.Builder, header MTRTUIHeader, width int) {
	tuiLine(b, "%s", buildMTRTUITitleLine(header, width))
	if width < 40 {
		tuiLine(b, "%s", truncateByDisplayWidth(buildMTRTUIRouteText(header), width))
	} else {
		tuiLine(b, "%s", buildMTRTUIRouteLine(header, width, mtrTUIClock(header)))
	}
	if header.Replay != nil {
		tuiLine(b, "%s", buildMTRReplayControls(header, width))
		return
	}
	if width >= 160 {
		tuiLine(b, "%s", buildMTRTUIControlsLine(header, width))
	} else {
		tuiLine(b, "%s", truncateByDisplayWidth("O:cols Q:quit "+mtrTUIStatusText(header.Status), width))
	}
}

// table applies two spaces of padding per cell. Attach explicit spacers to real
// cells so each requested space adds exactly one character, not another column.
func mtrTableColumnCells(columns []MTRColumn, stat *trace.MTRHopStat) []any {
	var cells []any
	spaces := ""
	for _, c := range columns {
		if c == MTRColumnSpace {
			spaces += " "
			continue
		}
		value := mtrColumnDefinitions[c].title
		if stat != nil {
			value = mtrColumnValue(c, *stat)
		}
		cells = append(cells, spaces+value)
		spaces = ""
	}
	if spaces != "" && len(cells) > 0 {
		last := len(cells) - 1
		cells[last] = cells[last].(string) + spaces
	}
	return cells
}
