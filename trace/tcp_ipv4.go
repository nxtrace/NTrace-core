package trace

import (
	"log"
	"math"
	"math/rand"
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

type TCPTracer struct {
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

	sem *semaphore.Weighted
}

func (t *TCPTracer) GetConfig() *Config {
	return &t.Config
}

func (t *TCPTracer) SetConfig(c Config) {
	t.Config = c
}

func (t *TCPTracer) Execute() (*Result, error) {
	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	t.SrcIP, _ = util.LocalIPPort(t.DestIP)

	var err error
	if t.SrcAddr != "" {
		t.tcp, err = net.ListenPacket("ip4:tcp", t.SrcAddr)
	} else {
		t.tcp, err = net.ListenPacket("ip4:tcp", t.SrcIP.String())
	}

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
	t.inflightRequestLock.Lock()
	t.inflightRequest = make(map[int]chan Hop)
	t.inflightRequestLock.Unlock()

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
			<-time.After(t.Config.PacketInterval)
		}
		<-time.After(t.Config.TTLInterval)
	}

	t.res.reduce(t.final)

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
			dstip := net.IP(msg.Msg[24:28])
			if dstip.Equal(t.DestIP) {
				rm, err := icmp.ParseMessage(1, msg.Msg[:*msg.N])
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
					//log.Println("received icmp message of unknown type", rm.Type)
				}
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
			if msg.Peer.String() != t.DestIP.String() {
				continue
			}

			// 解包
			packet := gopacket.NewPacket(msg.Msg[:*msg.N], layers.LayerTypeTCP, gopacket.Default)
			// 从包中获取TCP layer信息
			if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
				tcp, _ := tcpLayer.(*layers.TCP)
				// 取得目标主机的Sequence Number
				t.inflightRequestLock.Lock()
				if ch, ok := t.inflightRequest[int(tcp.Ack-1)]; ok {
					// 最后一跳
					ch <- Hop{
						Address: msg.Peer,
					}
				}
				t.inflightRequestLock.Unlock()
			}
		}
	}
}

func (t *TCPTracer) handleICMPMessage(msg ReceivedMessage, data []byte) {
	header, err := util.GetICMPResponsePayload(data)
	if err != nil {
		return
	}
	sequenceNumber := util.GetTCPSeq(header)
	t.inflightRequestLock.Lock()
	defer t.inflightRequestLock.Unlock()
	ch, ok := t.inflightRequest[int(sequenceNumber)]
	if !ok {
		return
	}
	ch <- Hop{
		Address: msg.Peer,
	}

}

func (t *TCPTracer) send(ttl int) error {
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
	_, srcPort := util.LocalIPPort(t.DestIP)
	ipHeader := &layers.IPv4{
		SrcIP:    t.SrcIP,
		DstIP:    t.DestIP,
		Protocol: layers.IPProtocolTCP,
		TTL:      uint8(ttl),
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
	if err := gopacket.SerializeLayers(buf, opts, tcpHeader); err != nil {
		return err
	}

	err = ipv4.NewPacketConn(t.tcp).SetTTL(ttl)
	if err != nil {
		return err
	}

	start := time.Now()
	if _, err := t.tcp.WriteTo(buf.Bytes(), &net.IPAddr{IP: t.DestIP}); err != nil {
		return err
	}
	t.inflightRequestLock.Lock()
	hopCh := make(chan Hop)
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
