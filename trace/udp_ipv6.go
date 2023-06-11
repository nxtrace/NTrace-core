package trace

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/sjlleo/nexttrace-core/util"
	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/sync/semaphore"
)

type UDPTracerv6 struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.Mutex

	icmp net.PacketConn

	final     int
	finalLock sync.Mutex

	sem *semaphore.Weighted
}

func (t *UDPTracerv6) GetConfig() *Config {
	return &t.Config
}

func (t *UDPTracerv6) SetConfig(c Config) {
	t.Config = c
}

func (t *UDPTracerv6) Execute() (*Result, error) {
	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	var err error
	t.icmp, err = icmp.ListenPacket("ip6:58", t.SrcAddr)
	if err != nil {
		return &t.res, err
	}
	defer t.icmp.Close()

	var cancel context.CancelFunc
	t.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	t.inflightRequest = make(map[int]chan Hop)
	t.final = -1

	go t.listenICMPv6()

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))
	for ttl := 1; ttl <= t.MaxHops; ttl++ {
		// 如果到达最终跳，则退出
		if t.final != -1 && ttl > t.final {
			break
		}
		for i := 0; i < t.NumMeasurements; i++ {
			t.wg.Add(1)
			go t.send(ttl)
			<-time.After(time.Millisecond * time.Duration(t.Config.PacketInterval))
		}
		<-time.After(time.Millisecond * time.Duration(t.Config.TTLInterval))
	}
	t.res.reduce(t.final)

	return &t.res, nil
}

func (t *UDPTracerv6) listenICMPv6() {
	lc := NewPacketListener(t.icmp, t.ctx)
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
			switch rm.Type {
			case ipv4.ICMPTypeTimeExceeded:
				t.handleICMPMessage(msg, rm.Body.(*icmp.TimeExceeded).Data)
			case ipv4.ICMPTypeDestinationUnreachable:
				t.handleICMPMessage(msg, rm.Body.(*icmp.DstUnreach).Data)
			default:
				// log.Println("received icmp message of unknown type", rm.Type)
			}
		}
	}

}

func (t *UDPTracerv6) handleICMPMessage(msg ReceivedMessage, data []byte) {
	header, err := util.GetICMPResponsePayload(data)
	if err != nil {
		return
	}
	srcPort := util.GetUDPSrcPort(header)
	t.inflightRequestLock.Lock()
	defer t.inflightRequestLock.Unlock()
	ch, ok := t.inflightRequest[int(srcPort)]
	if !ok {
		return
	}
	ch <- Hop{
		Address: msg.Peer,
	}
}

func (t *UDPTracerv6) getUDPConn(try int) (net.IP, int, net.PacketConn) {
	srcIP, _ := util.LocalIPPort(t.DestIP)

	var ipString string
	if srcIP == nil {
		ipString = ""
	} else {
		ipString = srcIP.String()
	}

	udpConn, err := net.ListenPacket("udp", "["+ipString+"]:0")
	if err != nil {
		if try > 3 {
			log.Fatal(err)
		}
		return t.getUDPConn(try + 1)
	}
	return srcIP, udpConn.LocalAddr().(*net.UDPAddr).Port, udpConn
}

func (t *UDPTracerv6) send(ttl int) error {
	err := t.sem.Acquire(context.Background(), 1)
	if err != nil {
		return err
	}
	defer t.sem.Release(1)

	defer t.wg.Done()
	if t.final != -1 && ttl > t.final {
		return nil
	}

	srcIP, srcPort, udpConn := t.getUDPConn(0)

	var payload []byte
	if t.Quic {
		payload = GenerateQuicPayloadWithRandomIds()
	} else {
		ipHeader := &layers.IPv4{
			SrcIP:    srcIP,
			DstIP:    t.DestIP,
			Protocol: layers.IPProtocolTCP,
			TTL:      uint8(ttl),
		}

		udpHeader := &layers.UDP{
			SrcPort: layers.UDPPort(srcPort),
			DstPort: layers.UDPPort(t.DestPort),
		}
		_ = udpHeader.SetNetworkLayerForChecksum(ipHeader)
		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}
		if err := gopacket.SerializeLayers(buf, opts, udpHeader, gopacket.Payload("HAJSFJHKAJSHFKJHAJKFHKASHKFHHKAFKHFAHSJK")); err != nil {
			return err
		}

		payload = buf.Bytes()
	}

	err = ipv4.NewPacketConn(udpConn).SetTTL(ttl)
	if err != nil {
		return err
	}

	start := time.Now()
	if _, err := udpConn.WriteTo(payload, &net.UDPAddr{IP: t.DestIP, Port: t.DestPort}); err != nil {
		return err
	}

	// 在对inflightRequest进行写操作的时候应该加锁保护，以免多个goroutine协程试图同时写入造成panic
	t.inflightRequestLock.Lock()
	hopCh := make(chan Hop)
	t.inflightRequest[srcPort] = hopCh
	t.inflightRequestLock.Unlock()
	defer func() {
		t.inflightRequestLock.Lock()
		close(hopCh)
		delete(t.inflightRequest, srcPort)
		t.inflightRequestLock.Unlock()
	}()

	go func() {
		reply := make([]byte, 1500)
		_, peer, err := udpConn.ReadFrom(reply)
		if err != nil {
			// probably because we closed the connection
			return
		}
		hopCh <- Hop{
			Address: peer,
		}
	}()

	select {
	case <-t.ctx.Done():
		return nil
	case h := <-hopCh:
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
		} else if addr, ok := h.Address.(*net.UDPAddr); ok && addr.IP.Equal(t.DestIP) {
			t.finalLock.Lock()
			if t.final == -1 || ttl < t.final {
				t.final = ttl
			}
			t.finalLock.Unlock()
		}

		h.TTL = ttl
		h.RTT = rtt

		t.res.add(h)

	case <-time.After(t.Timeout):
		if t.final != -1 && ttl > t.final {
			return nil
		}

		t.res.add(Hop{
			Address: nil,
			TTL:     ttl,
			RTT:     0,
			Error:   ErrHopLimitTimeout,
		})
	}

	return nil
}
