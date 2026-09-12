package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
)

type mtrJSONOptions struct {
	Target             string
	Method             trace.Method
	Config             trace.Config
	Report             bool
	MaxPerHop          int
	HopIntervalMs      int
	PacketSize         int
	PacketSizeExplicit bool
	IPv4Only           bool
	IPv6Only           bool
	DataProvider       string
	PowProvider        string
	DotServer          string
	RecordPath         string
	RecordDisplay      mtrRecordingDisplay
	PrepareAsync       bool
	CompactReport      bool
	HumanOutput        bool
	NoSummary          bool
	OnEvent            func(trace.MTRSessionEvent) error
}

var runMTRJSONRawFn = trace.RunMTRRaw

// Only used to route pre-parser diagnostics/standalone dispatch. Parsed flags
// remain the authority for selecting an output mode.
func requestsMTRJSON(args []string) bool {
	jsonOutput, mtr := false, !enableTraceroute && defaultMTR
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		name, _, inline := strings.Cut(arg, "=")
		if strings.HasPrefix(arg, "-") && !inline && doctorValueOption(strings.TrimLeft(name, "-")) {
			i++
			continue
		}
		switch name {
		case "-j", "--json":
			jsonOutput = true
		case "-t", "--mtr", "-r", "--report", "-w", "--wide":
			mtr = true
		}
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			jsonOutput = jsonOutput || strings.Contains(arg[1:], "j")
			mtr = mtr || strings.ContainsAny(arg[1:], "trw")
		}
	}
	return jsonOutput && mtr
}

func runMTRJSONCLI(opts mtrJSONOptions) int {
	return runMTRCLIWithSignals(func(ctx context.Context) int {
		return runMTRJSON(ctx, opts, os.Stdout, os.Stderr)
	})
}

func runMTRCLIWithSignals(run func(context.Context) int) int {
	// Let a closed stdout pipe return EPIPE to the writer so the runner can
	// cancel and join its workers instead of the runtime exiting on SIGPIPE.
	pipeSignals := make(chan os.Signal, 1)
	signal.Notify(pipeSignals, syscall.SIGPIPE)
	defer signal.Stop(pipeSignals)
	ctx, cancel := context.WithCancelCause(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case sig := <-signals:
			cancel(&mtrJSONSignal{signal: sig})
		case <-pipeSignals:
			cancel(io.ErrClosedPipe)
		case <-ctx.Done():
		}
	}()
	defer func() {
		signal.Stop(signals)
		cancel(nil)
		<-done
	}()
	return run(ctx)
}

func runMTRJSON(parent context.Context, opts mtrJSONOptions, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	out := newMTRJSONOutput(stdout, !opts.Report, opts.Target, opts.Method, cancel)
	stage := "validation"
	var cleanup func()
	err := normalizeMTRJSONOptions(&opts, stderr)
	var recording *mtrRecording
	if err == nil && opts.RecordPath != "" {
		stage = "record"
		recording, err = openMTRRecording(opts.RecordPath)
		if recording != nil {
			defer recording.close()
			opts.OnEvent = recording.event
		}
	}
	if err == nil {
		stage = "resolve"
		cleanup, err = prepareMTRJSON(ctx, &opts, out, &stage, stderr)
	}
	if recording != nil {
		display := opts.RecordDisplay
		if err == nil {
			display.sourceIP = resolveSrcIP(opts.Config)
		}
		if startErr := recording.start(out, display); startErr != nil && err == nil {
			err, stage = startErr, "record"
		}
	}
	if err == nil {
		out.startStream()
		stage = "probe"
		if ctx.Err() != nil {
			err = context.Cause(ctx)
		} else if opts.Report {
			out.report.Stats, out.report.PathEnd, err = collectMTRReport(ctx, opts.Method, opts.Config, trace.MTROptions{
				HopInterval: time.Duration(opts.HopIntervalMs) * time.Millisecond,
				MaxPerHop:   opts.MaxPerHop,
				OnEvent:     opts.OnEvent,
			})
		} else {
			err = runMTRJSONStream(ctx, opts, out)
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
	if recording != nil {
		if recordErr := recording.finish(err, stage); recordErr != nil && (err == nil || errors.Is(recordErr, err)) {
			// The scheduler can return the writer's original failure. Keep that
			// cause while classifying it as recording, not a probe failure.
			if err == nil {
				err = recordErr
			}
			stage = "record"
		}
	}
	return out.finish(err, stage, stderr)
}

func runMTRJSONStream(ctx context.Context, opts mtrJSONOptions, out *mtrJSONOutput) error {
	// The per-hop scheduler announces a path change before the associated RAW
	// callback. Hold only that change so the JSON probe precedes its conclusion.
	var pending *trace.StopReason
	pathChanged := false
	flushPath := func() {
		if pathChanged {
			out.pathEnd(pending)
			pathChanged = false
		}
	}
	err := runMTRJSONRawFn(ctx, opts.Method, opts.Config, trace.MTRRawOptions{
		HopInterval: time.Duration(opts.HopIntervalMs) * time.Millisecond,
		OnEvent:     opts.OnEvent,
		MaxPerHop:   opts.MaxPerHop, OnPathEnd: func(reason *trace.StopReason) {
			flushPath()
			pending, pathChanged = copyMTRPathEnd(reason), true
		},
	}, func(record trace.MTRRawRecord) {
		out.probe(record)
		flushPath()
	})
	// Only max-hops can conclude without a following probe callback. A recording
	// failure may drop the causal probe for any other pending path change.
	if pending != nil && pending.Reason == trace.StopReasonMaxHops {
		flushPath()
	}
	return err
}

func normalizeMTRJSONOptions(opts *mtrJSONOptions, stderr io.Writer) error {
	if err := trace.ValidateFWMarkPlatform(opts.Config.FWMarkSet); err != nil {
		return err
	}
	if strings.TrimSpace(normalizeCLITarget(opts.Target)) == "" {
		return errors.New("MTR JSON requires a target")
	}
	if opts.IPv4Only && opts.IPv6Only {
		return errors.New("--ipv4 cannot be combined with --ipv6")
	}
	if opts.Report && opts.MaxPerHop <= 0 {
		return errors.New("MTR JSON report requires --queries greater than zero")
	}
	if opts.MaxPerHop < 0 {
		opts.MaxPerHop = 0
	}
	if opts.Method != trace.TCPTrace && opts.MaxPerHop > 255 {
		_, _ = fmt.Fprintln(stderr, "Query 最大值为 255，已自动调整为 255")
		opts.MaxPerHop = 255
	}
	if opts.HopIntervalMs <= 0 {
		opts.HopIntervalMs = 1000
	}
	cfg := &opts.Config
	if cfg.TOS < 0 || cfg.TOS > 255 {
		return errors.New("--tos must be between 0 and 255")
	}
	if cfg.Timeout <= 0 {
		return errors.New("--timeout must be greater than zero")
	}
	if cfg.BeginHop <= 0 {
		cfg.BeginHop = 1
	}
	if cfg.MaxHops <= 0 {
		cfg.MaxHops = 30
	}
	if cfg.MaxHops > 255 {
		cfg.MaxHops = 255
	}
	if cfg.BeginHop > cfg.MaxHops {
		return errors.New("--first must not exceed --max-hops")
	}
	if cfg.ParallelRequests <= 0 {
		cfg.ParallelRequests = 1
	}
	applyDefaultPort(&cfg.DstPort, opts.Method == trace.UDPTrace)
	if opts.Method != trace.ICMPTrace && (cfg.SrcPort < -1 || cfg.SrcPort > 65535 || cfg.DstPort < 1 || cfg.DstPort > 65535) {
		return errors.New("probe ports must be between 1 and 65535 (source port 0 selects automatically, -1 selects randomly per probe)")
	}
	if cfg.ICMPMode <= 0 && util.EnvICMPMode > 0 {
		cfg.ICMPMode = util.EnvICMPMode
	}
	if cfg.ICMPMode < 0 || cfg.ICMPMode > 2 {
		cfg.ICMPMode = 0
	}
	cfg.NumMeasurements, cfg.MaxAttempts, cfg.TTLInterval = 1, 1, 0
	cfg.RealtimePrinter, cfg.AsyncPrinter = nil, nil
	return nil
}

func prepareMTRJSON(ctx context.Context, opts *mtrJSONOptions, out *mtrJSONOutput, stage *string, stderr io.Writer) (func(), error) {
	restoreOutput := setFastIPOutputSuppression(true)
	cleanup := restoreOutput
	configureGeoDNS(opts.DotServer)
	if err := ctx.Err(); err != nil {
		return cleanup, context.Cause(ctx)
	}
	ip, err := lookupTargetIP(ctx, normalizeCLITarget(opts.Target), opts.IPv4Only, opts.IPv6Only, opts.DotServer, !opts.HumanOutput)
	if err != nil {
		return cleanup, err
	}
	if ip == nil {
		return cleanup, errors.New("target resolution returned no address")
	}
	out.report.ResolvedIP = ptrStr(ip.String())
	*stage = "initialize"
	cfg := &opts.Config
	cfg.DstIP, cfg.Context = ip, ctx
	prepared, err := resolveCLIProbeSource(opts.Method, *cfg)
	if err != nil {
		return cleanup, err
	}
	*cfg = prepared
	src := cfg.SrcAddr
	if src != "" {
		sourceIP := net.ParseIP(src)
		if sourceIP == nil || (sourceIP.To4() == nil) != (ip.To4() == nil) {
			*stage = "validation"
			return cleanup, errors.New("--source must be an IP address matching the target IP family")
		}
	}
	packetSize := resolvePacketSizeArg(opts.PacketSize, opts.PacketSizeExplicit, opts.Method, ip)
	packet, err := trace.NormalizePacketSize(opts.Method, ip, packetSize)
	if err != nil {
		*stage = "validation"
		return cleanup, err
	}
	cfg.PktSize, cfg.RandomPacketSize = packet.PayloadSize, packet.Random
	status := util.TracePrivilegeStatus(appBinName, false)
	if status.Message != "" {
		_, _ = fmt.Fprintln(stderr, status.Message)
	}
	if status.Fatal {
		return cleanup, errors.New(status.Message)
	}
	if cfg.DN42 || isDN42Provider(opts.DataProvider) {
		if err := config.InitConfigWithWriter(stderr); err != nil {
			return cleanup, err
		}
		opts.DataProvider, cfg.DN42 = "DN42", true
	}
	conn := initNextTraceAPIV3WebSocket(ctx, &opts.DataProvider, &opts.PowProvider, opts.PrepareAsync)
	cleanup = func() { closeNextTraceAPIV3WebSocket(conn); restoreOutput() }
	// Runtime provider fallback can select DN42 as well.
	if !cfg.DN42 && isDN42Provider(opts.DataProvider) {
		if err := config.InitConfigWithWriter(stderr); err != nil {
			return cleanup, err
		}
		cfg.DN42 = true
	}
	descriptor := ipgeo.GetSourceDescriptorSession(opts.DataProvider)
	session := trace.CachedGeoSourceSession(descriptor)
	cfg.IPGeoSource, cfg.IPGeoDescriptor, cfg.RefreshIPGeoSource = session.Source, descriptor.Current, session.Refresh
	*cfg = normalizeMTRReportConfig(*cfg, !opts.CompactReport)
	util.SrcPort, util.DstIP = cfg.SrcPort, ip.String()
	params := &mtrJSONParameters{
		MaxPerHop: opts.MaxPerHop, HopIntervalMs: opts.HopIntervalMs, TimeoutMs: cfg.Timeout.Milliseconds(),
		BeginHop: cfg.BeginHop, MaxHops: cfg.MaxHops, ParallelRequests: cfg.ParallelRequests,
		SourceAddress: cfg.SrcAddr, SourceDevice: cfg.SourceDevice, PacketSize: packetSize, RandomPacketSize: packet.Random,
		TOS: cfg.TOS, DataProvider: opts.DataProvider, Language: cfg.Lang, DotServer: opts.DotServer,
		RDNS: cfg.RDNS, AlwaysWaitRDNS: cfg.RDNS && cfg.AlwaysWaitRDNS, DisableMPLS: cfg.DisableMPLS, DN42: cfg.DN42,
	}
	if cfg.FWMarkSet {
		mark := cfg.FWMark
		params.FWMark = &mark
	}
	if opts.Method != trace.ICMPTrace {
		params.Port, params.SourcePort = ptrInt(cfg.DstPort), ptrInt(cfg.SrcPort)
	}
	if cfg.OSType == 2 {
		params.ICMPMode = ptrInt(cfg.ICMPMode)
	}
	out.report.EffectiveParameters = params
	return cleanup, nil
}
