package trace

import (
	"testing"
	"time"
)

func TestLookupTCPSentByAck(t *testing.T) {
	startA := time.Unix(10, 0)
	startB := time.Unix(20, 0)
	sentAt := map[int]sentInfo{
		100: {srcPort: 40000, payloadSize: 20, start: startA},
		200: {srcPort: 40000, payloadSize: 48, start: startB},
	}

	seq, start, ok := lookupTCPSentByAck(sentAt, 40000, tcpReplyAckForProbe(200, 48))
	if !ok {
		t.Fatal("lookupTCPSentByAck() ok = false")
	}
	if seq != 200 {
		t.Fatalf("seq = %d, want 200", seq)
	}
	if !start.Equal(startB) {
		t.Fatalf("start = %v, want %v", start, startB)
	}
}
