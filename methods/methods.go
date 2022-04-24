package methods

import (
	"encoding/binary"
	"errors"
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

func GetIPHeaderLength(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, errors.New("received invalid IP header")
	}
	return int((data[0] & 0x0F) * 4), nil
}

func GetICMPResponsePayload(data []byte) ([]byte, error) {
	length, err := GetIPHeaderLength(data)
	if err != nil {
		return nil, err
	}

	if len(data) < length {
		return nil, errors.New("length of packet too short")
	}

	return data[length:], nil
}

func GetUDPSrcPort(data []byte) uint16 {
	srcPortBytes := data[:2]
	srcPort := binary.BigEndian.Uint16(srcPortBytes)
	return srcPort
}

func GetTCPSeq(data []byte) uint32 {
	seqBytes := data[4:8]
	return binary.BigEndian.Uint32(seqBytes)
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
