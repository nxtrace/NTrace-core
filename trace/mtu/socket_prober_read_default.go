//go:build !windows

package mtu

import (
	"context"
	"time"
)

func (p *socketProber) beginICMPResponseCapture(context.Context, time.Time) (icmpResponseCapture, error) {
	return nil, nil
}

func (p *socketProber) readICMPResponse(ctx context.Context, _ icmpResponseCapture, deadline time.Time, dstPort int, buf []byte) (probeResponse, error) {
	return p.readICMPResponseFromSocket(ctx, deadline, dstPort, buf)
}
