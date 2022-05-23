package trace

import (
	"log"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

type ICMPTracerv6 struct {
	Config
	wg         sync.WaitGroup
	res        Result
	ctx        context.Context
	resCh      chan Hop
	icmpListen net.PacketConn
	final      int
	finalLock  sync.Mutex
}

func (t *ICMPTracerv6) Execute() (*Result, error) {
	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	var err error

	t.icmpListen, err = net.ListenPacket("ip6:58", "::")
	if err != nil {
		return &t.res, err
	}
	defer t.icmpListen.Close()

	var cancel context.CancelFunc
	t.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	t.resCh = make(chan Hop)
	t.final = -1

	go t.listenICMP()

	for ttl := 1; ttl <= t.MaxHops; ttl++ {
		if t.final != -1 && ttl > t.final {
			break
		}
		for i := 0; i < t.NumMeasurements; i++ {
			t.wg.Add(1)
			go t.send(ttl)
		}
		// 一组TTL全部退出（收到应答或者超时终止）以后，再进行下一个TTL的包发送
		t.wg.Wait()
		if t.RealtimePrinter != nil {
			t.RealtimePrinter(&t.res, ttl-1)
		}
	}
	t.res.reduce(t.final)

	return &t.res, nil
}

func (t *ICMPTracerv6) listenICMP() {
	lc := NewPacketListener(t.icmpListen, t.ctx)
	go lc.Start()
	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-lc.Messages:
			if msg.N == nil {
				continue
			}
			rm, err := icmp.ParseMessage(58, msg.Msg[:*msg.N])
			if err != nil {
				log.Println(err)
				continue
			}
			// log.Println(msg.Peer)
			switch rm.Type {
			case ipv6.ICMPTypeTimeExceeded:
				t.handleICMPMessage(msg, 0, rm.Body.(*icmp.TimeExceeded).Data)
			case ipv6.ICMPTypeEchoReply:
				t.handleICMPMessage(msg, 1, rm.Body.(*icmp.Echo).Data)
			default:
				// log.Println("received icmp message of unknown type", rm.Type)
			}
		}
	}

}

func (t *ICMPTracerv6) handleICMPMessage(msg ReceivedMessage, icmpType int8, data []byte) {
	t.resCh <- Hop{
		Success: true,
		Address: msg.Peer,
	}
}

func (t *ICMPTracerv6) send(ttl int) error {
	defer t.wg.Done()
	if t.final != -1 && ttl > t.final {
		return nil
	}

	icmpHeader := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest, Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Data: []byte("HELLO-R-U-THERE"),
		},
	}

	p := ipv6.NewPacketConn(t.icmpListen)

	icmpHeader.Body.(*icmp.Echo).Seq = ttl
	p.SetHopLimit(ttl)

	wb, err := icmpHeader.Marshal(nil)
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	if _, err := t.icmpListen.WriteTo(wb, &net.IPAddr{IP: t.DestIP}); err != nil {
		log.Fatal(err)
	}
	if err := t.icmpListen.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		log.Fatal(err)
	}

	select {
	case <-t.ctx.Done():
		return nil
	case h := <-t.resCh:
		rtt := time.Since(start)
		if t.final != -1 && ttl > t.final {
			return nil
		}
		if addr, ok := h.Address.(*net.IPAddr); ok && addr.IP.Equal(t.DestIP) {
			t.finalLock.Lock()
			if t.final == -1 || ttl < t.final {
				t.final = ttl
			}
			t.finalLock.Unlock()
		} else if addr, ok := h.Address.(*net.TCPAddr); ok && addr.IP.Equal(t.DestIP) {
			t.finalLock.Lock()
			if t.final == -1 || ttl < t.final {
				t.final = ttl
			}
			t.finalLock.Unlock()
		}

		h.TTL = ttl
		h.RTT = rtt

		h.fetchIPData(t.Config)

		t.res.add(h)

	case <-time.After(t.Timeout):
		if t.final != -1 && ttl > t.final {
			return nil
		}

		t.res.add(Hop{
			Success: false,
			Address: nil,
			TTL:     ttl,
			RTT:     0,
			Error:   ErrHopLimitTimeout,
		})
	}

	return nil
}
