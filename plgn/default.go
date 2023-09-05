package plgn

import "net"

type DefaultPlugin struct {
}

func (d *DefaultPlugin) OnDNSResolve(domain string) (net.IP, error) {
	return nil, nil
}

func (d *DefaultPlugin) OnIPFound(ip net.Addr) error {
	return nil
}

func (d *DefaultPlugin) OnTTLChange(ttl int) error {
	return nil
}
