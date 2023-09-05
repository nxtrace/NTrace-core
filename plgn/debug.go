package plgn

import (
	"fmt"
	"net"
)

type DebugPlugin struct {
	DefaultPlugin
	DebugLevel int
}

func NewDebugPlugin(params interface{}) Plugin {
	debugLevel, ok := params.(int)
	if !ok {
		return nil
	}
	return &DebugPlugin{DebugLevel: debugLevel}
}

func (d *DebugPlugin) OnTTLChange(ttl int) error {
	if d.DebugLevel <= 2 {
		fmt.Println("Debug Level 2: TTL changed to", ttl)
	}
	return nil
}

func (d *DebugPlugin) OnIPFound(ip net.Addr) error {
	if d.DebugLevel <= 2 {
		fmt.Println("Debug Level 2: New IP Found: ", ip)
	}
	return nil
}
