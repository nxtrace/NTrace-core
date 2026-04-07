package trace

import "github.com/nxtrace/NTrace-core/trace/internal"

func applyICMPSourceDevice(spec *internal.ICMPSpec, osType int, sourceDevice string) {
	if spec == nil || sourceDevice == "" || osType == 2 {
		return
	}
	spec.SourceDevice = sourceDevice
}
