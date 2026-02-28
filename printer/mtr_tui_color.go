package printer

import (
	"strings"

	"github.com/fatih/color"
)

var (
	mtrTUIHeaderColor = color.New(color.FgHiCyan, color.Bold).SprintFunc()
	mtrTUIRouteColor  = color.New(color.FgHiWhite).SprintFunc()
	mtrTUITimeColor   = color.New(color.FgHiBlue).SprintFunc()
	mtrTUIKeyColor    = color.New(color.FgHiCyan).SprintFunc()
	mtrTUIStatusColor = color.New(color.FgHiYellow, color.Bold).SprintFunc()

	mtrTUIHopColor  = color.New(color.FgHiCyan, color.Bold).SprintFunc()
	mtrTUIHostColor = color.New(color.FgHiWhite).SprintFunc()
	mtrTUIMPLSColor = color.New(color.FgHiBlack).SprintFunc()
	mtrTUIWaitColor = color.New(color.FgHiBlack).SprintFunc()
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
