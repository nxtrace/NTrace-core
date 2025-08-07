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
	"golang.org/x/sync/semaphore"

	"github.com/nxtrace/NTrace-core/trace/internal"
)

type ICMPTracer struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.RWMutex
	icmp                net.PacketConn
	icmpConn            *ipv4.PacketConn
	hopLimitLock        sync.Mutex

	final     int
	finalLock sync.Mutex

	sem       *semaphore.Weighted
	fetchLock sync.Mutex
}

var psize = 52

func (t *ICMPTracer) PrintFunc() {
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

func (t *ICMPTracer) Execute() (*Result, error) {
	// 初始化 inflightRequest map
	t.inflightRequestLock.Lock()
	t.inflightRequest = make(map[int]chan Hop)
	t.inflightRequestLock.Unlock()

	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	var err error

	t.icmp, err = internal.ListenICMP("ip4:icmp", t.SrcAddr)
	if err != nil {
		return &t.res, err
	}
	defer t.icmp.Close()

	t.icmpConn = ipv4.NewPacketConn(t.icmp)

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
			// 将 TTL 编码到高 8 位；将索引 i 编码到低 8 位
			seq := (ttl << 8) | (i & 0xFF)

			t.wg.Add(1)
			go t.send(ttl, seq)
			<-time.After(time.Millisecond * time.Duration(t.Config.PacketInterval))
		}
		<-time.After(time.Millisecond * time.Duration(t.Config.TTLInterval))
	}
	t.wg.Wait()
	t.res.reduce(t.final)

	return &t.res, nil
}

func (t *ICMPTracer) listenICMP() {
	lc := NewPacketListener(t.icmp, t.ctx)
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
				echoReply, ok := rm.Body.(*icmp.Echo)
				if ok {
					// 从 Echo.Seq 恢复出先前编码的 (ttl<<8)|index
					seq := echoReply.Seq
					// 高 8 位是真正的 TTL
					ttl := seq >> 8
					// TTL 越界时舍弃
					if ttl < t.BeginHop || ttl > t.MaxHops {
						continue
					}
					// 只在 Peer 是目的地址时分发，用 seq 作为通道 key
					if msg.Peer.String() == t.DestIP.String() {
						t.handleICMPMessage(msg, 1, echoReply.Data, seq)
					}
				}
				continue
			}
			// 使用 inner ICMP header 的 Seq 作为唯一标识
			seq := int(binary.BigEndian.Uint16(msg.Msg[34:36]))
			ttl := seq >> 8
			if ttl < t.BeginHop || ttl > t.MaxHops {
				continue
			}
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
							t.handleICMPMessage(msg, 0, rm.Body.(*icmp.TimeExceeded).Data, seq)
						case ipv4.ICMPTypeEchoReply:
							t.handleICMPMessage(msg, 1, rm.Body.(*icmp.Echo).Data, seq)
						//unreachable
						case ipv4.ICMPTypeDestinationUnreachable:
							t.handleICMPMessage(msg, 2, rm.Body.(*icmp.DstUnreach).Data, seq)
						default:
							// log.Println("received icmp message of unknown type", rm.Type)
						}
					}
				}
			}
		}
	}

}

func (t *ICMPTracer) handleICMPMessage(msg ReceivedMessage, icmpType int8, data []byte, seq int) {
	if icmpType == 2 {
		if t.DestIP.String() != msg.Peer.String() {
			return
		}
	}

	t.inflightRequestLock.RLock()
	defer t.inflightRequestLock.RUnlock()

	mpls := extractMPLS(msg, data)
	if _, ok := t.inflightRequest[seq]; ok {
		t.inflightRequest[seq] <- Hop{
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

func (t *ICMPTracer) send(ttl, seq int) error {
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
			Seq:  seq,
		},
	}
	// 串行设置 TTL + 发送，放在同一把锁里保证并发安全
	t.hopLimitLock.Lock()
	if err := t.icmpConn.SetTTL(ttl); err != nil {
		t.hopLimitLock.Unlock()
		return err
	}
	wb, err := icmpHeader.Marshal(nil)
	if err != nil {
		t.hopLimitLock.Unlock()
		log.Fatal(err)
	}
	start := time.Now()
	if _, err := t.icmp.WriteTo(wb, &net.IPAddr{IP: t.DestIP}); err != nil {
		t.hopLimitLock.Unlock()
		log.Fatal(err)
	}
	if err := t.icmp.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.hopLimitLock.Unlock()
		log.Fatal(err)
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
