//go:build windows

package mtu

import (
	"context"
	"net"
	"time"

	"github.com/nxtrace/NTrace-core/util"
	wd "github.com/xjasonlyu/windivert-go"
)

type winDivertCapture struct {
	ctx    context.Context
	cancel context.CancelFunc
	handle wd.Handle
	buf    []byte
	addr   wd.Address
}

func (c *winDivertCapture) Close() error {
	if c == nil {
		return nil
	}
	c.cancel()
	return c.handle.Close()
}

func (p *socketProber) beginICMPResponseCapture(ctx context.Context, deadline time.Time) (icmpResponseCapture, error) {
	handle, err := wd.Open(winDivertMTUFilter(p.ipVersion, p.udp.LocalAddr()), wd.LayerNetwork, 0, wd.FlagSniff|wd.FlagRecvOnly)
	if err != nil {
		return nil, nil
	}
	probeCtx, cancel := context.WithDeadline(ctx, deadline)
	go func() {
		<-probeCtx.Done()
		_ = handle.Close()
	}()

	_ = handle.SetParam(wd.QueueLength, 8192)
	_ = handle.SetParam(wd.QueueTime, 4000)

	return &winDivertCapture{
		ctx:    probeCtx,
		cancel: cancel,
		handle: handle,
		buf:    make([]byte, 65535),
	}, nil
}

func (p *socketProber) readICMPResponse(ctx context.Context, capture icmpResponseCapture, deadline time.Time, dstPort int, buf []byte) (probeResponse, error) {
	if resp, err, ok := p.readICMPResponseViaWinDivert(ctx, capture, dstPort); ok {
		return resp, err
	}
	return p.readICMPResponseFromSocket(ctx, deadline, dstPort, buf)
}

func (p *socketProber) readICMPResponseViaWinDivert(ctx context.Context, capture icmpResponseCapture, dstPort int) (probeResponse, error, bool) {
	winCapture, ok := capture.(*winDivertCapture)
	if !ok || winCapture == nil {
		return probeResponse{}, nil, false
	}
	for {
		if err := winCapture.ctx.Err(); err != nil {
			if ctx.Err() != nil {
				return probeResponse{}, ctx.Err(), true
			}
			return probeResponse{Event: EventTimeout}, nil, true
		}

		n, err := winCapture.handle.Recv(winCapture.buf, &winCapture.addr)
		if err != nil {
			if ctx.Err() != nil {
				return probeResponse{}, ctx.Err(), true
			}
			if winCapture.ctx.Err() != nil {
				return probeResponse{Event: EventTimeout}, nil, true
			}
			return probeResponse{}, err, true
		}
		peerIP, icmpMsg, ok := extractWinDivertICMPMessage(p.ipVersion, winCapture.buf[:n])
		if !ok {
			continue
		}
		resp, ok := parseICMPProbeResult(p.ipVersion, icmpMsg, peerIP, p.dstIP, dstPort, p.srcPort)
		if !ok {
			continue
		}
		return resp, nil, true
	}
}

func winDivertMTUFilter(ipVersion int, localAddr net.Addr) string {
	srcIP := util.AddrIP(localAddr)
	if srcIP == nil {
		if ipVersion == 6 {
			return "inbound and icmpv6"
		}
		return "inbound and icmp"
	}
	if ipVersion == 6 {
		return "inbound and icmpv6 and ipv6.DstAddr == " + srcIP.String()
	}
	return "inbound and icmp and ip.DstAddr == " + srcIP.String()
}

func extractWinDivertICMPMessage(ipVersion int, raw []byte) (net.IP, []byte, bool) {
	if len(raw) == 0 {
		return nil, nil, false
	}
	icmpMsg, err := util.GetICMPResponsePayload(raw)
	if err != nil || len(icmpMsg) == 0 {
		return nil, nil, false
	}

	switch ipVersion {
	case 4:
		if len(raw) < 20 || raw[0]>>4 != 4 {
			return nil, nil, false
		}
		return append(net.IP(nil), raw[12:16]...), append([]byte(nil), icmpMsg...), true
	case 6:
		if len(raw) < 40 || raw[0]>>4 != 6 {
			return nil, nil, false
		}
		return append(net.IP(nil), raw[8:24]...), append([]byte(nil), icmpMsg...), true
	default:
		return nil, nil, false
	}
}
