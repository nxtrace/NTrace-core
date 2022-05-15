package methods

import (
	"net"
	"time"
)

// TracerouteHop type
type TracerouteHop struct {
	Success bool
	Address net.Addr
	TTL     uint16
	RTT     *time.Duration
}

type TracerouteConfig struct {
	MaxHops          uint16
	NumMeasurements  uint16
	ParallelRequests uint16

	Port    int
	Timeout time.Duration
}

func ReduceFinalResult(preliminary map[uint16][]TracerouteHop, maxHops uint16, destIP net.IP) map[uint16][]TracerouteHop {
	// reduce the results to remove all hops after the first encounter to final destination
	finalResults := map[uint16][]TracerouteHop{}
	for i := uint16(1); i < maxHops; i++ {
		foundFinal := false
		probes := preliminary[i]
		if probes == nil {
			break
		}
		finalResults[i] = []TracerouteHop{}
		for _, probe := range probes {
			if probe.Success && probe.Address.String() == destIP.String() {
				foundFinal = true
			}
			finalResults[i] = append(finalResults[i], probe)
		}
		if foundFinal {
			break
		}
	}
	return finalResults
}
