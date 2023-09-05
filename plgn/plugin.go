package plgn

import (
	"log"
	"net"
	"reflect"
	"strings"
)

type Plugin interface {
	OnDNSResolve(domain string) (net.IP, error)
	OnIPFound(ip net.Addr) error
	OnTTLChange(ttl int) error
}

var pluginRegistry = make(map[string]func(interface{}) Plugin)

func RegisterPlugin(name string, constructor func(interface{}) Plugin) {
	pluginRegistry[name] = constructor
}

func CreatePlugins(enabledPlugins string, params interface{}) []Plugin {
	var plugins []Plugin
	for _, name := range strings.Split(enabledPlugins, ",") {
		if constructor, exists := pluginRegistry[name]; exists {
			plugins = append(plugins, constructor(params))
		}
	}
	return plugins
}

func ExecuteHook(plugin Plugin, hookName string, args ...interface{}) {
	v := reflect.ValueOf(plugin)
	method := v.MethodByName(hookName)

	if !method.IsValid() {
		log.Printf("Method %s not found", hookName)
		return
	}

	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		in[i] = reflect.ValueOf(arg)
	}

	ret := method.Call(in)
	if len(ret) > 0 && !ret[0].IsNil() {
		err := ret[0].Interface().(error)
		log.Printf("Error in %s: %v", hookName, err)
	}
}
