package plgn

import (
	"fmt"
	"net"

	"github.com/sjlleo/nexttrace-core/core"
)

type DebugPlugin struct {
	DefaultPlugin
	DebugLevel int
}

func NewDebugPlugin(params interface{}) core.Plugin {
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

func (d *DebugPlugin) OnNewIPFound(ip net.Addr) error {
	if d.DebugLevel <= 2 {
		fmt.Println("Debug Level 2: New IP Found: ", ip)
	}
	return nil
}

func (d *DebugPlugin) OnTTLCompleted(ttl int, hop []core.Hop) error {
	if d.DebugLevel <= 2 {
		fmt.Println("Debug Level 2: ttl=", ttl, "Hop:", hop)
	}
	return nil
}
