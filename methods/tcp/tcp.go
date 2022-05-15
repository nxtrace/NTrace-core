package tcp

import (
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/xgadget-lab/nexttrace/listener_channel"
	"github.com/xgadget-lab/nexttrace/methods"
	"github.com/xgadget-lab/nexttrace/parallel_limiter"
	"github.com/xgadget-lab/nexttrace/signal"
	"github.com/xgadget-lab/nexttrace/util"
	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type inflightData struct {
	start time.Time
	ttl   uint16
}

type results struct {
	inflightRequests sync.Map

	results   map[uint16][]methods.TracerouteHop
	resultsMu sync.Mutex
	err       error

	concurrentRequests *parallel_limiter.ParallelLimiter
	reachedFinalHop    *signal.Signal
}

type Traceroute struct {
	opConfig    opConfig
	trcrtConfig methods.TracerouteConfig
	results     results
}

type opConfig struct {
	icmpConn net.PacketConn
	tcpConn  net.PacketConn
	tcpMu    sync.Mutex

	destIP net.IP
	srcIP  net.IP

	wg *sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc
}

func New(destIP net.IP, config methods.TracerouteConfig) *Traceroute {
	return &Traceroute{
		opConfig: opConfig{
			destIP: destIP,
		},
		trcrtConfig: config,
	}
}

func (tr *Traceroute) Start() (*map[uint16][]methods.TracerouteHop, error) {
	tr.opConfig.ctx, tr.opConfig.cancel = context.WithCancel(context.Background())

	tr.opConfig.srcIP, _ = util.LocalIPPort(tr.opConfig.destIP)

	var err error
	tr.opConfig.tcpConn, err = net.ListenPacket("ip4:tcp", tr.opConfig.srcIP.String())
	if err != nil {
		return nil, err
	}

	tr.opConfig.icmpConn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	tr.opConfig.wg = &wg

	tr.results = results{
		inflightRequests:   sync.Map{},
		concurrentRequests: parallel_limiter.New(int(tr.trcrtConfig.ParallelRequests)),
		reachedFinalHop:    signal.New(),

		results: map[uint16][]methods.TracerouteHop{},
	}

	return tr.start()
}

func (tr *Traceroute) timeoutLoop() {
	ticker := time.NewTicker(tr.trcrtConfig.Timeout / 4)
	go func() {
		for range ticker.C {
			tr.results.inflightRequests.Range(func(key, value interface{}) bool {
				request := value.(inflightData)
				expired := time.Since(request.start) > tr.trcrtConfig.Timeout
				if !expired {
					return true
				}
				tr.results.inflightRequests.Delete(key)
				tr.addToResult(request.ttl, methods.TracerouteHop{
					Success: false,
					TTL:     request.ttl,
				})
				tr.results.concurrentRequests.Finished()
				tr.opConfig.wg.Done()
				return true
			})
		}
	}()
	select {
	case <-tr.opConfig.ctx.Done():
		ticker.Stop()
	}
}

func (tr *Traceroute) addToResult(ttl uint16, hop methods.TracerouteHop) {
	tr.results.resultsMu.Lock()
	defer tr.results.resultsMu.Unlock()
	if tr.results.results[ttl] == nil {
		tr.results.results[ttl] = []methods.TracerouteHop{}
	}

	tr.results.results[ttl] = append(tr.results.results[ttl], hop)
}

func (tr *Traceroute) handleICMPMessage(msg listener_channel.ReceivedMessage, data []byte) {
	header, err := util.GetICMPResponsePayload(data)
	if err != nil {
		return
	}
	sequenceNumber := util.GetTCPSeq(header)
	val, ok := tr.results.inflightRequests.LoadAndDelete(sequenceNumber)
	if !ok {
		return
	}
	request := val.(inflightData)
	elapsed := time.Since(request.start)
	if msg.Peer.String() == tr.opConfig.destIP.String() {
		tr.results.reachedFinalHop.Signal()
	}
	tr.addToResult(request.ttl, methods.TracerouteHop{
		Success: true,
		Address: msg.Peer,
		TTL:     request.ttl,
		RTT:     &elapsed,
	})
	tr.results.concurrentRequests.Finished()
	tr.opConfig.wg.Done()
}

func (tr *Traceroute) icmpListener() {
	lc := listener_channel.New(tr.opConfig.icmpConn)

	defer lc.Stop()

	go lc.Start()

	for {
		select {
		case <-tr.opConfig.ctx.Done():
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
				body := rm.Body.(*icmp.TimeExceeded).Data
				tr.handleICMPMessage(msg, body)
			case ipv4.ICMPTypeDestinationUnreachable:
				body := rm.Body.(*icmp.DstUnreach).Data
				tr.handleICMPMessage(msg, body)
			default:
				log.Println("received icmp message of unknown type")
			}
		}
	}
}

func (tr *Traceroute) tcpListener() {
	lc := listener_channel.New(tr.opConfig.tcpConn)

	defer lc.Stop()

	go lc.Start()

	for {
		select {
		case <-tr.opConfig.ctx.Done():
			return
		case msg := <-lc.Messages:
			if msg.N == nil {
				continue
			}
			if msg.Peer.String() != tr.opConfig.destIP.String() {
				continue
			}
			// Decode a packet
			packet := gopacket.NewPacket(msg.Msg[:*msg.N], layers.LayerTypeTCP, gopacket.Default)
			// Get the TCP layer from this packet
			if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
				tcp, _ := tcpLayer.(*layers.TCP)

				val, ok := tr.results.inflightRequests.LoadAndDelete(tcp.Ack - 1)
				if !ok {
					continue
				}
				request := val.(inflightData)
				tr.results.concurrentRequests.Finished()
				elapsed := time.Since(request.start)
				if msg.Peer.String() == tr.opConfig.destIP.String() {
					tr.results.reachedFinalHop.Signal()
				}
				tr.addToResult(request.ttl, methods.TracerouteHop{
					Success: true,
					Address: msg.Peer,
					TTL:     request.ttl,
					RTT:     &elapsed,
				})
				tr.opConfig.wg.Done()
			}
		}
	}
}

func (tr *Traceroute) sendMessage(ttl uint16) {
	_, srcPort := util.LocalIPPort(tr.opConfig.destIP)
	ipHeader := &layers.IPv4{
		SrcIP:    tr.opConfig.srcIP,
		DstIP:    tr.opConfig.destIP,
		Protocol: layers.IPProtocolTCP,
		TTL:      uint8(ttl),
	}

	sequenceNumber := uint32(rand.Intn(math.MaxUint32))

	tcpHeader := &layers.TCP{
		SrcPort: layers.TCPPort(srcPort),
		DstPort: layers.TCPPort(tr.trcrtConfig.Port),
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
		tr.results.err = err
		tr.opConfig.cancel()
		return
	}

	tr.opConfig.tcpMu.Lock()
	defer tr.opConfig.tcpMu.Unlock()
	err := ipv4.NewPacketConn(tr.opConfig.tcpConn).SetTTL(int(ttl))
	if err != nil {
		tr.results.err = err
		tr.opConfig.cancel()
		return
	}

	start := time.Now()
	if _, err := tr.opConfig.tcpConn.WriteTo(buf.Bytes(), &net.IPAddr{IP: tr.opConfig.destIP}); err != nil {
		tr.results.err = err
		tr.opConfig.cancel()
		return
	}
	tr.results.inflightRequests.Store(sequenceNumber, inflightData{start: start, ttl: ttl})
}

func (tr *Traceroute) sendLoop() {
	rand.Seed(time.Now().UTC().UnixNano())
	defer tr.opConfig.wg.Done()

	for ttl := uint16(1); ttl <= tr.trcrtConfig.MaxHops; ttl++ {
		select {
		case <-tr.results.reachedFinalHop.Chan():
			return
		default:
		}
		for i := 0; i < int(tr.trcrtConfig.NumMeasurements); i++ {
			select {
			case <-tr.opConfig.ctx.Done():
				return
			case <-tr.results.concurrentRequests.Start():
				tr.opConfig.wg.Add(1)
				go tr.sendMessage(ttl)
			}
		}
	}
}

func (tr *Traceroute) start() (*map[uint16][]methods.TracerouteHop, error) {
	go tr.timeoutLoop()
	go tr.icmpListener()
	go tr.tcpListener()

	tr.opConfig.wg.Add(1)
	go tr.sendLoop()

	tr.opConfig.wg.Wait()
	tr.opConfig.cancel()

	if tr.results.err != nil {
		return nil, tr.results.err
	}

	result := methods.ReduceFinalResult(tr.results.results, tr.trcrtConfig.MaxHops, tr.opConfig.destIP)

	return &result, tr.results.err
}
