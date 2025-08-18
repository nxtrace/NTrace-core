package trace

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
	"golang.org/x/sync/semaphore"

	"github.com/nxtrace/NTrace-core/util"
)

type UDPTracerIPv6 struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.RWMutex
	SrcIP               net.IP
	icmp                net.PacketConn
	udp                 net.PacketConn
	udpConn             *ipv6.PacketConn
	hopLimitLock        sync.Mutex
	final               atomic.Int32
	sem                 *semaphore.Weighted
	udpMutex            sync.Mutex
}

func (t *UDPTracerIPv6) PrintFunc(ctx context.Context, status chan<- bool) {
	defer t.wg.Done()
	// 默认认为是正常退出，只有在 ctx 取消时改为 false
	normalExit := true
	defer func() {
		select {
		case status <- normalExit:
		default:
		}
		close(status)
	}()
	ttl := t.Config.BeginHop - 1
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
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
			if ttl == int(t.final.Load()) || ttl >= t.MaxHops {
				return
			}
		}
		select {
		case <-ctx.Done():
			// 非正常退出
			normalExit = false
			return
		case <-ticker.C:
		}
	}
}

func (t *UDPTracerIPv6) Execute() (res *Result, err error) {
	// 初始化 inflightRequest map
	t.inflightRequest = make(map[int]chan Hop)

	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}
	// 初始化 Result.Hops，并预分配到 MaxHops
	t.res.Hops = make([][]Hop, t.MaxHops)
	// 解析并校验用户指定的 IPv6 源地址
	SrcAddr := net.ParseIP(t.SrcAddr)
	if t.SrcAddr != "" && (SrcAddr == nil || SrcAddr.To4() != nil || SrcAddr.To16() == nil) {
		return nil, errors.New("invalid IPv6 SrcAddr: " + t.SrcAddr)
	}

	t.SrcIP, _ = util.LocalIPPortv6(t.DestIP, SrcAddr, "udp6")
	if t.SrcIP == nil {
		return nil, errors.New("cannot determine local IPv6 address")
	}

	t.udp, err = net.ListenPacket("ip6:udp", t.SrcIP.String())
	if err != nil {
		return nil, err
	}
	defer func() {
		if c := t.udp; c != nil { // 先拷一份引用，避免 defer 执行时 t.udp 已被并发改写
			_ = c.Close()
		}
	}()

	t.udpConn = ipv6.NewPacketConn(t.udp)

	t.icmp, err = icmp.ListenPacket("ip6:ipv6-icmp", t.SrcIP.String())
	if err != nil {
		return &t.res, err
	}
	defer func() {
		if c := t.icmp; c != nil { // 先拷一份引用，避免 defer 执行时 t.icmp 已被并发改写
			_ = c.Close()
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	t.final.Store(-1)

	t.wg.Add(1)
	go t.listenICMP(ctx)
	statusCh := make(chan bool, 1)
	t.wg.Add(1)
	go t.PrintFunc(ctx, statusCh)

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))

ttlLoop:
	for ttl := t.BeginHop; ttl <= t.MaxHops; ttl++ {
		// 如果到达最终跳，则退出
		if f := t.final.Load(); f != -1 && ttl > int(f) {
			break
		}
		for i := 0; i < t.NumMeasurements; i++ {
			// 若 ctx 已取消，则不再发起新的尝试
			if ctx.Err() != nil {
				break ttlLoop
			}
			t.wg.Add(1)
			go func(ttl int) {
				if err := t.send(ctx, ttl); err != nil && !errors.Is(err, context.Canceled) {
					log.Printf("send failed: ttl=%d: %v", ttl, err)
				}
			}(ttl)
			select {
			case <-ctx.Done():
				break ttlLoop
			case <-time.After(time.Millisecond * time.Duration(t.Config.PacketInterval)):
			}
		}
		select {
		case <-ctx.Done():
			break ttlLoop
		case <-time.After(time.Millisecond * time.Duration(t.Config.TTLInterval)):
		}
	}
	normalExit := <-statusCh
	stop()
	t.wg.Wait()
	t.res.reduce(int(t.final.Load()))
	if !normalExit {
		return &t.res, context.Canceled
	}
	return &t.res, nil
}

func (t *UDPTracerIPv6) listenICMP(ctx context.Context) {
	defer t.wg.Done()
	lc := NewPacketListener(t.icmp)
	go lc.Start(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-lc.Messages:
			if !ok {
				return
			}
			if msg.N == nil {
				continue
			}
			rm, err := icmp.ParseMessage(58, msg.Msg[:*msg.N])
			if err != nil {
				log.Println(err)
				continue
			}
			var data []byte
			switch rm.Type {
			case ipv6.ICMPTypeTimeExceeded:
				body, ok := rm.Body.(*icmp.TimeExceeded)
				if !ok || body == nil {
					continue
				}
				data = body.Data
			case ipv6.ICMPTypeDestinationUnreachable:
				body, ok := rm.Body.(*icmp.DstUnreach)
				if !ok || body == nil {
					continue
				}
				data = body.Data
			default:
				continue
				//log.Println("received icmp message of unknown type", rm.Type)
			}
			if len(data) < 40 || data[0]>>4 != 6 {
				continue
			}
			dstip := net.IP(data[24:40])
			if dstip.Equal(t.DestIP) || dstip.Equal(net.IPv6zero) {
				t.handleICMPMessage(msg)
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
	t.inflightRequestLock.RLock()
	ch, ok := t.inflightRequest[SrcPort]
	t.inflightRequestLock.RUnlock()
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

func (t *UDPTracerIPv6) send(ctx context.Context, ttl int) error {
	defer t.wg.Done()
	if !util.RandomPortEnabled() {
		t.udpMutex.Lock()
		defer t.udpMutex.Unlock()
	}

	if err := t.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer t.sem.Release(1)

	if f := t.final.Load(); f != -1 && ttl > int(f) {
		return nil
	}

	_, SrcPort := func() (net.IP, int) {
		if !util.RandomPortEnabled() && t.SrcPort > 0 {
			return nil, t.SrcPort
		}
		return util.LocalIPPortv6(t.DestIP, t.SrcIP, "udp6")
	}()

	t.inflightRequestLock.Lock()
	hopCh := make(chan Hop, 1)
	t.inflightRequest[SrcPort] = hopCh
	t.inflightRequestLock.Unlock()
	defer func() {
		t.inflightRequestLock.Lock()
		delete(t.inflightRequest, SrcPort)
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
	case <-ctx.Done():
		return context.Canceled
	case h := <-hopCh:
		rtt := time.Since(start)
		if f := t.final.Load(); f != -1 && ttl > int(f) {
			return nil
		}

		if ip := util.AddrIP(h.Address); ip != nil && ip.Equal(t.DestIP) {
			for {
				old := t.final.Load()
				if old != -1 && ttl >= int(old) {
					break
				}
				if t.final.CompareAndSwap(old, int32(ttl)) {
					break
				}
			}
		}

		h.TTL = ttl
		h.RTT = rtt

		if err := h.fetchIPData(t.Config); err != nil {
			return err
		}
		t.res.addLegacy(h)
	case <-time.After(t.Timeout):
		if f := t.final.Load(); f != -1 && ttl > int(f) {
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
