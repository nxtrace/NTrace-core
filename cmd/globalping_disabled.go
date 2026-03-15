//go:build flavor_tiny || flavor_ntr

package cmd

import (
	"fmt"
	"os"

	"github.com/nxtrace/NTrace-core/trace"
)

func handleGlobalpingTrace(_ *trace.GlobalpingOptions, _ *trace.Config) {
	fmt.Fprintf(os.Stderr, "--from (Globalping) is not available in %s; please use the full nexttrace build\n", appBinName)
	os.Exit(1)
}
