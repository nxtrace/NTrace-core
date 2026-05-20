package printer

import (
	"strings"

	"github.com/fatih/color"
)

var (
	mtrTUITitleColor  = color.New(color.FgHiWhite).SprintFunc()
	mtrTUIHeaderColor = color.New(color.FgHiWhite).SprintFunc()
	mtrTUIRouteColor  = func(s string) string { return s }
	mtrTUITimeColor   = func(s string) string { return s }
	mtrTUIKeyColor    = func(s string) string { return s }
	mtrTUIKeyHiColor  = color.New(color.FgHiWhite).SprintFunc()
	mtrTUIStatusColor = color.New(color.FgHiYellow, color.Bold).SprintFunc()

	mtrTUIHopColor  = color.New(color.FgHiCyan, color.Bold).SprintFunc()
	mtrTUIHostColor = color.New(color.FgHiWhite).SprintFunc()
	mtrTUIMPLSColor = color.New(color.FgHiBlack).SprintFunc()
	mtrTUIWaitColor = color.New(color.FgHiBlack).SprintFunc()
)

var (
	mtrTUIHistoryLatencyColors = [8]func(a ...interface{}) string{
		color.New(mtrHistoryLatencyColorAttr(0)).SprintFunc(),
		color.New(mtrHistoryLatencyColorAttr(1)).SprintFunc(),
		color.New(mtrHistoryLatencyColorAttr(2)).SprintFunc(),
		color.New(mtrHistoryLatencyColorAttr(3)).SprintFunc(),
		color.New(mtrHistoryLatencyColorAttr(4)).SprintFunc(),
		color.New(mtrHistoryLatencyColorAttr(5)).SprintFunc(),
		color.New(mtrHistoryLatencyColorAttr(6)).SprintFunc(),
		color.New(mtrHistoryLatencyColorAttr(7)).SprintFunc(),
	}
	mtrTUIHistoryTimeoutColorFunc = color.New(color.FgHiBlack).SprintFunc()
)

func mtrColorLossBucket(loss float64, waiting bool) color.Attribute {
	if waiting {
		return color.FgHiBlack
	}
	switch {
	case loss <= 0:
		return color.FgHiGreen
	case loss <= 5:
		return color.FgHiCyan
	case loss <= 20:
		return color.FgHiYellow
	case loss <= 50:
		return color.FgYellow
	default:
		return color.FgHiRed
	}
}

func mtrColorPacketsByLoss(lossCell, sntCell string, loss float64, waiting bool) (string, string) {
	attr := mtrColorLossBucket(loss, waiting)
	sty := color.New(attr, color.Bold).SprintFunc()
	if strings.TrimSpace(lossCell) != "" {
		lossCell = sty(lossCell)
	}
	if strings.TrimSpace(sntCell) != "" {
		sntCell = sty(sntCell)
	}
	return lossCell, sntCell
}

func mtrTUIPlainHistory() bool {
	return color.NoColor
}

func mtrTUIHistoryLatencyColor(s string, level int) string {
	if level < 0 {
		level = 0
	}
	if level >= len(mtrTUIHistoryLatencyColors) {
		level = len(mtrTUIHistoryLatencyColors) - 1
	}
	return mtrTUIHistoryLatencyColors[level](s)
}

func mtrHistoryLatencyColorAttr(level int) color.Attribute {
	switch {
	case level <= 3:
		return color.FgHiGreen
	case level <= 6:
		return color.FgHiYellow
	default:
		return color.FgHiRed
	}
}

func mtrTUIHistoryTimeoutColor(s string) string {
	return mtrTUIHistoryTimeoutColorFunc(s)
}
