package trace

import (
	"errors"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/nxtrace/NTrace-core/util"
	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
	"golang.org/x/sync/semaphore"
)

type UDPTracerIPv6 struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.Mutex
	SrcIP               net.IP
	icmp                net.PacketConn
	udp                 net.PacketConn
	udpConn             *ipv6.PacketConn
	hopLimitLock        sync.Mutex

	final     int
	finalLock sync.Mutex

	sem       *semaphore.Weighted
	fetchLock sync.Mutex
	udpMutex  sync.Mutex
}

func (t *UDPTracerIPv6) PrintFunc() {
	defer t.wg.Done()
	ttl := t.Config.BeginHop - 1
	for {
		if t.AsyncPrinter != nil {
			t.AsyncPrinter(&t.res)
		}
		// 接收的时候检查一下是不是 3 跳都齐了
		if ttl < len(t.res.Hops) &&
			len(t.res.Hops[ttl]) == t.NumMeasurements {
			if t.RealtimePrinter != nil {
				t.RealtimePrinter(&t.res, ttl)
			}
			ttl++
			if ttl == t.final || ttl >= t.MaxHops {
				return
			}
		}
		<-time.After(200 * time.Millisecond)
	}
}

func (t *UDPTracerIPv6) Execute() (res *Result, err error) {
	// 初始化 inflightRequest map
	t.inflightRequestLock.Lock()
	t.inflightRequest = make(map[int]chan Hop)
	t.inflightRequestLock.Unlock()

	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	t.SrcIP, _ = util.LocalIPPortv6(t.DestIP)

	if t.SrcAddr != "" {
		t.udp, err = net.ListenPacket("ip6:udp", t.SrcAddr)
	} else {
		t.udp, err = net.ListenPacket("ip6:udp", t.SrcIP.String())
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if c := t.udp; c != nil { // 先拷一份引用，避免 defer 执行时 t.udp 已被并发改写
			err = errors.Join(err, c.Close())
		}
	}()

	t.udpConn = ipv6.NewPacketConn(t.udp)

	t.icmp, err = icmp.ListenPacket("ip6:ipv6-icmp", t.SrcIP.String())
	if err != nil {
		return &t.res, err
	}
	defer func() {
		if c := t.icmp; c != nil { // 先拷一份引用，避免 defer 执行时 t.icmp 已被并发改写
			err = errors.Join(err, c.Close())
		}
	}()

	var cancel context.CancelFunc
	t.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	t.final = -1

	go t.listenICMP()
	t.wg.Add(1)
	go t.PrintFunc()

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))

	for ttl := t.BeginHop; ttl <= t.MaxHops; ttl++ {
		// 如果到达最终跳，则退出
		if t.final != -1 && ttl > t.final {
			break
		}
		for i := 0; i < t.NumMeasurements; i++ {
			t.wg.Add(1)
			go func(ttl int) {
				if err := t.send(ttl); err != nil {
					log.Printf("send failed: ttl=%d: %v", ttl, err)
				}
			}(ttl)
			<-time.After(time.Millisecond * time.Duration(t.Config.PacketInterval))
		}
		<-time.After(time.Millisecond * time.Duration(t.Config.TTLInterval))
	}
	t.wg.Wait()
	t.res.reduce(t.final)

	return &t.res, nil
}

func (t *UDPTracerIPv6) listenICMP() {
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
			case ipv6.ICMPTypeTimeExceeded:
				body := rm.Body.(*icmp.TimeExceeded)
				data := body.Data
				if len(data) < 40 || data[0]>>4 != 6 {
					continue
				}
				dstip := net.IP(data[24:40])
				if dstip.Equal(t.DestIP) {
					t.handleICMPMessage(msg)
				}
			case ipv6.ICMPTypeDestinationUnreachable:
				body := rm.Body.(*icmp.DstUnreach)
				data := body.Data
				if len(data) < 40 || data[0]>>4 != 6 {
					continue
				}
				dstip := net.IP(data[24:40])
				if dstip.Equal(t.DestIP) {
					t.handleICMPMessage(msg)
				}
			default:
				//log.Println("received icmp message of unknown type", rm.Type)
			}
		}
	}
}

func (t *UDPTracerIPv6) handleICMPMessage(msg ReceivedMessage) {
	// 消息至少要有 IPv6 基本头 (40B) + ICMPv6 头 (8B)
	if msg.N == nil || *msg.N < 48 {
		return
	}

	raw := msg.Msg[:*msg.N]
	inner := raw[8:]

	header, err := util.GetICMPResponsePayload(inner)
	if err != nil {
		return
	}

	packet := gopacket.NewPacket(header, layers.LayerTypeUDP, gopacket.Default)
	udpLayer := packet.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		return
	}

	origUDP := udpLayer.(*layers.UDP)
	SrcPort := int(origUDP.SrcPort)
	// 取出通道后立刻解锁
	t.inflightRequestLock.Lock()
	ch, ok := t.inflightRequest[SrcPort]
	t.inflightRequestLock.Unlock()
	if !ok || ch == nil {
		return
	}

	h := Hop{
		Success: true,
		Address: msg.Peer,
	}
	// 非阻塞发送，避免重复回包把缓冲塞满导致阻塞
	select {
	case ch <- h:
	default:
		// 丢弃重复/迟到的相同 icmp 回包，避免阻塞
	}
}

func (t *UDPTracerIPv6) send(ttl int) error {
	defer t.wg.Done()

	if util.EnvRandomPort == "" {
		t.udpMutex.Lock()
		defer t.udpMutex.Unlock()
	}

	if err := t.sem.Acquire(t.ctx, 1); err != nil {
		return err
	}
	defer t.sem.Release(1)

	if t.final != -1 && ttl > t.final {
		return nil
	}

	_, SrcPort := func() (net.IP, int) {
		if util.EnvRandomPort == "" && t.SrcPort != 0 {
			return nil, t.SrcPort
		}
		return util.LocalIPPortv6(t.DestIP)
	}()

	t.inflightRequestLock.Lock()
	hopCh := make(chan Hop, 1)
	t.inflightRequest[SrcPort] = hopCh
	t.inflightRequestLock.Unlock()
	defer func() {
		t.inflightRequestLock.Lock()
		delete(t.inflightRequest, SrcPort)
		close(hopCh)
		t.inflightRequestLock.Unlock()
	}()

	ipHeader := &layers.IPv6{
		SrcIP:      t.SrcIP,
		DstIP:      t.DestIP,
		HopLimit:   uint8(ttl),
		NextHeader: layers.IPProtocolUDP,
	}

	udpHeader := &layers.UDP{
		SrcPort: layers.UDPPort(SrcPort),
		DstPort: layers.UDPPort(t.DestPort),
	}
	_ = udpHeader.SetNetworkLayerForChecksum(ipHeader)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	desiredPayloadSize := t.Config.PktSize
	payload := make([]byte, desiredPayloadSize)
	// 设置随机种子
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range payload {
		payload[i] = byte(r.Intn(256))
	}

	if err := gopacket.SerializeLayers(buf, opts, udpHeader, gopacket.Payload(payload)); err != nil {
		return err
	}
	// 串行设置 HopLimit + 发送，放在同一把锁里保证并发安全
	t.hopLimitLock.Lock()
	if err := t.udpConn.SetHopLimit(ttl); err != nil {
		t.hopLimitLock.Unlock()
		return err
	}
	start := time.Now()
	if _, err := t.udp.WriteTo(buf.Bytes(), &net.IPAddr{IP: t.DestIP}); err != nil {
		t.hopLimitLock.Unlock()
		return err
	}
	t.hopLimitLock.Unlock()

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

		t.fetchLock.Lock()
		defer t.fetchLock.Unlock()
		if err := h.fetchIPData(t.Config); err != nil {
			return err
		}

		t.res.addLegacy(h)
	case <-time.After(t.Timeout):
		if t.final != -1 && ttl > t.final {
			return nil
		}

		h := Hop{
			Success: false,
			Address: nil,
			TTL:     ttl,
			RTT:     0,
			Error:   ErrHopLimitTimeout,
		}
		t.res.addLegacy(h)
	}
	return nil
}
