package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

func containsMTRRecordFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		name, _, inline := strings.Cut(arg, "=")
		if name == "--mtr-record" {
			return true
		}
		if strings.HasPrefix(arg, "-") && !inline && doctorValueOption(strings.TrimLeft(name, "-")) {
			i++
		}
	}
	return false
}

func validateMTRRecordMode(path string, explicit bool, modes effectiveMTRModes, standalone string, mcp bool) error {
	if !explicit {
		return nil
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("--mtr-record requires a new file path")
	}
	if !modes.mtr || standalone != "" || mcp {
		return errors.New("--mtr-record requires MTR mode and cannot be combined with a standalone mode")
	}
	return nil
}

type mtrRecordingDisplay struct {
	hostMode int
	sourceIP string
	showIPs  bool
	wide     bool
	columns  string
}

type mtrRecording struct {
	writer  *mtrsession.Writer
	pathEnd *trace.StopReason
}

func openMTRRecording(path string) (*mtrRecording, error) {
	w, err := mtrsession.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open MTR recording: %w", err)
	}
	return &mtrRecording{writer: w}, nil
}

func (r *mtrRecording) start(out *mtrJSONOutput, display mtrRecordingDisplay) error {
	if display.hostMode < 0 || display.hostMode > 4 {
		display.hostMode = 0 // Live JSON ignores this presentation option.
	}
	host, _ := os.Hostname()
	session := mtrsession.Session{
		Version: out.report.Version, Target: out.report.Target, Protocol: out.report.Protocol,
		StartedAt: out.start, EffectiveParameters: out.report.EffectiveParameters,
		SourceHost: host, SourceIP: display.sourceIP,
		Display: mtrsession.DisplayParameters{
			HostMode: display.hostMode, ShowIPs: display.showIPs, ShowMPLS: true,
			Wide: display.wide, Columns: display.columns,
		},
	}
	if out.report.ResolvedIP != nil {
		session.ResolvedIP = *out.report.ResolvedIP
	}
	if session.EffectiveParameters != nil {
		params := *session.EffectiveParameters
		session.EffectiveParameters = &params
		session.Display.ShowMPLS = !params.DisableMPLS
	}
	return r.writer.Start(session)
}

func (r *mtrRecording) event(event trace.MTRSessionEvent) error {
	if event.Type == trace.MTRSessionPathEndEvent {
		r.pathEnd = copyMTRPathEnd(event.PathEnd)
	}
	if event.Type == trace.MTRSessionResetEvent {
		r.pathEnd = nil
	}
	return r.writer.Event(event)
}

func (r *mtrRecording) finish(err error, stage string) error {
	end := mtrsession.End{EndedAt: time.Now(), EndReason: "completed", PathEnd: r.pathEnd}
	var interrupted *mtrJSONSignal
	switch {
	case errors.As(err, &interrupted):
		end.EndReason, end.Signal = "interrupted", "SIGINT"
		if interrupted.signal == syscall.SIGTERM {
			end.Signal = "SIGTERM"
		}
	case errors.Is(err, context.Canceled):
		end.EndReason = "interrupted"
	case err != nil:
		end.EndReason = "error"
		end.Error = &mtrsession.Error{Stage: stage, Message: err.Error()}
	}
	return r.writer.Finish(end)
}

func (r *mtrRecording) close() {
	// Finish owns and reports normal close errors. This also closes an early
	// failure without inventing a complete end record.
	_ = r.writer.Close()
}

func runMTRRecordedCLI(opts mtrJSONOptions, modes effectiveMTRModes, showIPs bool, hostMode int, columnNames string, columns []printer.MTRColumn) int {
	return runMTRCLIWithSignals(func(ctx context.Context) int {
		return runMTRRecorded(ctx, opts, modes, showIPs, hostMode, columnNames, columns, os.Stderr)
	})
}

func runMTRRecorded(ctx context.Context, opts mtrJSONOptions, modes effectiveMTRModes, showIPs bool, hostMode int, columnNames string, columns []printer.MTRColumn, stderr io.Writer) int {
	if hostMode < 0 || hostMode > 4 {
		_, _ = fmt.Fprintln(stderr, "--ipinfo/-y must be between 0 and 4")
		return 2
	}
	if err := normalizeMTRRecordedOptions(&opts, modes, stderr); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	recording, err := openMTRRecording(opts.RecordPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer recording.close()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	out := newMTRJSONOutput(io.Discard, false, opts.Target, opts.Method, cancel)
	opts.PrepareAsync = shouldUseAsyncNextTraceAPIV3ForMTR(modes, CheckTTY(int(os.Stdin.Fd())), CheckTTY(int(os.Stdout.Fd())))
	opts.CompactReport = modes.report && !modes.wide && !modes.raw
	stage := "resolve"
	cleanup, err := prepareMTRJSON(ctx, &opts, out, &stage, stderr)
	display := mtrRecordingDisplay{hostMode: hostMode, showIPs: showIPs, wide: modes.wide, columns: columnNames}
	if err == nil {
		display.sourceIP = resolveSrcIP(opts.Config)
	}
	if startErr := recording.start(out, display); startErr != nil && err == nil {
		err, stage = startErr, "record"
	}
	if err == nil {
		stage = "probe"
		switch chooseMTRRunMode(modes.raw, modes.report) {
		case mtrRunRaw:
			err = runMTRRaw(opts.Method, opts.Config, opts.HopIntervalMs, opts.MaxPerHop, opts.DataProvider, recording.event)
		case mtrRunReport:
			err = runMTRReport(opts.Method, opts.Config, opts.HopIntervalMs, opts.MaxPerHop, opts.Target, opts.DataProvider, modes.wide, showIPs, recording.event, columns...)
		default:
			err = runMTRTUI(opts.Method, opts.Config, opts.HopIntervalMs, opts.MaxPerHop, opts.Target, opts.DataProvider, showIPs, hostMode, recording.event,
				mtrTUIOptions{NoSummary: opts.NoSummary, Parameters: out.report.EffectiveParameters}, columns...)
		}
	}
	if cause := context.Cause(ctx); cause != nil && (err == nil || errors.Is(err, context.Canceled)) {
		err = cause
	}
	if trace.IsInitializationError(err) {
		stage = "initialize"
	}
	if cleanup != nil {
		cleanup()
	}
	if finishErr := recording.finish(err, stage); finishErr != nil && err == nil {
		err = fmt.Errorf("write MTR recording: %w", finishErr)
	}
	var interrupted *mtrJSONSignal
	if errors.As(err, &interrupted) {
		if interrupted.signal == syscall.SIGTERM {
			return 143
		}
		return 130
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func normalizeMTRRecordedOptions(opts *mtrJSONOptions, modes effectiveMTRModes, stderr io.Writer) error {
	opts.HumanOutput = true
	// Human report defaults nonpositive counts to ten; RAW keeps its explicit
	// unlimited count even when --report was used to select the mode.
	opts.Report = modes.report && !modes.raw
	if opts.Report && opts.MaxPerHop <= 0 {
		opts.MaxPerHop = 10
	}
	return normalizeMTRJSONOptions(opts, stderr)
}
