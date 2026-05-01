//go:build !flavor_tiny && !flavor_ntr

package cmd

import (
	"net"

	"github.com/nxtrace/NTrace-core/server"
)

func runDeploy(opts deployRunOptions, onReady func(net.Addr)) error {
	return server.RunWithOptions(server.Options{
		ListenAddr:  opts.ListenAddr,
		EnableMCP:   opts.EnableMCP,
		AuthEnabled: opts.AuthEnabled,
		DeployToken: opts.DeployToken,
	}, onReady)
}
