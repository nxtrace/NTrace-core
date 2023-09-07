package plgn

import (
	"net"

	"github.com/sjlleo/nexttrace-core/core"
)

type DefaultPlugin struct {
}

func (d *DefaultPlugin) OnDNSResolve(domain string) (net.IP, error) {
	return nil, nil
}

func (d *DefaultPlugin) OnNewIPFound(ip net.Addr) error {
	return nil
}

func (d *DefaultPlugin) OnTTLChange(ttl int) error {
	return nil
}

func (d *DefaultPlugin) OnTTLCompleted(ttl int, hop []core.Hop) error {
	return nil
}
