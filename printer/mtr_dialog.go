package printer

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-runewidth"
)

// Dialog values are immutable frame state supplied by the input controller.
type MTRSaveDialog struct {
	Active, Busy  bool
	Format, Path  string
	Error, Notice string
	CapturedAt    time.Time
	Field         int // 0 = format, 1 = path
}

type MTRHelpDialog struct {
	Active bool
	Offset int
}

func mtrDialogText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, text)
}

func renderMTRSaveDialog(b *strings.Builder, dialog MTRSaveDialog, width, height int) {
	remaining := max(0, height-1)
	line := func(text string) {
		if remaining > 0 {
			tuiLine(b, "%s", truncateByDisplayWidth(mtrDialogText(text), width))
			remaining--
		}
	}
	line("Save report")
	format := "  Format: " + dialog.Format
	if dialog.Field == 0 {
		format = "> Format: " + dialog.Format
	}
	line(format)
	prefix := "  Path: "
	if dialog.Field == 1 {
		prefix = "> Path: "
	}
	if width <= displayWidth(prefix)+1 {
		prefix = ">"
	}
	path := mtrDialogText(dialog.Path)
	available := max(0, width-displayWidth(prefix)-1)
	runes := []rune(path)
	start, used := len(runes), 0
	for start > 0 {
		columns := runewidth.RuneWidth(runes[start-1])
		if used+columns > available {
			break
		}
		used += columns
		start--
	}
	path = string(runes[start:])
	line(prefix + path + "_")
	switch {
	case dialog.Busy:
		line("Saving...")
	case dialog.Error != "":
		line(dialog.Error)
	case dialog.Notice != "":
		line(dialog.Notice)
	}
	// Keep primary controls available before optional metadata on short screens.
	line("Enter: save  Esc: close")
	line("Tab: field  Left/Right/Space: format")
	line("Backspace: delete  Ctrl-U: clear")
	line("Ctrl-C: quit")
	if !dialog.CapturedAt.IsZero() {
		line("Captured: " + dialog.CapturedAt.Format("2006-01-02T15:04:05.000Z07:00"))
	}
}

func mtrHelpEntries(replay bool) []string {
	entries := []string{
		"q/Q, Ctrl-C: quit",
		"s/S: save current report",
		"?: open or close this help",
		"Up/Down: scroll help",
		"Esc: close help or editor",
		"p/P: pause probing",
		"Space: resume probing",
		"r/R: reset statistics",
	}
	if replay {
		entries[5] = "p/P: pause playback"
		entries[6] = "Space: play recording"
		entries[7] = "r/R: rewind and pause"
		entries = append(entries, "j/J: go to HH:MM:SS[.mmm]")
	}
	return append(entries,
		"y/Y: cycle IP/PTR, ASN, City, Owner, Full",
		"n/N: toggle PTR/IP or IP only",
		"e/E: toggle MPLS labels",
		"d/D: toggle history view",
		"g/G: cycle charts in history view",
		"o/O: edit statistics columns",
		"Fields: Enter applies; Space adds a gap",
		"Save: Tab switches format and path",
		"Save format: Left/Right/Space cycles",
		"Save: Enter writes; existing files stay intact",
		"Editors: Backspace deletes; Ctrl-U clears",
		"Editors: pasted newlines never submit",
		"Editors: Ctrl-C quits the session",
	)
}

func mtrHelpLines(replay bool, width int) []string {
	width = max(1, width)
	var lines []string
	for _, entry := range mtrHelpEntries(replay) {
		var line strings.Builder
		columns := 0
		for _, r := range entry {
			w := runewidth.RuneWidth(r)
			if columns+w > width && line.Len() > 0 {
				lines = append(lines, line.String())
				line.Reset()
				columns = 0
			}
			line.WriteRune(r)
			columns += w
		}
		lines = append(lines, line.String())
	}
	return lines
}

func MTRHelpMaxOffset(replay bool, width, height int) int {
	return max(0, len(mtrHelpLines(replay, width))-max(1, height-3))
}

func renderMTRHelpDialog(b *strings.Builder, dialog MTRHelpDialog, replay bool, width, height int) {
	remaining := max(0, height-1)
	if remaining == 0 {
		return
	}
	tuiLine(b, "%s", truncateByDisplayWidth("Keyboard shortcuts", width))
	remaining--
	if remaining == 0 {
		return
	}
	lines := mtrHelpLines(replay, width)
	count := max(0, remaining-1)
	offset := min(max(0, dialog.Offset), max(0, len(lines)-count))
	for _, line := range lines[offset:min(len(lines), offset+count)] {
		tuiLine(b, "%s", line)
	}
	footer := fmt.Sprintf("?:close Esc:close Up/Down (%d/%d)", offset+1, len(lines))
	tuiLine(b, "%s", truncateByDisplayWidth(footer, width))
}
