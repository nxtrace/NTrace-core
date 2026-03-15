//go:build flavor_tiny || flavor_ntr

package cmd

import "fmt"

func runDeploy(_ string) error {
	return fmt.Errorf("WebUI (--deploy) is not available in %s; please use the full nexttrace build", appBinName)
}
