//go:build !flavor_tiny && !flavor_ntr

package cmd

import (
	"net"

	"github.com/nxtrace/NTrace-core/server"
)

func runDeploy(listenAddr string, onReady func(net.Addr)) error {
	return server.RunWithReady(listenAddr, onReady)
}
