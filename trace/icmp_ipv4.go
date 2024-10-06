package trace

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/nxtrace/NTrace-core/trace/internal"
)

type ICMPTracer struct {
	Config
	wg                    sync.WaitGroup
	res                   Result
	ctx                   context.Context
	inflightRequest       map[int]chan Hop
	inflightRequestRWLock sync.RWMutex
	icmpListen            net.PacketConn
	final                 int
	finalLock             sync.Mutex
	fetchLock             sync.Mutex
}

var psize = 52

func (t *ICMPTracer) PrintFunc() {
	defer t.wg.Done()
	var ttl = t.Config.BeginHop - 1
	for {
		if t.AsyncPrinter != nil {
			t.AsyncPrinter(&t.res)
		}
		// 接收的时候检查一下是不是 3 跳都齐了
		if len(t.res.Hops)-1 > ttl {
			if len(t.res.Hops[ttl]) == t.NumMeasurements {
				if t.RealtimePrinter != nil {
					t.RealtimePrinter(&t.res, ttl)
				}
				ttl++

				if ttl == t.final-1 || ttl >= t.MaxHops-1 {
					return
				}
			}
		}
		<-time.After(200 * time.Millisecond)
	}
}

func (t *ICMPTracer) Execute() (*Result, error) {
	t.inflightRequestRWLock.Lock()
	t.inflightRequest = make(map[int]chan Hop)
	t.inflightRequestRWLock.Unlock()

	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	var err error

	t.icmpListen, err = internal.ListenICMP("ip4:icmp", t.SrcAddr)
	if err != nil {
		return &t.res, err
	}
	defer t.icmpListen.Close()

	var cancel context.CancelFunc
	t.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	t.final = -1

	go t.listenICMP()
	t.wg.Add(1)
	go t.PrintFunc()
	for ttl := t.BeginHop; ttl <= t.MaxHops; ttl++ {
		t.inflightRequestRWLock.Lock()
		t.inflightRequest[ttl] = make(chan Hop, t.NumMeasurements)
		t.inflightRequestRWLock.Unlock()
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

	t.wg.Wait()
	t.res.reduce(t.final)
	if t.final != -1 {
		if t.RealtimePrinter != nil {
			t.RealtimePrinter(&t.res, t.final-1)
		}
	} else {
		for i := 0; i < t.NumMeasurements; i++ {
			t.res.add(Hop{
				Success: false,
				Address: nil,
				TTL:     30,
				RTT:     0,
				Error:   ErrHopLimitTimeout,
			})
		}
		if t.RealtimePrinter != nil {
			t.RealtimePrinter(&t.res, t.MaxHops-1)
		}
	}
	return &t.res, nil
}

func (t *ICMPTracer) listenICMP() {
	lc := NewPacketListener(t.icmpListen, t.ctx)
	psize = t.Config.PktSize
	go lc.Start()
	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-lc.Messages:
			if msg.N == nil {
				continue
			}
			// log.Println(msg.Msg)
			if msg.Msg[0] == 0 {
				rm, err := icmp.ParseMessage(1, msg.Msg[:*msg.N])
				if err != nil {
					log.Println(err)
					continue
				}
				echoReply := rm.Body.(*icmp.Echo)
				ttl := echoReply.Seq // This is the TTL value
				if ttl > 100 {
					continue
				}
				if msg.Peer.String() == t.DestIP.String() {
					t.handleICMPMessage(msg, 1, rm.Body.(*icmp.Echo).Data, ttl)
				}
				continue
			}
			ttl := int64(binary.BigEndian.Uint16(msg.Msg[34:36]))
			packetId := strconv.FormatInt(int64(binary.BigEndian.Uint16(msg.Msg[32:34])), 2)
			if processId, _, err := reverseID(packetId); err == nil {
				if processId == int64(os.Getpid()&0x7f) {
					dstip := net.IP(msg.Msg[24:28])
					if dstip.Equal(t.DestIP) || dstip.Equal(net.IPv4zero) {
						// 匹配再继续解析包，否则直接丢弃
						rm, err := icmp.ParseMessage(1, msg.Msg[:*msg.N])
						if err != nil {
							log.Println(err)
							continue
						}

						switch rm.Type {
						case ipv4.ICMPTypeTimeExceeded:
							t.handleICMPMessage(msg, 0, rm.Body.(*icmp.TimeExceeded).Data, int(ttl))
						case ipv4.ICMPTypeEchoReply:
							t.handleICMPMessage(msg, 1, rm.Body.(*icmp.Echo).Data, int(ttl))
						//unreachable
						case ipv4.ICMPTypeDestinationUnreachable:
							t.handleICMPMessage(msg, 2, rm.Body.(*icmp.DstUnreach).Data, int(ttl))
						default:
							// log.Println("received icmp message of unknown type", rm.Type)
						}
					}
				}
			}
		}
	}

}

func (t *ICMPTracer) handleICMPMessage(msg ReceivedMessage, icmpType int8, data []byte, ttl int) {
	if icmpType == 2 {
		if t.DestIP.String() != msg.Peer.String() {
			return
		}
	}

	t.inflightRequestRWLock.RLock()
	defer t.inflightRequestRWLock.RUnlock()

	mpls := extractMPLS(msg, data)
	if _, ok := t.inflightRequest[ttl]; ok {
		t.inflightRequest[ttl] <- Hop{
			Success: true,
			Address: msg.Peer,
			MPLS:    mpls,
		}
	}
}

func gernerateID(ttlInt int) int {
	const IdFixedHeader = "10"
	var processID = fmt.Sprintf("%07b", os.Getpid()&0x7f) //取进程ID的前7位
	var ttl = fmt.Sprintf("%06b", ttlInt)                 //取TTL的后6位

	var parity int
	id := IdFixedHeader + processID + ttl
	for _, c := range id {
		if c == '1' {
			parity++
		}
	}
	if parity%2 == 0 {
		id += "1"
	} else {
		id += "0"
	}

	res, _ := strconv.ParseInt(id, 2, 32)
	return int(res)
}

func reverseID(id string) (int64, int64, error) {
	if len(id) < 16 {
		return 0, 0, errors.New("err")
	}
	ttl, err := strconv.ParseInt(id[9:15], 2, 32)
	if err != nil {
		return 0, 0, err
	}
	//process ID
	processID, _ := strconv.ParseInt(id[2:9], 2, 32)

	parity := 0
	for i := 0; i < len(id)-1; i++ {
		if id[i] == '1' {
			parity++
		}
	}

	if parity%2 == 1 {
		if id[len(id)-1] == '0' {
			// fmt.Println("Parity check passed.")
		} else {
			// fmt.Println("Parity check failed.")
			return 0, 0, errors.New("err")
		}
	} else {
		if id[len(id)-1] == '1' {
			// fmt.Println("Parity check passed.")
		} else {
			// fmt.Println("Parity check failed.")
			return 0, 0, errors.New("err")
		}
	}
	return processID, ttl, nil
}

func (t *ICMPTracer) send(ttl int) error {

	defer t.wg.Done()
	if t.final != -1 && ttl > t.final {
		return nil
	}

	//id := gernerateID(ttl)
	id := gernerateID(0)
	// log.Println("发送的", id)

	//data := []byte{byte(ttl)}
	data := []byte{byte(0)}
	data = append(data, bytes.Repeat([]byte{1}, t.Config.PktSize-5)...)
	data = append(data, 0x00, 0x00, 0x4f, 0xff)

	icmpHeader := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: id,
			//Data: []byte("HELLO-R-U-THERE"),
			Data: data,
			Seq:  ttl,
		},
	}

	err := ipv4.NewPacketConn(t.icmpListen).SetTTL(ttl)
	if err != nil {
		return err
	}

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
	case h := <-t.inflightRequest[ttl]:
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
		err := h.fetchIPData(t.Config)
		if err != nil {
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
