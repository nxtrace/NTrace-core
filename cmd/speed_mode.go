package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/akamensky/argparse"

	speedconfig "github.com/nxtrace/NTrace-core/internal/speedtest/config"
	speedrender "github.com/nxtrace/NTrace-core/internal/speedtest/render"
	speedrunner "github.com/nxtrace/NTrace-core/internal/speedtest/runner"
	"github.com/nxtrace/NTrace-core/wshandle"
)

type speedMetadataBackend interface {
	Close()
}

var newSpeedMetadataBackend = func(ctx context.Context) speedMetadataBackend {
	return wshandle.NewWithContextAsync(ctx)
}

func registerSpeedFlag(parser *argparse.Parser) *bool {
	return registerSpeedFlagWithAvailability(parser, enableSpeed)
}

func registerSpeedFlagWithAvailability(parser *argparse.Parser, enabled bool) *bool {
	if enabled {
		return parser.Flag("", "speed", &argparse.Options{Help: "Run CDN speed test mode. See `nexttrace --speed --help` for details"})
	}
	return ptrBool(false)
}

func maybeRunSpeedMode(rawArgs []string, stdout, stderr io.Writer) (bool, int) {
	return maybeRunSpeedModeWithAvailability(enableSpeed, rawArgs, stdout, stderr)
}

func maybeRunSpeedModeWithAvailability(enabled bool, rawArgs []string, stdout, stderr io.Writer) (bool, int) {
	if !containsSpeedFlag(rawArgs) {
		return false, 0
	}
	if !enabled {
		_, _ = fmt.Fprintf(stderr, "--speed is not available in %s; please use the full nexttrace build\n", appBinName)
		return true, 1
	}
	return true, runSpeedMode(rawArgs, stdout, stderr)
}

func containsSpeedFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--speed" {
			return true
		}
		if v, ok := strings.CutPrefix(arg, "--speed="); ok {
			return v == "" || strings.EqualFold(v, "true") || v == "1"
		}
	}
	return false
}

func runSpeedMode(rawArgs []string, stdout, stderr io.Writer) int {
	cfg, err := speedconfig.Load(rawArgs...)
	if err != nil {
		if errors.Is(err, speedconfig.ErrHelp) {
			_, _ = io.WriteString(stdout, speedconfig.Usage())
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "speed mode error: %v\n\n%s", err, speedconfig.Usage())
		return 1
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	restoreFastIPOutput := suppressSpeedMetadataOutput(cfg)
	defer restoreFastIPOutput()
	metadataBackend := startSpeedMetadataBackend(rootCtx, cfg)
	defer closeSpeedMetadataBackend(metadataBackend)

	var bus *speedrender.Bus
	isTTY := speedrender.IsTTY()
	if cfg.OutputJSON {
		bus = speedrender.NewBus(speedrender.NewPlainRenderer(io.Discard))
		isTTY = false
	} else if isTTY {
		bus = speedrender.NewBus(speedrender.NewTTYRenderer(stderr, cfg.NoColor))
	} else {
		bus = speedrender.NewBus(speedrender.NewPlainRenderer(stderr))
	}
	defer bus.Close()

	res := speedrunner.Run(rootCtx, cfg, bus, isTTY)
	if cfg.OutputJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(res); err != nil {
			_, _ = fmt.Fprintf(stderr, "failed to encode speed JSON: %v\n", err)
			return 1
		}
	}
	return res.ExitCode
}

func suppressSpeedMetadataOutput(cfg *speedconfig.Config) func() {
	if cfg == nil || cfg.NoMetadata || !cfg.OutputJSON {
		return func() {}
	}
	return setFastIPOutputSuppression(true)
}

func startSpeedMetadataBackend(ctx context.Context, cfg *speedconfig.Config) speedMetadataBackend {
	if cfg == nil || cfg.NoMetadata {
		return nil
	}
	return newSpeedMetadataBackend(ctx)
}

func closeSpeedMetadataBackend(backend speedMetadataBackend) {
	if backend != nil {
		backend.Close()
	}
}
