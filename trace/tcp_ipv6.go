package trace

import (
	"context"
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

	"github.com/google/gopacket/layers"
	"golang.org/x/net/icmp"
	"golang.org/x/sync/semaphore"

	"github.com/nxtrace/NTrace-core/trace/internal"
	"github.com/nxtrace/NTrace-core/util"
)

type TCPTracerIPv6 struct {
	Config
	wg         sync.WaitGroup
	res        Result
	pendingMu  sync.Mutex
	pending    map[int]struct{}
	sentMu     sync.RWMutex
	sentAt     map[int]sentInfo
	SrcIP      net.IP
	icmp       net.PacketConn
	final      atomic.Int32
	sem        *semaphore.Weighted
	matchQ     chan matchTask
	readyICMP  chan struct{}
	readyTCP   chan struct{}
	readyPrint chan struct{}
}

func (t *TCPTracerIPv6) waitAllReady(ctx context.Context) {
	timeout := time.After(5 * time.Second)
	waiting := 3
	for waiting > 0 {
		select {
		case <-ctx.Done():
			return
		case <-t.readyICMP:
			waiting--
		case <-t.readyTCP:
			waiting--
		case <-t.readyPrint:
			waiting--
		case <-timeout:
			return
		}
	}
	<-time.After(100 * time.Millisecond)
}

func (t *TCPTracerIPv6) PrintFunc(ctx context.Context, ready chan struct{}, cancel context.CancelCauseFunc) {
	defer t.wg.Done()

	ttl := t.BeginHop - 1
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	close(ready)

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

func (t *TCPTracerIPv6) ttlComp(ttl int) bool {
	idx := ttl - 1
	t.res.lock.RLock()
	defer t.res.lock.RUnlock()
	return idx < len(t.res.Hops) && len(t.res.Hops[idx]) >= t.NumMeasurements
}

func (t *TCPTracerIPv6) launchTTL(ctx context.Context, s *internal.TCPSpec, ttl int) {
	go func(ttl int) {
		for i := 0; i < t.MaxAttempts; i++ {
			// 若此 TTL 已完成或 ctx 已取消，则不再发起新的尝试
			if t.ttlComp(ttl) || ctx.Err() != nil {
				return
			}

			t.wg.Add(1)
			go func(ttl, i int) {
				if err := t.send(ctx, s, ttl, i); err != nil && !errors.Is(err, context.Canceled) {
					if util.EnvDevMode {
						panic(err)
					}
					log.Fatal(err)
				}
			}(ttl, i)

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond * time.Duration(t.PacketInterval)):
			}
		}
	}(ttl)
}

func (t *TCPTracerIPv6) markPending(seq int) {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	t.pending[seq] = struct{}{}
}

func (t *TCPTracerIPv6) clearPending(seq int) bool {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	_, ok := t.pending[seq]
	delete(t.pending, seq)
	return ok
}

func (t *TCPTracerIPv6) storeSent(seq, srcPort int, start time.Time) {
	t.sentMu.Lock()
	defer t.sentMu.Unlock()
	t.sentAt[seq] = sentInfo{srcPort: srcPort, start: start}
}

func (t *TCPTracerIPv6) lookupSent(seq int) (srcPort int, start time.Time, ok bool) {
	t.sentMu.RLock()
	defer t.sentMu.RUnlock()
	si, ok := t.sentAt[seq]
	if !ok {
		return 0, time.Time{}, false
	}
	return si.srcPort, si.start, true
}

func (t *TCPTracerIPv6) dropSent(seq int) {
	t.sentMu.Lock()
	defer t.sentMu.Unlock()
	delete(t.sentAt, seq)
}

func (t *TCPTracerIPv6) addHopWithIndex(peer net.Addr, ttl, i int, rtt time.Duration, mpls []string) {
	if f := t.final.Load(); f != -1 && ttl > int(f) {
		return
	}

	if ip := util.AddrIP(peer); ip != nil && ip.Equal(t.DstIP) {
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

	h := Hop{
		Success: true,
		Address: peer,
		TTL:     ttl,
		RTT:     rtt,
		MPLS:    mpls,
	}

	_ = h.fetchIPData(t.Config) // 忽略错误，继续添加结果

	t.res.add(h, i, t.NumMeasurements, t.MaxAttempts)
}

func (t *TCPTracerIPv6) matchWorker(ctx context.Context) {
	defer t.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-t.matchQ:
			if !ok {
				return
			}

			// 固定等待 10ms，缓解登记竞态
			timer := time.NewTimer(10 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			timer.Stop()

			// 尝试一次匹配
			srcPort, start, ok := t.lookupSent(task.seq)
			if !ok {
				continue
			}

			if task.srcPort != srcPort {
				continue
			}

			if !t.clearPending(task.seq) {
				t.dropSent(task.seq)
				continue
			}

			// 将 task.seq 转为 32 位无符号数
			u := uint32(task.seq)

			// 高 8 位是 TTL
			ttl := int((u >> 24) & 0xFF)

			// 低 24 位是 i
			i := int(u & 0xFFFFFF)
			rtt := task.finish.Sub(start)
			t.addHopWithIndex(task.peer, ttl, i, rtt, task.mpls)
			t.dropSent(task.seq)
		}
	}
}

func (t *TCPTracerIPv6) Execute() (res *Result, err error) {
	// 初始化 pending、sentAt 和 matchQ
	t.pending = make(map[int]struct{})
	t.sentAt = make(map[int]sentInfo)
	t.matchQ = make(chan matchTask, 60)

	// 创建就绪通道
	t.readyICMP = make(chan struct{})
	t.readyTCP = make(chan struct{})
	t.readyPrint = make(chan struct{})

	if len(t.res.Hops) > 0 {
		return &t.res, errTracerouteExecuted
	}

	// 初始化 Result.Hops，并预分配到 MaxHops
	t.res.Hops = make([][]Hop, t.MaxHops)
	t.res.tailDone = make([]bool, t.MaxHops)

	// 解析并校验用户指定的 IPv6 源地址
	SrcAddr := net.ParseIP(t.SrcAddr)
	if t.SrcAddr != "" && !util.IsIPv6(SrcAddr) {
		return nil, errors.New("invalid IPv6 SrcAddr: " + t.SrcAddr)
	}
	t.SrcIP, _ = util.LocalIPPortv6(t.DstIP, SrcAddr, "tcp6")
	if t.SrcIP == nil {
		return nil, errors.New("cannot determine local IPv6 address")
	}

	t.icmp, err = icmp.ListenPacket("ip6:ipv6-icmp", t.SrcIP.String())
	if err != nil {
		return &t.res, err
	}
	defer func() {
		if c := t.icmp; c != nil { // 先拷一份引用，避免 defer 执行时 t.icmp 已被并发改写
			_ = c.Close()
		}
	}()

	s := internal.NewTCPSpec(
		6,
		t.SrcIP,
		t.DstIP,
		t.DstPort,
		t.icmp,
		t.PktSize,
	)

	s.InitTCP()
	defer s.Close()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancelCause(sigCtx)
	t.final.Store(-1)

	workerN := 16
	for i := 0; i < workerN; i++ {
		t.wg.Add(1)
		go t.matchWorker(ctx)
	}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		s.ListenICMP(ctx, t.readyICMP, func(msg internal.ReceivedMessage, finish time.Time, data []byte) {
			t.handleICMPMessage(msg, finish, data)
		},
		)
	}()
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		s.ListenTCP(ctx, t.readyTCP, func(srcPort, seq int, peer net.Addr, finish time.Time) {
			// 非阻塞投递，队列满则丢弃任务（由超时协程兜底）
			select {
			case t.matchQ <- matchTask{
				srcPort: srcPort, seq: seq, peer: peer, finish: finish, mpls: nil,
			}:
			default:
				// 丢弃以避免阻塞抓包循环
			}
		})
	}()
	t.wg.Add(1)
	go t.PrintFunc(ctx, t.readyPrint, cancel)
	t.waitAllReady(ctx)

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		// 立即启动 BeginHop 对应的 TTL 组
		t.launchTTL(ctx, s, t.BeginHop)

		// 之后按 TTLInterval 周期启动后续 TTL 组
		ticker := time.NewTicker(time.Millisecond * time.Duration(t.TTLInterval))
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
			t.launchTTL(ctx, s, ttl)
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

func (t *TCPTracerIPv6) handleICMPMessage(msg internal.ReceivedMessage, finish time.Time, data []byte) {
	mpls := extractMPLS(msg)

	header, err := util.GetICMPResponsePayload(data)
	if err != nil {
		return
	}

	srcPort, dstPort, err := util.GetTCPPorts(header)
	if err != nil {
		return
	}

	if dstPort != t.DstPort {
		return
	}

	seq, err := util.GetTCPSeq(header)
	if err != nil {
		return
	}

	// 非阻塞投递；如果队列已满则直接丢弃该任务
	select {
	case t.matchQ <- matchTask{
		srcPort: srcPort, seq: int(seq), peer: msg.Peer, finish: finish, mpls: mpls,
	}:
	default:
		// 丢弃以避免阻塞抓包循环
	}
}

func (t *TCPTracerIPv6) send(ctx context.Context, s *internal.TCPSpec, ttl, i int) error {
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

	// 将 TTL 编码到高 8 位；将索引 i 编码到低 24 位
	seq := (ttl << 24) | (i & 0xFFFFFF)

	_, SrcPort := func() (net.IP, int) {
		if !util.RandomPortEnabled() && t.SrcPort > 0 {
			return nil, t.SrcPort
		}
		return util.LocalIPPortv6(t.DstIP, t.SrcIP, "tcp6")
	}()

	ipHeader := &layers.IPv6{
		Version:    6,
		SrcIP:      t.SrcIP,
		DstIP:      t.DstIP,
		NextHeader: layers.IPProtocolTCP,
		HopLimit:   uint8(ttl),
	}

	tcpHeader := &layers.TCP{
		SrcPort: layers.TCPPort(SrcPort),
		DstPort: layers.TCPPort(t.DstPort),
		Seq:     uint32(seq),
		SYN:     true,
		Window:  14600,
		Options: []layers.TCPOption{
			{OptionType: layers.TCPOptionKindMSS, OptionLength: 4, OptionData: []byte{0x05, 0xA0}}, // MSS=1440
		},
	}

	desiredPayloadSize := t.PktSize
	payload := make([]byte, desiredPayloadSize)

	// 设置随机种子
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for k := range payload {
		payload[k] = byte(r.Intn(256))
	}

	// 登记 pending，并启动超时守护
	t.markPending(seq)
	go func(seq, ttl, i int) {
		select {
		case <-ctx.Done():
			_ = t.clearPending(seq)
			return
		case <-time.After(t.Timeout):
			// 仍未完成且未超出 final/未达成 ttlComp 才补位
			if !t.clearPending(seq) {
				return
			}

			if f := t.final.Load(); f != -1 && ttl > int(f) {
				return
			}

			if t.ttlComp(ttl) {
				return
			}

			h := Hop{
				Success: false,
				Address: nil,
				TTL:     ttl,
				RTT:     0,
				Error:   errHopLimitTimeout,
			}

			t.res.add(h, i, t.NumMeasurements, t.MaxAttempts)
			t.dropSent(seq)
		}
	}(seq, ttl, i)

	start, err := s.SendTCP(ctx, ipHeader, tcpHeader, payload)
	if err != nil {
		return err
	}
	t.storeSent(seq, SrcPort, start)
	return nil
}
