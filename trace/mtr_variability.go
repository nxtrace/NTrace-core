package trace

import "math"

// Keep successful samples in aggregator order, independently for each responder.
// Timeouts never enter this calculation. The first jitter is zero and counts in
// the mean, as in mtr. Interarrival jitter uses mtr's unscaled accumulator;
// retain floating-point precision instead of mtr's integer microsecond rounding.
func updateMTRVariability(acc *mtrHopAccum, rtt float64) {
	if rtt <= 0 {
		acc.hasZeroRTT = true
	} else {
		acc.logSum += math.Log(rtt)
	}
	if acc.received == 0 {
		acc.first = rtt
		return
	}
	acc.jitter = math.Abs(rtt - acc.last)
	acc.jitterSum += acc.jitter
	acc.jitterMax = max(acc.jitterMax, acc.jitter)
	acc.jitterInterarrival = acc.jitterInterarrival*15/16 + acc.jitter
}

// MigrateStats appends the source summary after the destination summary, matching
// its existing last-RTT policy. Join the successful sequences at their boundary.
// A capped summary retains derived statistics, not an actual truncated prefix.
func mergeMTRVariability(dst, src *mtrHopAccum) {
	if src.received == 0 {
		return
	}
	dst.logSum += src.logSum
	dst.hasZeroRTT = dst.hasZeroRTT || src.hasZeroRTT
	if dst.received == 0 {
		dst.first = src.first
		dst.jitter = src.jitter
		dst.jitterSum = src.jitterSum
		dst.jitterMax = src.jitterMax
		dst.jitterInterarrival = src.jitterInterarrival
		return
	}
	boundary := math.Abs(src.first - dst.last)
	dst.jitter = src.jitter
	if src.received == 1 {
		dst.jitter = boundary
	}
	dst.jitterSum += src.jitterSum + boundary
	dst.jitterMax = max(dst.jitterMax, src.jitterMax, boundary)
	decay := math.Pow(15.0/16, float64(src.received-1))
	dst.jitterInterarrival = (dst.jitterInterarrival*15/16+boundary)*decay + src.jitterInterarrival
}

func mtrGeometricMean(acc *mtrHopAccum) float64 {
	if acc.received == 0 || acc.hasZeroRTT {
		return 0
	}
	return math.Exp(acc.logSum / float64(acc.received))
}

func mtrJitterMean(acc *mtrHopAccum) float64 {
	if acc.received == 0 {
		return 0
	}
	return acc.jitterSum / float64(acc.received)
}
