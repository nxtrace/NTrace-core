package cmd

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/trace"
)

func TestTraceRunErrorExitStatus(t *testing.T) {
	for _, mode := range []string{"failure", "report", "canceled", "success"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			process := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTraceRunErrorProcess$")
			process.Env = append(os.Environ(), "NTRACE_TEST_TRACE_ERROR="+mode)
			var stdout, stderr bytes.Buffer
			process.Stdout, process.Stderr = &stdout, &stderr
			err := process.Run()
			if mode == "failure" || mode == "report" {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 1 || stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("probe setup failed")) {
					t.Fatalf("error=%v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
				}
			} else if err != nil || stderr.Len() != 0 {
				t.Fatalf("error=%v stderr=%s", err, stderr.String())
			}
		})
	}
}

func TestTraceRunErrorProcess(t *testing.T) {
	switch os.Getenv("NTRACE_TEST_TRACE_ERROR") {
	case "failure":
		exitOnTraceRunError(errors.New("probe setup failed"))
	case "report":
		runMTRReportFn = func(context.Context, trace.Method, trace.Config, trace.MTROptions, trace.MTROnSnapshot) error {
			return errors.New("probe setup failed")
		}
		maybeRunMTRMode(effectiveMTRModes{mtr: true, report: true}, trace.ICMPTrace,
			trace.Config{SrcAddr: "127.0.0.1", DstIP: net.IPv4(127, 0, 0, 1)}, true, 1, false, 0,
			"127.0.0.1", "disable-geoip", false, 0, mtrTUIOptions{})
	case "canceled":
		exitOnTraceRunError(context.Canceled)
	case "success":
		exitOnTraceRunError(nil)
	}
}
