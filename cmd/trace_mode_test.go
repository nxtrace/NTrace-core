package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/akamensky/argparse"
)

func TestResolveTraceModes(t *testing.T) {
	type modeCase struct {
		name     string
		opts     traceModeOptions
		want     effectiveMTRModes
		conflict string
	}
	for _, useDefault := range []bool{false, true} {
		tests := []modeCase{
			{name: "default", want: effectiveMTRModes{mtr: useDefault}},
			{name: "raw follows default", opts: traceModeOptions{raw: true}, want: effectiveMTRModes{mtr: useDefault, raw: useDefault}},
			{name: "traditional", opts: traceModeOptions{traceroute: true}},
			{name: "traditional raw", opts: traceModeOptions{traceroute: true, raw: true}},
			{name: "traditional format", opts: traceModeOptions{traditional: true}},
			{name: "traditional format and raw", opts: traceModeOptions{traditional: true, raw: true}},
			{name: "explicit mtr", opts: traceModeOptions{mtr: true}, want: effectiveMTRModes{mtr: true}},
			{name: "report", opts: traceModeOptions{report: true}, want: effectiveMTRModes{mtr: true, report: true}},
			{name: "wide", opts: traceModeOptions{wide: true}, want: effectiveMTRModes{mtr: true, report: true, wide: true}},
			{name: "mtr raw", opts: traceModeOptions{mtr: true, raw: true}, want: effectiveMTRModes{mtr: true, raw: true}},
			{name: "report raw", opts: traceModeOptions{report: true, raw: true}, want: effectiveMTRModes{mtr: true, report: true, raw: true}},
			{name: "mtr conflict", opts: traceModeOptions{traceroute: true, mtr: true}, conflict: "--mtr"},
			{name: "report conflict", opts: traceModeOptions{traceroute: true, report: true}, conflict: "--report"},
			{name: "wide conflict", opts: traceModeOptions{traceroute: true, wide: true}, conflict: "--wide"},
			// Keep explicit MTR selected so the existing output conflict validator rejects it.
			{name: "explicit mtr with format", opts: traceModeOptions{mtr: true, traditional: true}, want: effectiveMTRModes{mtr: true}},
		}
		for _, mode := range []string{"--mtu", "--nali", "--deploy"} {
			tests = append(tests, modeCase{
				name: mode, opts: traceModeOptions{standalone: mode, traditional: true},
			}, modeCase{
				name: mode + " conflict", opts: traceModeOptions{standalone: mode, traceroute: true}, conflict: mode,
			})
		}
		for _, tt := range tests {
			t.Run(tt.name+map[bool]string{false: "/trace-default", true: "/mtr-default"}[useDefault], func(t *testing.T) {
				got, err := resolveTraceModes(tt.opts, useDefault)
				if tt.conflict != "" {
					if err == nil || !strings.Contains(err.Error(), tt.conflict) {
						t.Fatalf("error = %v, want conflict %s", err, tt.conflict)
					}
					return
				}
				if err != nil || got != tt.want {
					t.Fatalf("got %#v, %v; want %#v", got, err, tt.want)
				}
			})
		}
	}
}

func TestTracerouteFlagAliasesAndConflicts(t *testing.T) {
	for _, flag := range []string{"-k", "--traceroute"} {
		for _, other := range []string{"", "-t", "--mtr", "-r", "--report", "-w", "--wide"} {
			for _, reversed := range []bool{false, true} {
				parser := argparse.NewParser("nexttrace", "")
				traditional := registerTracerouteFlagWithAvailability(parser, true)
				mtr := parser.Flag("t", "mtr", nil)
				report := parser.Flag("r", "report", nil)
				wide := parser.Flag("w", "wide", nil)
				args := []string{flag}
				if other != "" {
					args = append(args, other)
				}
				if reversed && len(args) == 2 {
					args[0], args[1] = args[1], args[0]
				}
				if err := parser.Parse(append([]string{"nexttrace"}, args...)); err != nil {
					t.Fatal(err)
				}
				for _, useDefault := range []bool{false, true} {
					got, err := resolveTraceModes(traceModeOptions{traceroute: *traditional, mtr: *mtr, report: *report, wide: *wide}, useDefault)
					if (err != nil) != (other != "") {
						t.Fatalf("args %v: error %v", args, err)
					}
					if other == "" && got.mtr {
						t.Fatalf("args %v selected MTR", args)
					}
				}
			}
		}
	}
}

func TestModeFlagsMatchFlavorCapabilities(t *testing.T) {
	parser := argparse.NewParser(appBinName, "")
	traditional := registerTracerouteFlag(parser)
	registerTracerouteOutputFlags(parser)
	mtr := registerMTRFlags(parser)
	registerPacketIntervalFlag(parser)
	registerFileFlag(parser)
	usage := parser.Usage(nil)
	for _, flag := range []string{"--traceroute", "--classic", "--output", "--send-time", "--file", "--no-stop-reason"} {
		if strings.Contains(usage, flag) != enableTraceroute {
			t.Errorf("%s availability incorrect in %s", flag, appBinName)
		}
	}
	for _, flag := range []string{"--json", "--report", "--wide", "--show-ips", "--ipinfo"} {
		if !strings.Contains(usage, flag) {
			t.Errorf("missing %s in %s", flag, appBinName)
		}
	}
	got, err := resolveTraceModes(traceModeOptions{traceroute: *traditional, mtr: *mtr.mtrMode}, defaultMTR)
	if err != nil || got.mtr != (appBinName == "ntr") {
		t.Fatalf("default mode = %#v, %v", got, err)
	}
	if err := parser.Parse([]string{appBinName, "--mtr"}); (err == nil) != enableTraceroute {
		t.Fatalf("--mtr parse in %s: %v", appBinName, err)
	}
}

func TestTraditionalOutputFlagsSurviveMTRDefault(t *testing.T) {
	for _, flag := range []string{"--json", "--table", "--classic", "--output-default", "--route-path", "--output"} {
		parser := argparse.NewParser("nexttrace", "")
		registerTracerouteFlagWithAvailability(parser, true)
		flags := registerTracerouteOutputFlagsWithAvailability(parser, true)
		args := []string{"nexttrace", flag}
		if flag == "--output" {
			args = append(args, "trace.log")
		}
		if err := parser.Parse(args); err != nil {
			t.Fatal(err)
		}
		selected := *flags.jsonPrint || *flags.tablePrint || *flags.classicPrint || *flags.outputDefault || *flags.routePath || *flags.outputPath != ""
		modes, err := resolveTraceModes(traceModeOptions{traditional: selected}, true)
		if err != nil || modes.mtr {
			t.Fatalf("%s: %#v, %v", flag, modes, err)
		}
	}
}

func TestEarlyModesRejectTracerouteBeforeExecution(t *testing.T) {
	for _, flag := range []string{"-k", "--traceroute"} {
		var out, errOut bytes.Buffer
		handled, code := maybeRunDNSModeWithAvailability(true, []string{"--dns", flag, "example.com"}, &out, &errOut, func([]string, io.Writer, io.Writer) int {
			t.Fatal("DNS runner called")
			return 0
		})
		if !handled || code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "cannot be combined") {
			t.Fatalf("DNS: %v/%d stdout %q stderr %q", handled, code, out.String(), errOut.String())
		}
		for _, args := range [][]string{{"--speed", flag}, {flag, "--speed"}} {
			out.Reset()
			errOut.Reset()
			handled, code = maybeRunSpeedModeWithAvailability(true, args, &out, &errOut)
			if !handled || code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "cannot be combined") {
				t.Fatalf("speed: %v/%d stdout %q stderr %q", handled, code, out.String(), errOut.String())
			}
		}
	}
	if containsTracerouteFlag([]string{"--", "--traceroute"}) {
		t.Fatal("interpreted positional argument as flag")
	}
}

func TestEarlyModesRejectTracerouteBeforeTerminator(t *testing.T) {
	for _, flag := range []string{"-k", "--traceroute"} {
		var out, errOut bytes.Buffer
		handled, code := maybeRunDNSModeWithAvailability(true, []string{"--dns", flag, "--", "example.com"}, &out, &errOut, func([]string, io.Writer, io.Writer) int {
			t.Fatal("DNS runner called")
			return 0
		})
		if !handled || code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "cannot be combined") {
			t.Fatalf("DNS: %v/%d stdout %q stderr %q", handled, code, out.String(), errOut.String())
		}
		out.Reset()
		errOut.Reset()
		handled, code = maybeRunSpeedModeWithAvailability(true, []string{"--speed", flag, "--", "example.com"}, &out, &errOut)
		if !handled || code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "cannot be combined") {
			t.Fatalf("speed: %v/%d stdout %q stderr %q", handled, code, out.String(), errOut.String())
		}
	}
}

func TestNoStopReasonFlagAvailability(t *testing.T) {
	parser := argparse.NewParser(appBinName, "")
	flags := registerTracerouteOutputFlags(parser)
	if *flags.noStopReason {
		t.Fatal("stop reason hidden by default")
	}
	err := parser.Parse([]string{appBinName, "--no-stop-reason"})
	if (err == nil) != enableTraceroute {
		t.Fatalf("--no-stop-reason parse in %s: %v", appBinName, err)
	}
	if *flags.noStopReason != enableTraceroute {
		t.Fatalf("--no-stop-reason value in %s = %v", appBinName, *flags.noStopReason)
	}
}
