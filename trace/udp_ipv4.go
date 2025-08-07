package trace

import (
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
	"golang.org/x/net/ipv4"
	"golang.org/x/sync/semaphore"
)

type UDPTracer struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.Mutex
	SrcIP               net.IP
	icmp                net.PacketConn
	udp                 net.PacketConn
	udpConn             *ipv4.RawConn
	hopLimitLock        sync.Mutex

	final     int
	finalLock sync.Mutex

	sem       *semaphore.Weighted
	fetchLock sync.Mutex
}

func (t *UDPTracer) PrintFunc() {
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

func (t *UDPTracer) launchTTL(ttl int) {
	go func(ttl int) {
		for i := 0; i < t.NumMeasurements; i++ {
			// 将 TTL 编码到高 8 位；将索引 i 编码到低 8 位
			seq := (ttl << 8) | (i & 0xFF)

			t.wg.Add(1)
			go t.send(ttl, seq)
			<-time.After(time.Millisecond * time.Duration(t.Config.PacketInterval))
		}
	}(ttl)
}

func (t *UDPTracer) Execute() (*Result, error) {
	// 初始化 inflightRequest map
	t.inflightRequestLock.Lock()
	t.inflightRequest = make(map[int]chan Hop)
	t.inflightRequestLock.Unlock()

	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	t.SrcIP, _ = util.LocalIPPort(t.DestIP)

	var err error

	if t.SrcAddr != "" {
		t.udp, err = net.ListenPacket("ip4:udp", t.SrcAddr)
	} else {
		t.udp, err = net.ListenPacket("ip4:udp", t.SrcIP.String())
	}
	if err != nil {
		return nil, err
	}
	defer t.udp.Close()

	t.udpConn, err = ipv4.NewRawConn(t.udp)
	if err != nil {
		return nil, err
	}

	t.icmp, err = icmp.ListenPacket("ip4:icmp", t.SrcAddr)
	if err != nil {
		return &t.res, err
	}
	defer t.icmp.Close()

	var cancel context.CancelFunc
	t.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	t.final = -1

	go t.listenICMP()
	t.wg.Add(1)
	go t.PrintFunc()

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))

	go func() {
		// 立即启动 BeginHop 对应的 TTL 组
		t.launchTTL(t.BeginHop)
		// 之后按 TTLInterval 周期启动后续 TTL 组
		ticker := time.NewTicker(time.Millisecond * time.Duration(t.Config.TTLInterval))
		defer ticker.Stop()

		for ttl := t.BeginHop + 1; ttl <= t.MaxHops; ttl++ {
			<-ticker.C
			// 如果到达最终跳，则退出
			if t.final != -1 && ttl > t.final {
				break
			}
			// 并发启动这个 TTL 的所有测量
			t.launchTTL(ttl)
		}
	}()
	t.wg.Wait()
	t.res.reduce(t.final)

	return &t.res, nil
}

func (t *UDPTracer) listenICMP() {
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
			rm, err := icmp.ParseMessage(1, msg.Msg[:*msg.N])
			if err != nil {
				log.Println(err)
				continue
			}
			switch rm.Type {
			case ipv4.ICMPTypeTimeExceeded:
				body := rm.Body.(*icmp.TimeExceeded)
				data := body.Data   // 内层原始 IP 包（IP 头 + 8B）
				if len(data) < 20 { // 内层 IP 头至少 20B
					continue
				}
				dstip := net.IP(data[16:20]) // 内层 IP 头目的地址
				if dstip.Equal(t.DestIP) {
					t.handleICMPMessage(msg, data)
				}
			case ipv4.ICMPTypeDestinationUnreachable:
				body := rm.Body.(*icmp.DstUnreach)
				data := body.Data
				if len(data) < 20 {
					continue
				}
				dstip := net.IP(data[16:20])
				if dstip.Equal(t.DestIP) {
					t.handleICMPMessage(msg, data)
				}
			default:
				//log.Println("received icmp message of unknown type", rm.Type)
			}
		}
	}
}

func (t *UDPTracer) handleICMPMessage(msg ReceivedMessage, data []byte) {
	seq, err := util.GetUDPSeq(data)
	if err != nil {
		return
	}

	t.inflightRequestLock.Lock()
	defer t.inflightRequestLock.Unlock()

	ch, ok := t.inflightRequest[int(seq)]
	if !ok {
		return
	}

	ch <- Hop{
		Success: true,
		Address: msg.Peer,
	}
}

func (t *UDPTracer) send(ttl, seq int) error {
	defer t.wg.Done()

	if err := t.sem.Acquire(t.ctx, 1); err != nil {
		return err
	}
	defer t.sem.Release(1)

	if t.final != -1 && ttl > t.final {
		return nil
	}

	t.inflightRequestLock.Lock()
	hopCh := make(chan Hop, 1)
	t.inflightRequest[seq] = hopCh
	t.inflightRequestLock.Unlock()
	defer func() {
		t.inflightRequestLock.Lock()
		delete(t.inflightRequest, seq)
		t.inflightRequestLock.Unlock()
	}()

	_, SrcPort := func() (net.IP, int) {
		if util.EnvRandomPort == "" && t.SrcPort != 0 {
			return nil, t.SrcPort
		}
		return util.LocalIPPort(t.DestIP)
	}()
	//var payload []byte
	//if t.Quic {
	//	payload = GenerateQuicPayloadWithRandomIds()
	//} else {
	ipHeader := &layers.IPv4{
		SrcIP:    t.SrcIP,
		DstIP:    t.DestIP,
		Id:       uint16(seq),
		Protocol: layers.IPProtocolUDP,
		TTL:      uint8(ttl),
		//Flags:    layers.IPv4DontFragment,
	}
	if t.DontFragment {
		ipHeader.Flags = layers.IPv4DontFragment
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
	//copy(buf.Bytes(), payload)
	if err := gopacket.SerializeLayers(buf, opts, ipHeader, udpHeader, gopacket.Payload(payload)); err != nil {
		return err
	}
	// 完整的报文字节
	packet := buf.Bytes()
	// 解析 IP 头长度（IHL）
	ihl := int(packet[0]&0x0f) * 4
	// 构造 ipv4.Header
	hdr, err := ipv4.ParseHeader(packet[:ihl])
	if err != nil {
		return err
	}
	// 串行设置 TTL + 发送，放在同一把锁里保证并发安全
	t.hopLimitLock.Lock()
	start := time.Now()
	if err := t.udpConn.WriteTo(hdr, packet[ihl:], nil); err != nil {
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
