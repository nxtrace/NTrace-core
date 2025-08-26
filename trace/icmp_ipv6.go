package trace

import (
	"bytes"
	"context"
	"encoding/binary"
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

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
	"golang.org/x/sync/semaphore"

	"github.com/nxtrace/NTrace-core/trace/internal"
	"github.com/nxtrace/NTrace-core/util"
)

type ICMPTracerv6 struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	echoIDTag           uint8
	pidLow              uint8
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.RWMutex
	icmp                net.PacketConn
	icmpConn            *ipv6.PacketConn
	hopLimitLock        sync.Mutex
	final               atomic.Int32
	sem                 *semaphore.Weighted
}

func (t *ICMPTracerv6) PrintFunc(ctx context.Context, cancel context.CancelCauseFunc) {
	defer t.wg.Done()

	ttl := t.Config.BeginHop - 1
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		if t.AsyncPrinter != nil {
			t.AsyncPrinter(&t.res)
		}

		// 接收的时候检查一下是不是 3 跳都齐了
		if t.ttlComp(ttl + 1) {
			if t.RealtimePrinter != nil {
				t.RealtimePrinter(&t.res, ttl)
			}
			ttl++
			if ttl == int(t.final.Load()) || ttl >= t.MaxHops {
				cancel(errNaturalDone) // 标记为“自然完成”
				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (t *ICMPTracerv6) ttlComp(ttl int) bool {
	idx := ttl - 1
	t.res.lock.RLock()
	defer t.res.lock.RUnlock()
	return idx < len(t.res.Hops) && len(t.res.Hops[idx]) >= t.NumMeasurements
}

func (t *ICMPTracerv6) launchTTL(ctx context.Context, ttl int) {
	go func(ttl int) {
		for i := 0; i < t.MaxAttempts; i++ {
			// 若此 TTL 已完成或 ctx 已取消，则不再发起新的尝试
			if t.ttlComp(ttl) || ctx.Err() != nil {
				return
			}

			t.wg.Add(1)
			go func(ttl, i int) {
				if err := t.send(ctx, ttl, i); err != nil && !errors.Is(err, context.Canceled) {
					log.Printf("send failed: ttl=%d i=%d: %v", ttl, i, err)
				}
			}(ttl, i)

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond * time.Duration(t.Config.PacketInterval)):
			}
		}
	}(ttl)
}

func (t *ICMPTracerv6) initEchoID() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	t.echoIDTag = uint8(r.Intn(256))     // 高 8 位随机 tag
	t.pidLow = uint8(os.Getpid() & 0xFF) // 低 8 位为 pid
}

func (t *ICMPTracerv6) Execute() (res *Result, err error) {
	// 初始化 Echo.ID
	t.initEchoID()

	// 初始化 inflightRequest map
	t.inflightRequest = make(map[int]chan Hop)

	if len(t.res.Hops) > 0 {
		return &t.res, errTracerouteExecuted
	}

	// 初始化 Result.Hops，并预分配到 MaxHops
	t.res.Hops = make([][]Hop, t.MaxHops)

	// 解析并校验用户指定的 IPv6 源地址
	SrcAddr := net.ParseIP(t.SrcAddr)
	if t.SrcAddr != "" && !util.IsIPv6(SrcAddr) {
		return nil, errors.New("invalid IPv6 SrcAddr: " + t.SrcAddr)
	}
	SrcIP := ""
	if SrcAddr != nil {
		SrcIP = SrcAddr.String()
	}

	t.icmp, err = internal.ListenPacket("ip6:ipv6-icmp", SrcIP)
	if err != nil {
		return &t.res, err
	}
	defer func() {
		if c := t.icmp; c != nil { // 先拷一份引用，避免 defer 执行时 t.icmp 已被并发改写
			_ = c.Close()
		}
	}()
	t.icmpConn = ipv6.NewPacketConn(t.icmp)

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancelCause(sigCtx)
	t.final.Store(-1)

	t.wg.Add(1)
	go t.listenICMP(ctx)
	t.wg.Add(1)
	go t.PrintFunc(ctx, cancel)

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		// 立即启动 BeginHop 对应的 TTL 组
		t.launchTTL(ctx, t.BeginHop)

		// 之后按 TTLInterval 周期启动后续 TTL 组
		ticker := time.NewTicker(time.Millisecond * time.Duration(t.Config.TTLInterval))
		defer ticker.Stop()

		for ttl := t.BeginHop + 1; ttl <= t.MaxHops; ttl++ {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			// 如果到达最终跳，则退出
			if f := t.final.Load(); f != -1 && ttl > int(f) {
				return
			}

			// 并发启动这个 TTL 的所有测量
			t.launchTTL(ctx, ttl)
		}
	}()

	<-ctx.Done()
	stop()
	t.wg.Wait()

	bound := int(t.final.Load())
	if bound == -1 {
		bound = t.MaxHops
	}
	t.res.reduce(bound)

	if cause := context.Cause(ctx); !errors.Is(cause, errNaturalDone) {
		return &t.res, cause
	}
	return &t.res, nil
}

func (t *ICMPTracerv6) listenICMP(ctx context.Context) {
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

			var (
				data []byte
			)
			switch rm.Type {
			case ipv6.ICMPTypeEchoReply:
				echo, ok := rm.Body.(*icmp.Echo)
				if !ok || echo == nil {
					continue
				}

				// 只在 Peer 是目的地址时分发，用 seq 作为通道 key
				if ip := util.AddrIP(msg.Peer); ip != nil && ip.Equal(t.DestIP) {
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
					t.handleICMPMessage(msg, seq)
				}
				continue
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
				inner, err := util.GetICMPResponsePayload(data)
				if err != nil || len(inner) < 8 {
					continue
				}
				id := binary.BigEndian.Uint16(inner[4:6])
				if uint8(id>>8) != t.echoIDTag || uint8(id&0xFF) != t.pidLow {
					continue
				}
				seq := int(binary.BigEndian.Uint16(inner[6:8]))
				t.handleICMPMessage(msg, seq)
			}
		}
	}
}

func (t *ICMPTracerv6) handleICMPMessage(msg ReceivedMessage, seq int) {
	mpls := extractMPLS(msg)

	// 取出通道后立刻解锁
	t.inflightRequestLock.RLock()
	ch, ok := t.inflightRequest[seq]
	t.inflightRequestLock.RUnlock()
	if !ok || ch == nil {
		return
	}

	h := Hop{
		Success: true,
		Address: msg.Peer,
		MPLS:    mpls,
	}

	// 非阻塞发送，避免重复回包把缓冲塞满导致阻塞
	select {
	case ch <- h:
	default:
		// 丢弃重复/迟到的相同 seq 回包，避免阻塞
	}
}

func (t *ICMPTracerv6) send(ctx context.Context, ttl, i int) error {
	defer t.wg.Done()
	if t.ttlComp(ttl) {
		// 快路径短路：若该 TTL 已完成，直接返回避免竞争信号量与无谓发包
		return nil
	}

	if err := t.sem.Acquire(ctx, 1); err != nil {
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

	// 将 TTL 编码到高 8 位；将索引 i 编码到低 8 位
	seq := (ttl << 8) | (i & 0xFF)

	t.inflightRequestLock.Lock()
	hopCh := make(chan Hop, 1)
	t.inflightRequest[seq] = hopCh
	t.inflightRequestLock.Unlock()
	defer func() {
		t.inflightRequestLock.Lock()
		delete(t.inflightRequest, seq)
		t.inflightRequestLock.Unlock()
	}()

	// 将随机 tag 编码到高 8 位；将 pid 的低 8 位编码到低 8 位
	id := int(uint16(t.echoIDTag)<<8 | uint16(t.pidLow))

	var data []byte
	if t.Config.PktSize < 3 {
		data = bytes.Repeat([]byte{0}, t.Config.PktSize)
	} else {
		data = bytes.Repeat([]byte{0}, t.Config.PktSize-3)
		data = append(data, 0x6e, 0x74, 0x72) // "ntr" 作为标识
	}

	icmpHeader := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
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
		return err
	}
	start := time.Now()
	if _, err := t.icmp.WriteTo(wb, &net.IPAddr{IP: t.DestIP}); err != nil {
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
			Error:   errHopLimitTimeout,
		}

		t.res.add(h, i, t.NumMeasurements, t.MaxAttempts)
	}
	return nil
}
