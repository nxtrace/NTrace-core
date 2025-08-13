package trace

import (
	"errors"
	"log"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/nxtrace/NTrace-core/util"
	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/sync/semaphore"
)

type TCPTracer struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.RWMutex
	SrcIP               net.IP
	icmp                net.PacketConn
	tcp                 net.PacketConn
	tcpConn             *ipv4.PacketConn
	hopLimitLock        sync.Mutex
	final               atomic.Int32
	sem                 *semaphore.Weighted
}

func (t *TCPTracer) PrintFunc() {
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
			if ttl == int(t.final.Load()) || ttl >= t.MaxHops {
				return
			}
		}
		<-time.After(200 * time.Millisecond)
	}
}

func (t *TCPTracer) ttlComp(ttl int) bool {
	idx := ttl - 1
	t.res.lock.RLock()
	defer t.res.lock.RUnlock()
	return idx < len(t.res.Hops) && len(t.res.Hops[idx]) >= t.NumMeasurements
}

func (t *TCPTracer) launchTTL(ttl int) {
	go func(ttl int) {
		for i := 0; i < t.MaxAttempts; i++ {
			// 若此 TTL 已完成，则不再发起新的尝试
			if t.ttlComp(ttl) {
				return
			}
			t.wg.Add(1)
			go func(ttl, i int) {
				if err := t.send(ttl, i); err != nil {
					log.Printf("send failed: ttl=%d i=%d: %v", ttl, i, err)
				}
			}(ttl, i)
			<-time.After(time.Millisecond * time.Duration(t.Config.PacketInterval))
		}
	}(ttl)
}

func (t *TCPTracer) Execute() (res *Result, err error) {
	// 初始化 inflightRequest map
	t.inflightRequest = make(map[int]chan Hop)

	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}
	// 初始化 Result.Hops，并预分配到 MaxHops
	t.res.Hops = make([][]Hop, t.MaxHops)

	t.SrcIP, _ = util.LocalIPPort(t.DestIP)

	if t.SrcAddr != "" {
		t.tcp, err = net.ListenPacket("ip4:tcp", t.SrcAddr)
	} else {
		t.tcp, err = net.ListenPacket("ip4:tcp", t.SrcIP.String())
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if c := t.tcp; c != nil { // 先拷一份引用，避免 defer 执行时 t.tcp 已被并发改写
			err = errors.Join(err, c.Close())
		}
	}()

	t.tcpConn = ipv4.NewPacketConn(t.tcp)

	t.icmp, err = icmp.ListenPacket("ip4:icmp", t.SrcAddr)
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

	t.final.Store(-1)

	go t.listenICMP()
	go t.listenTCP()
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
			if f := t.final.Load(); f != -1 && ttl > int(f) {
				break
			}
			// 并发启动这个 TTL 的所有测量
			t.launchTTL(ttl)
		}
	}()
	t.wg.Wait()
	t.res.reduce(int(t.final.Load()))

	return &t.res, nil
}

func (t *TCPTracer) listenICMP() {
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
				data := body.Data
				if len(data) < 20 || data[0]>>4 != 4 {
					continue
				}
				dstip := net.IP(data[16:20])
				if dstip.Equal(t.DestIP) {
					t.handleICMPMessage(msg, data)
				}
			case ipv4.ICMPTypeDestinationUnreachable:
				body := rm.Body.(*icmp.DstUnreach)
				data := body.Data
				if len(data) < 20 || data[0]>>4 != 4 {
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

// @title    listenTCP
// @description   监听TCP的响应数据包
func (t *TCPTracer) listenTCP() {
	lc := NewPacketListener(t.tcp, t.ctx)
	go lc.Start()

	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-lc.Messages:
			if msg.N == nil {
				continue
			}
			if ip, ok := msg.Peer.(*net.IPAddr); ok && ip.IP.Equal(t.DestIP) {
				// 解包
				packet := gopacket.NewPacket(msg.Msg[:*msg.N], layers.LayerTypeTCP, gopacket.Default)
				// 从包中获取TCP layer信息
				if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
					tcp, _ := tcpLayer.(*layers.TCP)
					// 从对端返回的 ACK-1 恢复出原始探测包的 seq
					seq := int(tcp.Ack - 1)
					// 取出通道后立刻解锁
					t.inflightRequestLock.RLock()
					ch, ok := t.inflightRequest[seq]
					t.inflightRequestLock.RUnlock()
					if !ok || ch == nil {
						continue
					}

					h := Hop{
						Success: true,
						Address: msg.Peer,
					}
					// 非阻塞发送，避免重复回包把缓冲塞满导致阻塞
					select {
					case ch <- h:
					default:
						// 丢弃重复/迟到的相同 seq 回包，避免阻塞
					}
				}
			}
		}
	}
}

func (t *TCPTracer) handleICMPMessage(msg ReceivedMessage, data []byte) {
	header, err := util.GetICMPResponsePayload(data)
	if err != nil {
		return
	}

	seq, err := util.GetTCPSeq(header)
	if err != nil {
		return
	}
	// 取出通道后立刻解锁
	t.inflightRequestLock.RLock()
	ch, ok := t.inflightRequest[int(seq)]
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
		// 丢弃重复/迟到的相同 seq 回包，避免阻塞
	}
}

func (t *TCPTracer) send(ttl, i int) error {
	defer t.wg.Done()

	if t.ttlComp(ttl) {
		// 快路径短路：若该 TTL 已完成，直接返回避免竞争信号量与无谓发包
		return nil
	}

	if err := t.sem.Acquire(t.ctx, 1); err != nil {
		return err
	}
	defer t.sem.Release(1)

	if f := t.final.Load(); f != -1 && ttl > int(f) {
		return nil
	}

	if t.ttlComp(ttl) {
		// 竞态兜底：获取信号量期间可能已完成，再次检查以避免冗余发包
		return nil
	}
	// 将 TTL 编码到高 8 位；将索引 i 编码到低 24 位
	seq := (ttl << 24) | (i & 0xFFFFFF)

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

	ipHeader := &layers.IPv4{
		SrcIP:    t.SrcIP,
		DstIP:    t.DestIP,
		Protocol: layers.IPProtocolTCP,
		TTL:      uint8(ttl),
		//Flags:    layers.IPv4DontFragment,
	}
	if t.DontFragment {
		ipHeader.Flags = layers.IPv4DontFragment
	}

	tcpHeader := &layers.TCP{
		SrcPort: layers.TCPPort(SrcPort),
		DstPort: layers.TCPPort(t.DestPort),
		Seq:     uint32(seq),
		SYN:     true,
		Window:  14600,
	}
	_ = tcpHeader.SetNetworkLayerForChecksum(ipHeader)

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
	if err := gopacket.SerializeLayers(buf, opts, tcpHeader, gopacket.Payload(payload)); err != nil {
		return err
	}
	// 串行设置 TTL + 发送，放在同一把锁里保证并发安全
	t.hopLimitLock.Lock()
	if err := t.tcpConn.SetTTL(ttl); err != nil {
		t.hopLimitLock.Unlock()
		return err
	}
	start := time.Now()
	if _, err := t.tcp.WriteTo(buf.Bytes(), &net.IPAddr{IP: t.DestIP}); err != nil {
		t.hopLimitLock.Unlock()
		return err
	}
	t.hopLimitLock.Unlock()

	select {
	case <-t.ctx.Done():
		return nil
	case h := <-hopCh:
		rtt := time.Since(start)
		if f := t.final.Load(); f != -1 && ttl > int(f) {
			return nil
		}

		if addr, ok := h.Address.(*net.IPAddr); ok && addr.IP.Equal(t.DestIP) {
			for {
				old := t.final.Load()
				if old != -1 && ttl >= int(old) {
					break
				}
				if t.final.CompareAndSwap(old, int32(ttl)) {
					break
				}
			}
		} else if addr, ok := h.Address.(*net.TCPAddr); ok && addr.IP.Equal(t.DestIP) {
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

		t.res.add(h, i, t.NumMeasurements, t.MaxAttempts)
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

		t.res.add(h, i, t.NumMeasurements, t.MaxAttempts)
	}
	return nil
}
