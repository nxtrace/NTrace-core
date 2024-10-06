package trace

import (
	"encoding/binary"
	"math"
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

type TCPTracerv6 struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.Mutex
	SrcIP               net.IP
	icmp                net.PacketConn
	tcp                 net.PacketConn

	final     int
	finalLock sync.Mutex

	sem       *semaphore.Weighted
	fetchLock sync.Mutex
}

func (t *TCPTracerv6) Execute() (*Result, error) {
	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	t.SrcIP, _ = util.LocalIPPortv6(t.DestIP)
	// log.Println(util.LocalIPPortv6(t.DestIP))
	var err error
	t.tcp, err = net.ListenPacket("ip6:tcp", t.SrcIP.String())
	if err != nil {
		return nil, err
	}
	t.icmp, err = icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		return &t.res, err
	}
	defer t.icmp.Close()

	var cancel context.CancelFunc
	t.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	t.inflightRequest = make(map[int]chan Hop)
	t.final = -1

	go t.listenICMP()
	go t.listenTCP()

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))

	for ttl := t.BeginHop; ttl <= t.MaxHops; ttl++ {
		// 如果到达最终跳，则退出
		if t.final != -1 && ttl > t.final {
			break
		}
		for i := 0; i < t.NumMeasurements; i++ {
			t.wg.Add(1)
			go t.send(ttl)
			<-time.After(time.Millisecond * time.Duration(t.Config.PacketInterval))
		}
		if t.RealtimePrinter != nil {
			// 对于实时模式，应该按照TTL进行并发请求
			t.wg.Wait()
			t.RealtimePrinter(&t.res, ttl-1)
		}
		<-time.After(time.Millisecond * time.Duration(t.Config.TTLInterval))
	}

	go func() {
		if t.AsyncPrinter != nil {
			for {
				t.AsyncPrinter(&t.res)
				time.Sleep(200 * time.Millisecond)
			}
		}

	}()

	if t.RealtimePrinter == nil {
		t.wg.Wait()
	}
	t.res.reduce(t.final)

	return &t.res, nil
}

func (t *TCPTracerv6) listenICMP() {
	lc := NewPacketListener(t.icmp, t.ctx)
	go lc.Start()
	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-lc.Messages:
			// log.Println(msg)

			if msg.N == nil {
				continue
			}
			rm, err := icmp.ParseMessage(58, msg.Msg[:*msg.N])
			if err != nil {
				// log.Println(err)
				continue
			}
			switch rm.Type {
			case ipv6.ICMPTypeTimeExceeded:
				t.handleICMPMessage(msg)
			case ipv6.ICMPTypeDestinationUnreachable:
				t.handleICMPMessage(msg)
			default:
				//log.Println("received icmp message of unknown type", rm.Type)
			}
		}
	}

}

// @title    listenTCP
// @description   监听TCP的响应数据包
func (t *TCPTracerv6) listenTCP() {
	lc := NewPacketListener(t.tcp, t.ctx)
	go lc.Start()

	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-lc.Messages:
			// log.Println(msg)
			// return
			if msg.N == nil {
				continue
			}
			if msg.Peer.String() != t.DestIP.String() {
				continue
			}

			// 解包
			packet := gopacket.NewPacket(msg.Msg[:*msg.N], layers.LayerTypeTCP, gopacket.Default)
			// 从包中获取TCP layer信息
			if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
				tcp, _ := tcpLayer.(*layers.TCP)
				// 取得目标主机的Sequence Number

				if ch, ok := t.inflightRequest[int(tcp.Ack-1)]; ok {
					// 最后一跳
					ch <- Hop{
						Success: true,
						Address: msg.Peer,
					}
				}
			}
		}
	}
}

func (t *TCPTracerv6) handleICMPMessage(msg ReceivedMessage) {
	var sequenceNumber = binary.BigEndian.Uint32(msg.Msg[52:56])

	t.inflightRequestLock.Lock()
	defer t.inflightRequestLock.Unlock()
	ch, ok := t.inflightRequest[int(sequenceNumber)]
	if !ok {
		return
	}
	// log.Println("发送数据", sequenceNumber)
	ch <- Hop{
		Success: true,
		Address: msg.Peer,
	}
	// log.Println("发送成功")
}

func (t *TCPTracerv6) send(ttl int) error {
	err := t.sem.Acquire(context.Background(), 1)
	if err != nil {
		return err
	}
	defer t.sem.Release(1)

	defer t.wg.Done()
	if t.final != -1 && ttl > t.final {
		return nil
	}
	// 随机种子
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	_, srcPort := util.LocalIPPortv6(t.DestIP)
	ipHeader := &layers.IPv6{
		SrcIP:      t.SrcIP,
		DstIP:      t.DestIP,
		NextHeader: layers.IPProtocolTCP,
		HopLimit:   uint8(ttl),
	}
	// 使用Uint16兼容32位系统，防止在rand的时候因使用int32而溢出
	sequenceNumber := uint32(r.Intn(math.MaxUint16))

	tcpHeader := &layers.TCP{
		SrcPort: layers.TCPPort(srcPort),
		DstPort: layers.TCPPort(t.DestPort),
		Seq:     sequenceNumber,
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
	rand.Seed(time.Now().UnixNano())

	// 填充随机数
	for i := range payload {
		payload[i] = byte(rand.Intn(256))
	}
	//copy(buf.Bytes(), payload)

	if err := gopacket.SerializeLayers(buf, opts, tcpHeader, gopacket.Payload(payload)); err != nil {
		return err
	}

	err = ipv6.NewPacketConn(t.tcp).SetHopLimit(ttl)
	if err != nil {
		return err
	}

	start := time.Now()
	if _, err := t.tcp.WriteTo(buf.Bytes(), &net.IPAddr{IP: t.DestIP}); err != nil {
		return err
	}
	// log.Println(ttl, sequenceNumber)
	t.inflightRequestLock.Lock()
	hopCh := make(chan Hop, 1)
	t.inflightRequest[int(sequenceNumber)] = hopCh
	t.inflightRequestLock.Unlock()

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
		} else if addr, ok := h.Address.(*net.TCPAddr); ok && addr.IP.Equal(t.DestIP) {
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
