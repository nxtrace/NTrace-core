package trace

import "time"

func tcpReplyAckForProbe(seq int, payloadSize int) int {
	return int(uint32(seq) + 1 + uint32(payloadSize))
}

func lookupTCPSentByAck(sentAt map[int]sentInfo, srcPort, ack int) (seq int, start time.Time, ok bool) {
	for candidateSeq, info := range sentAt {
		if info.srcPort != srcPort {
			continue
		}
		if tcpReplyAckForProbe(candidateSeq, info.payloadSize) != ack {
			continue
		}
		return candidateSeq, info.start, true
	}
	return 0, time.Time{}, false
}
