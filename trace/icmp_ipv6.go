package trace

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
	"golang.org/x/sync/semaphore"

	"github.com/nxtrace/NTrace-core/trace/internal"
)

type ICMPTracerv6 struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.RWMutex
	icmp                net.PacketConn
	icmpConn            *ipv6.PacketConn
	hopLimitLock        sync.Mutex

	final     int
	finalLock sync.Mutex

	sem       *semaphore.Weighted
	fetchLock sync.Mutex
}

func (t *ICMPTracerv6) PrintFunc() {
	defer t.wg.Done()
	var ttl = t.Config.BeginHop - 1
	for {
		if t.AsyncPrinter != nil {
			t.AsyncPrinter(&t.res)
		}

		// 接收的时候检查一下是不是 3 跳都齐了
		if len(t.res.Hops) > ttl {
			if len(t.res.Hops[ttl]) == t.NumMeasurements {
				if t.RealtimePrinter != nil {
					t.RealtimePrinter(&t.res, ttl)
				}
				ttl++

				if ttl == t.final || ttl >= t.MaxHops {
					return
				}
			}
		}
		<-time.After(200 * time.Millisecond)
	}
}

func (t *ICMPTracerv6) Execute() (*Result, error) {
	// 初始化 inflightRequest map
	t.inflightRequestLock.Lock()
	t.inflightRequest = make(map[int]chan Hop)
	t.inflightRequestLock.Unlock()

	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	var err error

	t.icmp, err = internal.ListenICMP("ip6:ipv6-icmp", t.SrcAddr)
	if err != nil {
		return &t.res, err
	}
	defer t.icmp.Close()

	t.icmpConn = ipv6.NewPacketConn(t.icmp)

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
	// for ttl := t.BeginHop; ttl <= t.MaxHops; ttl++ {
	// 	if t.final != -1 && ttl > t.final {
	// 		break
	// 	}
	// 	for i := 0; i < t.NumMeasurements; i++ {
	// 		t.wg.Add(1)
	// 		go t.send(ttl)
	// 	}
	// 	// 一组TTL全部退出（收到应答或者超时终止）以后，再进行下一个TTL的包发送
	// 	t.wg.Wait()
	// 	if t.RealtimePrinter != nil {
	// 		t.RealtimePrinter(&t.res, ttl-1)
	// 	}

	// 	if t.AsyncPrinter != nil {
	// 		t.AsyncPrinter(&t.res)
	// 	}
	// }
	t.wg.Wait()
	t.res.reduce(t.final)

	return &t.res, nil
}

func (t *ICMPTracerv6) listenICMP() {
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
			if msg.Msg[0] == 129 {
				rm, err := icmp.ParseMessage(58, msg.Msg[:*msg.N])
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
					// 只在目的地址匹配时分发，使用 seq 作为 key
					if msg.Peer.String() == t.DestIP.String() {
						t.handleICMPMessage(msg, 1, echoReply.Data, seq)
					}
				}
				continue
			}
			// 使用 inner ICMP header 的 Seq 作为唯一标识
			seq := int(binary.BigEndian.Uint16(msg.Msg[54:56]))
			ttl := seq >> 8
			if ttl < t.BeginHop || ttl > t.MaxHops {
				continue
			}
			packetId := strconv.FormatInt(int64(binary.BigEndian.Uint16(msg.Msg[52:54])), 2)
			if processId, _, err := reverseID(packetId); err == nil {
				if processId == int64(os.Getpid()&0x7f) {
					dstip := net.IP(msg.Msg[32:48])
					// 无效包本地环回包
					if dstip.String() == "::" {
						continue
					}
					if dstip.Equal(t.DestIP) || dstip.Equal(net.IPv6zero) {
						// 匹配再继续解析包，否则直接丢弃
						rm, err := icmp.ParseMessage(58, msg.Msg[:*msg.N])
						if err != nil {
							log.Println(err)
							continue
						}
						switch rm.Type {
						case ipv6.ICMPTypeTimeExceeded:
							t.handleICMPMessage(msg, 0, rm.Body.(*icmp.TimeExceeded).Data, seq)
						case ipv6.ICMPTypeEchoReply:
							t.handleICMPMessage(msg, 1, rm.Body.(*icmp.Echo).Data, seq)
						case ipv6.ICMPTypeDestinationUnreachable:
							t.handleICMPMessage(msg, 2, rm.Body.(*icmp.DstUnreach).Data, seq)
						default:
							// log.Println("received icmp message of unknown type", rm.Type)
						}
					}
				}
			}
			// dstip := net.IP(msg.Msg[32:48])
			// if binary.BigEndian.Uint16(msg.Msg[52:54]) != uint16(os.Getpid()&0xffff) {
			// 	//	// 如果类型为应答消息，且应答消息包的进程ID与主进程相同时不跳过
			// 	if binary.BigEndian.Uint16(msg.Msg[52:54]) != 0 {
			// 		continue
			// 	} else {
			// 		if dstip.String() != "::" {
			// 			continue
			// 		}
			// 		if msg.Peer.String() != t.DestIP.String() {
			// 			continue
			// 		}
			// 	}
			// }

			// if dstip.Equal(t.DestIP) || dstip.Equal(net.IPv6zero) {
			// 	rm, err := icmp.ParseMessage(58, msg.Msg[:*msg.N])
			// 	if err != nil {
			// 		log.Println(err)
			// 		continue
			// 	}
			// 	// log.Println(msg.Peer)
			// 	switch rm.Type {
			// 	case ipv6.ICMPTypeTimeExceeded:
			// 		t.handleICMPMessage(msg, 0, rm.Body.(*icmp.TimeExceeded).Data)
			// 	case ipv6.ICMPTypeEchoReply:
			// 		t.handleICMPMessage(msg, 1, rm.Body.(*icmp.Echo).Data)
			// 	default:
			// 		// log.Println("received icmp message of unknown type", rm.Type)
			// 	}
			// }
		}
	}

}

func (t *ICMPTracerv6) handleICMPMessage(msg ReceivedMessage, icmpType int8, data []byte, seq int) {
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

func (t *ICMPTracerv6) send(ttl, seq int) error {
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
	//data := []byte{byte(ttl)}
	data := []byte{byte(0)}
	data = append(data, bytes.Repeat([]byte{1}, t.Config.PktSize-5)...)
	data = append(data, 0x00, 0x00, 0x4f, 0xff)

	icmpHeader := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest, Code: 0,
		Body: &icmp.Echo{
			ID: id,
			//Data: []byte("HELLO-R-U-THERE"),
			Data: data,
			Seq:  seq,
		},
	}
	// 串行设置 HopLimit + 发送，放在同一把锁里保证并发安全
	t.hopLimitLock.Lock()
	if err := t.icmpConn.SetHopLimit(ttl); err != nil {
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
