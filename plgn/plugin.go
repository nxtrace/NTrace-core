package plgn

import (
	"log"
	"reflect"
	"strings"

	"github.com/sjlleo/nexttrace-core/core"
)

var pluginRegistry = make(map[string]func(interface{}) core.Plugin)

func RegisterPlugin(name string, constructor func(interface{}) core.Plugin) {
	pluginRegistry[name] = constructor
}

func CreatePlugins(enabledPlugins string, params interface{}) []core.Plugin {
	var plugins []core.Plugin
	for _, name := range strings.Split(enabledPlugins, ",") {
		if constructor, exists := pluginRegistry[name]; exists {
			plugins = append(plugins, constructor(params))
		}
	}
	return plugins
}

func ExecuteHook(plugin core.Plugin, hookName string, args ...interface{}) {
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
