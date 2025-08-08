package trace

import (
	"bytes"
	"encoding/binary"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/trace/internal"
	"github.com/nxtrace/NTrace-core/util"
	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/sync/semaphore"
)

type ICMPTracer struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	echoIDTag           uint8
	pidLow              uint8
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.Mutex
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

func (t *ICMPTracer) initEchoID() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	t.echoIDTag = uint8(r.Intn(256))     // 高 8 位随机 tag
	t.pidLow = uint8(os.Getpid() & 0xFF) // 低 8 位为 pid
}

func (t *ICMPTracer) Execute() (*Result, error) {
	// 初始化 inflightRequest map
	t.inflightRequestLock.Lock()
	t.inflightRequest = make(map[int]chan Hop)
	t.inflightRequestLock.Unlock()
	// 初始化 Echo.ID
	t.initEchoID()

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
			case ipv4.ICMPTypeEchoReply:
				echo := rm.Body.(*icmp.Echo)
				data := echo.Data
				// 只在 Peer 是目的地址时分发，用 seq 作为通道 key
				if ip, ok := msg.Peer.(*net.IPAddr); ok && ip.IP.Equal(t.DestIP) {
					id := uint16(echo.ID)
					if uint8(id>>8) != t.echoIDTag || uint8(id&0xFF) != t.pidLow {
						continue
					}
					// 从 Echo.Seq 恢复出先前编码的 (ttl<<8)|index
					seq := echo.Seq
					// 高 8 位是真正的 TTL
					ttl := seq >> 8
					// TTL 越界时舍弃
					if ttl < t.BeginHop || ttl > t.MaxHops {
						continue
					}
					t.handleICMPMessage(msg, 1, data, seq)
				}
			case ipv4.ICMPTypeTimeExceeded:
				body := rm.Body.(*icmp.TimeExceeded)
				data := body.Data
				if len(data) < 20 || data[0]>>4 != 4 {
					continue
				}
				dstip := net.IP(data[16:20])
				if dstip.Equal(t.DestIP) || dstip.Equal(net.IPv4zero) {
					inner, err := util.GetICMPResponsePayload(data)
					if err != nil || len(inner) < 8 {
						continue
					}
					id := binary.BigEndian.Uint16(inner[4:6])
					if uint8(id>>8) != t.echoIDTag || uint8(id&0xFF) != t.pidLow {
						continue
					}
					seq := int(binary.BigEndian.Uint16(inner[6:8]))
					t.handleICMPMessage(msg, 0, data, seq)
				}
			case ipv4.ICMPTypeDestinationUnreachable:
				body := rm.Body.(*icmp.DstUnreach)
				data := body.Data
				if len(data) < 20 || data[0]>>4 != 4 {
					continue
				}
				dstip := net.IP(data[16:20])
				if dstip.Equal(t.DestIP) || dstip.Equal(net.IPv4zero) {
					inner, err := util.GetICMPResponsePayload(data)
					if err != nil || len(inner) < 8 {
						continue
					}
					id := binary.BigEndian.Uint16(inner[4:6])
					if uint8(id>>8) != t.echoIDTag || uint8(id&0xFF) != t.pidLow {
						continue
					}
					seq := int(binary.BigEndian.Uint16(inner[6:8]))
					t.handleICMPMessage(msg, 2, data, seq)
				}
			default:
				//log.Println("received icmp message of unknown type", rm.Type)
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

	t.inflightRequestLock.Lock()
	defer t.inflightRequestLock.Unlock()

	mpls := extractMPLS(msg, data)
	if _, ok := t.inflightRequest[seq]; ok {
		t.inflightRequest[seq] <- Hop{
			Success: true,
			Address: msg.Peer,
			MPLS:    mpls,
		}
	}
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
	// 高8位放随机tag，低8位放pid低8位
	id := int(uint16(t.echoIDTag)<<8 | uint16(t.pidLow))
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
