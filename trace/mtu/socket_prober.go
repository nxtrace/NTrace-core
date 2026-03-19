package mtu

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	traceinternal "github.com/nxtrace/NTrace-core/trace/internal"
	"github.com/nxtrace/NTrace-core/util"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type socketProber struct {
	ipVersion int
	dstIP     net.IP
	dstPort   int
	srcPort   int
	udp       *net.UDPConn
	icmp      net.PacketConn
	udp4      *ipv4.PacketConn
	udp6      *ipv6.PacketConn
	sendMu    sync.Mutex
}

var ErrWinDivertUnavailable = errors.New("windivert capture unavailable")

func newSocketProber(cfg Config) (*socketProber, error) {
	network := "udp4"
	icmpNetwork := "ip4:icmp"
	if cfg.ipVersion() == 6 {
		network = "udp6"
		icmpNetwork = "ip6:ipv6-icmp"
	}

	localAddr := &net.UDPAddr{IP: cfg.SrcIP, Port: cfg.SrcPort}
	udpConn, err := net.ListenUDP(network, localAddr)
	if err != nil {
		return nil, err
	}
	if err := configurePMTUSocket(udpConn, cfg.ipVersion()); err != nil {
		udpConn.Close()
		return nil, err
	}

	icmpConn, err := traceinternal.ListenPacket(icmpNetwork, cfg.SrcIP.String())
	if err != nil {
		udpConn.Close()
		return nil, err
	}

	prober := &socketProber{
		ipVersion: cfg.ipVersion(),
		dstIP:     append(net.IP(nil), cfg.DstIP...),
		dstPort:   cfg.DstPort,
		udp:       udpConn,
		icmp:      icmpConn,
	}
	if addr, ok := udpConn.LocalAddr().(*net.UDPAddr); ok && addr != nil {
		prober.srcPort = addr.Port
	}
	if prober.ipVersion == 6 {
		prober.udp6 = ipv6.NewPacketConn(udpConn)
	} else {
		prober.udp4 = ipv4.NewPacketConn(udpConn)
	}
	return prober, nil
}

func (p *socketProber) Close() error {
	if p == nil {
		return nil
	}
	if p.icmp != nil {
		_ = p.icmp.Close()
	}
	if p.udp != nil {
		return p.udp.Close()
	}
	return nil
}

func (p *socketProber) Probe(ctx context.Context, plan probePlan) (probeResponse, error) {
	if err := ctx.Err(); err != nil {
		return probeResponse{}, err
	}

	dstPort := probeDstPort(p.dstPort, plan.Token)
	payload := buildProbePayload(plan.PayloadSize)
	captureDeadline := deadlineFromStart(ctx, time.Now(), plan.Timeout)
	capture, err := p.beginICMPResponseCapture(ctx, captureDeadline)
	if err != nil {
		if !errors.Is(err, ErrWinDivertUnavailable) {
			return probeResponse{}, err
		}
	}
	if capture != nil {
		defer capture.Close()
	}
	startSend := time.Now()
	if err := p.send(plan.TTL, payload, dstPort); err != nil {
		if isSendSizeErr(err) {
			return probeResponse{}, &localMTUError{MTU: socketPathMTU(p.udp, p.ipVersion)}
		}
		return probeResponse{}, err
	}

	buf := make([]byte, 4096)
	deadline := deadlineFromStart(ctx, startSend, plan.Timeout)
	resp, err := p.readICMPResponse(ctx, capture, deadline, dstPort, buf)
	if err != nil {
		return probeResponse{}, err
	}
	resp.RTT = time.Since(startSend)
	return resp, nil
}

func (p *socketProber) send(ttl int, payload []byte, dstPort int) error {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()

	if p.ipVersion == 6 {
		if err := p.udp6.SetHopLimit(ttl); err != nil {
			return err
		}
	} else {
		if err := p.udp4.SetTTL(ttl); err != nil {
			return err
		}
	}
	_, err := p.udp.WriteToUDP(payload, &net.UDPAddr{IP: p.dstIP, Port: dstPort})
	return err
}

func probeDstPort(base int, token uint32) int {
	if base <= 0 || base > 65535 {
		base = 33494
	}
	if token == 0 {
		token = 1
	}
	maxOffset := 65535 - base
	if maxOffset <= 0 {
		return base
	}
	offset := int((token - 1) % uint32(maxOffset+1))
	return base + offset
}

func buildWinDivertMTUFilter(ipVersion int, srcIP net.IP) string {
	if srcIP == nil || srcIP.IsUnspecified() {
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

type icmpResponseCapture interface {
	Close() error
}

func deadlineFromStart(ctx context.Context, start time.Time, timeout time.Duration) time.Time {
	deadline := start.Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

func (p *socketProber) readICMPResponseFromSocket(ctx context.Context, deadline time.Time, dstPort int, buf []byte) (probeResponse, error) {
	for {
		if err := ctx.Err(); err != nil {
			return probeResponse{}, err
		}
		if err := p.icmp.SetReadDeadline(deadline); err != nil {
			return probeResponse{}, err
		}
		n, peer, err := p.icmp.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return probeResponse{}, ctx.Err()
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return probeResponse{Event: EventTimeout}, nil
			}
			if isRecvSizeErr(err) {
				continue
			}
			return probeResponse{}, err
		}
		resp, ok := parseICMPProbeResult(p.ipVersion, buf[:n], util.AddrIP(peer), p.dstIP, dstPort, p.srcPort)
		if !ok {
			continue
		}
		return resp, nil
	}
}
