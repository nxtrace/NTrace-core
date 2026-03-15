//go:build !flavor_tiny && !flavor_ntr

package cmd

import (
	"github.com/nxtrace/NTrace-core/server"
)

func runDeploy(listenAddr string) error {
	return server.Run(listenAddr)
}
