package printer

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/nxtrace/NTrace-core/trace"
	"golang.org/x/term"
)

// MTRTUIRenderer updates an interactive session without erasing the display.
// Its zero value is ready for use. The session's single rendering worker must
// call it sequentially with the same terminal writer throughout the session.
type MTRTUIRenderer struct {
	width, height int
	rows          []string
}

// Render writes only changed rows, using the current stdout terminal size.
func (r *MTRTUIRenderer) Render(w io.Writer, header MTRTUIHeader, stats []trace.MTRHopStat) error {
	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || height <= 0 {
		height = 24
	}
	return r.renderWithSize(w, header, stats, getTermWidth(), height)
}

func (r *MTRTUIRenderer) renderWithSize(w io.Writer, header MTRTUIHeader, stats []trace.MTRHopStat, width, height int) error {
	if width <= 0 {
		width = tuiDefaultTerm
	}
	if height <= 0 {
		height = 24
	}
	frame := buildMTRTUIFrame(header, stats, width, height)
	rows := strings.Split(strings.TrimSuffix(frame, "\r\n"), "\r\n")
	rows = rows[:min(len(rows), height-1)]
	for i := range rows {
		rows[i] = ansi.Truncate(rows[i], width, "")
	}
	full := r.width != width || r.height != height
	count := max(len(rows), len(r.rows))
	if full {
		// Resizing may reflow old text onto any visible row, including the
		// reserved bottom row. EL never pushes that text into scrollback.
		count = height
	}
	var update strings.Builder
	for i := 0; i < count; i++ {
		var row string
		if i < len(rows) {
			row = rows[i]
		}
		if !full && i < len(r.rows) && row == r.rows[i] {
			continue
		}
		// Clear before writing: clearing after a full-width row would erase
		// its final character while the terminal is pending an automatic wrap.
		fmt.Fprintf(&update, "\033[%d;1H\033[0m\033[2K%s\033[0m", i+1, row)
	}
	if update.Len() > 0 {
		n, err := io.WriteString(w, update.String())
		if err == nil && n != update.Len() {
			err = io.ErrShortWrite
		}
		if err != nil {
			// The terminal may have consumed only part of the update. The next
			// attempt must rebuild the visible area rather than trust the cache.
			*r = MTRTUIRenderer{}
			return err
		}
	}
	r.width, r.height, r.rows = width, height, rows
	return nil
}
