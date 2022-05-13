package udp

import (
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/xgadget-lab/nexttrace/listener_channel"
	"github.com/xgadget-lab/nexttrace/methods"
	"github.com/xgadget-lab/nexttrace/methods/quic"
	"github.com/xgadget-lab/nexttrace/parallel_limiter"
	"github.com/xgadget-lab/nexttrace/signal"
	"github.com/xgadget-lab/nexttrace/taskgroup"
	"github.com/xgadget-lab/nexttrace/util"
	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type inflightData struct {
	icmpMsg chan<- net.Addr
}

type opConfig struct {
	quic   bool
	destIP net.IP
	wg     *taskgroup.TaskGroup

	icmpConn net.PacketConn

	ctx    context.Context
	cancel context.CancelFunc
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
	trcrtConfig methods.TracerouteConfig
	opConfig    opConfig
	results     results
}

func New(destIP net.IP, quic bool, config methods.TracerouteConfig) *Traceroute {
	return &Traceroute{
		opConfig: opConfig{
			quic:   quic,
			destIP: destIP,
		},
		trcrtConfig: config,
	}
}

func (tr *Traceroute) Start() (*map[uint16][]methods.TracerouteHop, error) {
	tr.opConfig.ctx, tr.opConfig.cancel = context.WithCancel(context.Background())

	tr.results = results{
		inflightRequests:   sync.Map{},
		concurrentRequests: parallel_limiter.New(int(tr.trcrtConfig.ParallelRequests)),
		results:            map[uint16][]methods.TracerouteHop{},
		reachedFinalHop:    signal.New(),
	}

	var err error
	tr.opConfig.icmpConn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}

	return tr.start()
}

func (tr *Traceroute) addToResult(ttl uint16, hop methods.TracerouteHop) {
	tr.results.resultsMu.Lock()
	defer tr.results.resultsMu.Unlock()
	if tr.results.results[ttl] == nil {
		tr.results.results[ttl] = []methods.TracerouteHop{}
	}

	tr.results.results[ttl] = append(tr.results.results[ttl], hop)
}

func (tr *Traceroute) getUDPConn(try int) (net.IP, int, net.PacketConn) {
	srcIP, _ := util.LocalIPPort(tr.opConfig.destIP)

	var ipString string

	if srcIP == nil {
		ipString = ""
	} else {
		ipString = srcIP.String()
	}

	udpConn, err := net.ListenPacket("udp", ipString+":0")
	if err != nil {
		if try > 3 {
			log.Fatal(err)
		}
		return tr.getUDPConn(try + 1)
	}

	return srcIP, udpConn.LocalAddr().(*net.UDPAddr).Port, udpConn
}

func (tr *Traceroute) sendMessage(ttl uint16) {
	srcIP, srcPort, udpConn := tr.getUDPConn(0)

	var payload []byte
	if tr.opConfig.quic {
		payload = quic.GenerateWithRandomIds()
	} else {
		ipHeader := &layers.IPv4{
			SrcIP:    srcIP,
			DstIP:    tr.opConfig.destIP,
			Protocol: layers.IPProtocolTCP,
			TTL:      uint8(ttl),
		}

		udpHeader := &layers.UDP{
			SrcPort: layers.UDPPort(srcPort),
			DstPort: layers.UDPPort(tr.trcrtConfig.Port),
		}
		_ = udpHeader.SetNetworkLayerForChecksum(ipHeader)
		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}
		if err := gopacket.SerializeLayers(buf, opts, udpHeader, gopacket.Payload("HAJSFJHKAJSHFKJHAJKFHKASHKFHHKAFKHFAHSJK")); err != nil {
			tr.results.err = err
			tr.opConfig.cancel()
			return
		}

		payload = buf.Bytes()
	}

	err := ipv4.NewPacketConn(udpConn).SetTTL(int(ttl))
	if err != nil {
		tr.results.err = err
		tr.opConfig.cancel()
		return
	}

	icmpMsg := make(chan net.Addr, 1)
	udpMsg := make(chan net.Addr, 1)

	start := time.Now()
	if _, err := udpConn.WriteTo(payload, &net.UDPAddr{IP: tr.opConfig.destIP, Port: tr.trcrtConfig.Port}); err != nil {
		tr.results.err = err
		tr.opConfig.cancel()
		return
	}

	inflight := inflightData{
		icmpMsg: icmpMsg,
	}

	tr.results.inflightRequests.Store(uint16(srcPort), inflight)

	go func() {
		reply := make([]byte, 1500)
		_, peer, err := udpConn.ReadFrom(reply)
		if err != nil {
			// probably because we closed the connection
			return
		}
		udpMsg <- peer
	}()

	select {
	case peer := <-icmpMsg:
		rtt := time.Since(start)
		if peer.(*net.IPAddr).IP.Equal(tr.opConfig.destIP) {
			tr.results.reachedFinalHop.Signal()
		}
		tr.addToResult(ttl, methods.TracerouteHop{
			Success: true,
			Address: peer,
			TTL:     ttl,
			RTT:     &rtt,
		})
	case peer := <-udpMsg:
		rtt := time.Since(start)
		ip := peer.(*net.UDPAddr).IP
		if ip.Equal(tr.opConfig.destIP) {
			tr.results.reachedFinalHop.Signal()
		}
		tr.addToResult(ttl, methods.TracerouteHop{
			Success: true,
			Address: &net.IPAddr{IP: ip},
			TTL:     ttl,
			RTT:     &rtt,
		})
	case <-time.After(tr.trcrtConfig.Timeout):
		tr.addToResult(ttl, methods.TracerouteHop{
			Success: false,
			Address: nil,
			TTL:     ttl,
			RTT:     nil,
		})
	}

	tr.results.inflightRequests.Delete(uint16(srcPort))
	udpConn.Close()
	tr.results.concurrentRequests.Finished()
	tr.opConfig.wg.Done()
}

func (tr *Traceroute) handleICMPMessage(msg listener_channel.ReceivedMessage, data []byte) {
	header, err := methods.GetICMPResponsePayload(data)
	if err != nil {
		return
	}
	srcPort := methods.GetUDPSrcPort(header)
	val, ok := tr.results.inflightRequests.LoadAndDelete(srcPort)
	if !ok {
		return
	}
	request := val.(inflightData)
	request.icmpMsg <- msg.Peer
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
				log.Println("received icmp message of unknown type", rm.Type)
			}
		}
	}
}

func (tr *Traceroute) sendLoop() {
	rand.Seed(time.Now().UTC().UnixNano())

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
				tr.opConfig.wg.Add()
				go tr.sendMessage(ttl)
			}
		}
	}
}

func (tr *Traceroute) start() (*map[uint16][]methods.TracerouteHop, error) {
	go tr.icmpListener()

	wg := taskgroup.New()
	tr.opConfig.wg = wg

	tr.sendLoop()

	wg.Wait()

	tr.opConfig.cancel()
	tr.opConfig.icmpConn.Close()

	if tr.results.err != nil {
		return nil, tr.results.err
	}

	result := methods.ReduceFinalResult(tr.results.results, tr.trcrtConfig.MaxHops, tr.opConfig.destIP)

	return &result, tr.results.err
}
