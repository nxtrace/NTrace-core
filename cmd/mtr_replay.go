package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

type mtrReplayOptions struct {
	file                                 string
	report, wide, json, noColor, showIPs bool
	columns, language                    string
	hostMode                             int
	noSummary                            bool
	explicit                             map[string]bool
}

func containsMTRReplayFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return false
		}
		name, _, inline := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if strings.HasPrefix(arg, "-") && name == "mtr-replay" {
			return true
		}
		if strings.HasPrefix(arg, "-") && !inline && doctorValueOption(name) {
			i++
		}
	}
	return false
}

func normalizeMTRReplayArgs(fs *flag.FlagSet, args []string) []string {
	result := append([]string(nil), args...)
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		name, _, inline := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		if !strings.HasPrefix(args[i], "-") || inline || !doctorValueOption(name) {
			continue
		}
		if name == "mtr-replay" {
			// Do not consume a recognized option as a missing filename.
			// Explicit = or ./ still permits option-like filenames.
			if i+1 == len(args) {
				result[i] += "="
				continue
			}
			next, _, _ := strings.Cut(strings.TrimLeft(args[i+1], "-"), "=")
			if strings.HasPrefix(args[i+1], "-") && (args[i+1] == "--" || fs.Lookup(next) != nil) {
				result[i] += "="
				continue
			}
		}
		i++ // Preserve values belonging to other options or the replay path.
	}
	return result
}

func maybeRunMTRReplayMode(args []string, stdout, stderr io.Writer) (bool, int) {
	if !containsMTRReplayFlag(args) {
		return false, 0
	}
	fs := flag.NewFlagSet(appBinName+" --mtr-replay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o := mtrReplayOptions{explicit: make(map[string]bool)}
	var help, version bool
	fs.StringVar(&o.file, "mtr-replay", "", "Read an MTR session offline")
	for _, name := range []string{"h", "help"} {
		fs.BoolVar(&help, name, false, "Show replay help")
	}
	for _, name := range []string{"V", "version"} {
		fs.BoolVar(&version, name, false, "Show version")
	}
	for _, name := range []string{"r", "report"} {
		fs.BoolVar(&o.report, name, false, "Print the final statistics")
	}
	for _, name := range []string{"w", "wide"} {
		fs.BoolVar(&o.wide, name, false, "Print a wide final report")
	}
	for _, name := range []string{"C", "no-color"} {
		fs.BoolVar(&o.noColor, name, false, "Disable color")
	}
	for _, name := range []string{"g", "language"} {
		fs.StringVar(&o.language, name, "", "Display language: cn or en")
	}
	for _, name := range []string{"y", "ipinfo"} {
		fs.IntVar(&o.hostMode, name, 0, "Initial host display mode (0-4)")
	}
	for _, name := range []string{"j", "json"} {
		fs.BoolVar(&o.json, name, false, "Print the offline replay result as JSON")
	}
	fs.BoolVar(&o.showIPs, "show-ips", false, "Show PTR and IP together")
	fs.StringVar(&o.columns, "mtr-columns", "", "Select text statistic columns")
	fs.BoolVar(&o.noSummary, "no-mtr-summary", false, "Suppress the summary printed after leaving the MTR TUI")
	ordered, err := doctorArgs(fs, normalizeMTRReplayArgs(fs, args))
	if err == nil {
		err = fs.Parse(ordered)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Invalid replay arguments; use --mtr-replay FILE --help.")
		return true, 2
	}
	if help || version {
		return true, runMTRCLIWithSignals(func(context.Context) int {
			output := &mtrReplayOutput{w: stdout}
			// The sink retains errors, including writes hidden by PrintDefaults.
			if help {
				_, _ = fmt.Fprintf(output, "Usage: %s --mtr-replay FILE [--report|--wide|--json]\nOffline only. J: time, Space: play, P: pause, R: rewind, Q: quit.\n", appBinName)
				fs.SetOutput(output)
				fs.PrintDefaults()
			} else {
				_, _ = fmt.Fprintln(output, config.Version, config.CommitID)
			}
			if output.err != nil {
				_, _ = fmt.Fprintln(stderr, output.err)
				return 1
			}
			return 0
		})
	}
	fs.Visit(func(f *flag.Flag) { o.explicit[f.Name] = true })
	if fs.NArg() != 0 || strings.TrimSpace(o.file) == "" || o.file == "-" {
		_, _ = fmt.Fprintln(stderr, "Replay requires one --mtr-replay file and no target")
		return true, 2
	}
	if o.hostMode < 0 || o.hostMode > 4 || (o.language != "" && o.language != "cn" && o.language != "en") || (o.json && o.explicit["mtr-columns"]) {
		_, _ = fmt.Fprintln(stderr, "Invalid replay display options")
		return true, 2
	}
	if o.explicit["mtr-columns"] {
		if _, err = printer.ParseMTRColumns(o.columns, false); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return true, 2
		}
	}
	return true, runMTRCLIWithSignals(func(ctx context.Context) int {
		err := runMTRReplay(ctx, o, stdout, stderr)
		if cause := context.Cause(ctx); cause != nil {
			err = cause
		}
		var interrupted *mtrJSONSignal
		if errors.As(err, &interrupted) {
			if interrupted.signal == syscall.SIGTERM {
				return 143
			}
			return 130
		}
		if err != nil {
			if err != errMTRReplayIncomplete {
				_, _ = fmt.Fprintln(stderr, sanitizeMTRReplayText(err.Error()))
			}
			return 1
		}
		return 0
	})
}

var errMTRReplayIncomplete = errors.New("incomplete MTR recording")

func runMTRReplay(ctx context.Context, opts mtrReplayOptions, stdout, stderr io.Writer) (err error) {
	r, err := mtrsession.OpenReader(opts.file)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if !opts.json && !opts.report && !opts.wide && stdout == os.Stdout && CheckTTY(int(os.Stdin.Fd()), int(os.Stdout.Fd())) {
		_, _ = fmt.Fprintln(stderr, "Loading recording...")
	}
	loaded, err := readMTRReplay(ctx, r, time.Duration(1<<63-1))
	if err != nil {
		return err
	}
	complete := !r.Incomplete()
	defer func() {
		if err == nil && !complete {
			err = errMTRReplayIncomplete
		}
	}()
	duration := loaded.cursor
	if !complete {
		_, _ = fmt.Fprintln(stderr, "Incomplete recording: displaying all complete events.")
	}
	if opts.json {
		snap := loaded.state.Snapshot()
		if snap.Stats == nil {
			snap.Stats = []trace.MTRHopStat{}
		}
		return json.NewEncoder(&mtrReplayOutput{w: stdout}).Encode(struct {
			SchemaVersion  int                 `json:"schema_version"`
			Type           string              `json:"type"`
			Session        *mtrsession.Session `json:"session"`
			Complete       bool                `json:"complete"`
			CursorNS       int64               `json:"cursor_ns"`
			DurationNS     int64               `json:"duration_ns"`
			Generation     uint64              `json:"generation"`
			RecordedPaused bool                `json:"recorded_paused"`
			PathEnd        *trace.StopReason   `json:"path_end"`
			Stats          []trace.MTRHopStat  `json:"stats"`
			End            *mtrsession.End     `json:"end,omitempty"`
		}{1, "mtr_replay", loaded.session, complete, int64(duration), int64(duration), loaded.state.Generation(), loaded.state.Paused(), loaded.state.PathEnd(), snap.Stats, loaded.end})
	}
	header, err := mtrReplayHeader(loaded.session, opts)
	if err != nil {
		return err
	}
	if opts.report || opts.wide || stdout != os.Stdout || !CheckTTY(int(os.Stdin.Fd()), int(os.Stdout.Fd())) {
		wide := opts.wide || (!opts.report && loaded.session.Display.Wide)
		return printer.WriteMTRReplayReport(stdout, sanitizeMTRReplayStats(loaded.state.Snapshot().Stats), printer.MTRReportOptions{Columns: header.Columns, StartTime: header.StartTime, SrcHost: header.SrcHost, Wide: wide, ShowIPs: header.ShowIPs, Lang: header.Lang})
	}
	applyColorMode(opts.noColor)
	return runMTRReplayTUI(ctx, r, loaded, header, duration, complete, opts.noSummary, stdout)
}

func mtrReplayHeader(session *mtrsession.Session, opts mtrReplayOptions) (printer.MTRTUIHeader, error) {
	header := printer.MTRTUIHeader{Target: session.Target, Domain: session.Target, TargetIP: session.ResolvedIP, Version: session.Version, StartTime: session.StartedAt, SrcHost: session.SourceHost, DisplayMode: session.Display.HostMode, NameMode: session.Display.HostNameMode, ShowIPs: session.Display.ShowIPs, DisableMPLS: !session.Display.ShowMPLS}
	if p := session.EffectiveParameters; p != nil {
		header.Lang = p.Language
		header.SrcIP = p.SourceAddress
	}
	if session.SourceIP != "" {
		header.SrcIP = session.SourceIP
	}
	if opts.explicit["language"] || opts.explicit["g"] {
		header.Lang = opts.language
	}
	if opts.explicit["ipinfo"] || opts.explicit["y"] {
		header.DisplayMode = opts.hostMode
	}
	if opts.explicit["show-ips"] {
		header.ShowIPs = opts.showIPs
	}
	columns := session.Display.Columns
	if opts.explicit["mtr-columns"] {
		columns = opts.columns
	}
	if columns != "" {
		var err error
		header.Columns, err = printer.ParseMTRColumns(columns, false)
		if err != nil {
			return header, err
		}
	}
	for _, field := range []*string{&header.Target, &header.Domain, &header.TargetIP, &header.Version, &header.SrcHost, &header.SrcIP} {
		*field = sanitizeMTRReplayText(*field)
	}
	return header, nil
}
