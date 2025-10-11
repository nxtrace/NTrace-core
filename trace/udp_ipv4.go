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
	"golang.org/x/sync/semaphore"

	"github.com/nxtrace/NTrace-core/trace/internal"
	"github.com/nxtrace/NTrace-core/util"
)

type UDPTracer struct {
	Config
	wg        sync.WaitGroup
	res       Result
	ttlQueues map[int][]attemptPort
	ttlQMu    sync.Mutex
	pending   map[attemptKey]struct{}
	pendingMu sync.Mutex
	sentAt    map[int]sentInfo
	sentMu    sync.RWMutex
	SrcIP     net.IP
	final     atomic.Int32
	sem       *semaphore.Weighted
	matchQ    chan matchTask
	readyOut  chan struct{}
	readyICMP chan struct{}
	readyUDP  chan struct{}
}

func (t *UDPTracer) waitAllReady(ctx context.Context) {
	timeout := time.After(5 * time.Second)
	waiting := 3
	for waiting > 0 {
		select {
		case <-ctx.Done():
			return
		case <-t.readyOut:
			waiting--
		case <-t.readyICMP:
			waiting--
		case <-t.readyUDP:
			waiting--
		case <-timeout:
			return
		}
	}
	<-time.After(100 * time.Millisecond)
}

func (t *UDPTracer) ttlComp(ttl int) bool {
	idx := ttl - 1
	t.res.lock.RLock()
	defer t.res.lock.RUnlock()
	return idx < len(t.res.Hops) && len(t.res.Hops[idx]) >= t.NumMeasurements
}

func (t *UDPTracer) PrintFunc(ctx context.Context, cancel context.CancelCauseFunc) {
	defer t.wg.Done()

	ttl := t.BeginHop - 1
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

func (t *UDPTracer) launchTTL(ctx context.Context, s *internal.UDPSpec, ttl int) {
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

func (t *UDPTracer) tryMatchTTLPort(ttl, srcPort int) (int, bool) {
	t.ttlQMu.Lock()
	defer t.ttlQMu.Unlock()
	q := t.ttlQueues[ttl]
	if len(q) == 0 {
		return 0, false
	}
	head := q[0]
	if head.srcPort != srcPort {
		return 0, false
	}
	t.ttlQueues[ttl] = q[1:]
	return head.i, true
}

func (t *UDPTracer) enqueueTTLPort(ttl, i, srcPort int) {
	ap := attemptPort{srcPort: srcPort, i: i}
	t.ttlQMu.Lock()
	defer t.ttlQMu.Unlock()
	t.ttlQueues[ttl] = append(t.ttlQueues[ttl], ap)
}

func (t *UDPTracer) markPending(ttl, i int) {
	key := attemptKey{ttl: ttl, i: i}
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	t.pending[key] = struct{}{}
}

func (t *UDPTracer) clearPending(ttl, i int) bool {
	key := attemptKey{ttl: ttl, i: i}
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	_, ok := t.pending[key]
	delete(t.pending, key)
	return ok
}

func (t *UDPTracer) storeSent(seq, ttl, i, srcPort int, start time.Time) {
	t.sentMu.Lock()
	defer t.sentMu.Unlock()
	if t.OSType != 1 {
		t.sentAt[seq] = sentInfo{srcPort: srcPort, start: start}
	} else {
		t.sentAt[seq] = sentInfo{ttl: ttl, i: i, srcPort: srcPort, start: start}
	}
}

func (t *UDPTracer) lookupSent(seq int) (ttl, i, srcPort int, start time.Time, ok bool) {
	t.sentMu.RLock()
	defer t.sentMu.RUnlock()
	si, ok := t.sentAt[seq]
	if !ok {
		return 0, 0, 0, time.Time{}, false
	}
	return si.ttl, si.i, si.srcPort, si.start, true
}

func (t *UDPTracer) dropSent(seq int) {
	t.sentMu.Lock()
	defer t.sentMu.Unlock()
	delete(t.sentAt, seq)
}

func (t *UDPTracer) dropByAttempt(ttl, i int) {
	t.sentMu.Lock()
	defer t.sentMu.Unlock()
	for k, si := range t.sentAt {
		if si.ttl == ttl && si.i == i {
			delete(t.sentAt, k)
			return
		}
	}
}

func (t *UDPTracer) addHopWithIndex(peer net.Addr, ttl, i int, rtt time.Duration, mpls []string) {
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

func (t *UDPTracer) matchWorker(ctx context.Context) {
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
			ttl, i, srcPort, start, ok := t.lookupSent(task.seq)
			if !ok {
				continue
			}

			if task.srcPort != srcPort {
				continue
			}

			if t.OSType != 1 {
				// 将 task.seq 转为 16 位无符号数
				u := uint16(task.seq)

				// 高 8 位是 TTL
				ttl = int((u >> 8) & 0xFF)

				// 低 8 位是索引 i
				i = int(u & 0xFF)
			}

			if t.clearPending(ttl, i) {
				rtt := task.finish.Sub(start)
				t.addHopWithIndex(task.peer, ttl, i, rtt, task.mpls)
			}
			t.dropSent(task.seq)
		}
	}
}

func (t *UDPTracer) Execute() (res *Result, err error) {
	// 初始化 ttlQueues、pending、sentAt 和 matchQ
	t.ttlQueues = make(map[int][]attemptPort)
	t.pending = make(map[attemptKey]struct{})
	t.sentAt = make(map[int]sentInfo)
	t.matchQ = make(chan matchTask, 60)

	// 创建就绪通道
	t.readyOut = make(chan struct{})
	t.readyICMP = make(chan struct{})
	t.readyUDP = make(chan struct{})

	if len(t.res.Hops) > 0 {
		return &t.res, errTracerouteExecuted
	}

	// 初始化 res.Hops 和 res.tailDone，并预分配到 MaxHops
	t.res.Hops = make([][]Hop, t.MaxHops)
	t.res.tailDone = make([]bool, t.MaxHops)

	// 解析并校验用户指定的 IPv4 源地址
	SrcAddr := net.ParseIP(t.SrcAddr).To4()
	if t.SrcAddr != "" && SrcAddr == nil {
		return nil, errors.New("invalid IPv4 SrcAddr:" + t.SrcAddr)
	}
	t.SrcIP, _ = util.LocalIPPort(t.DstIP, SrcAddr, "udp")
	if t.SrcIP == nil {
		return nil, errors.New("cannot determine local IPv4 address")
	}

	s := internal.NewUDPSpec(
		4,
		t.ICMPMode,
		t.SrcIP,
		t.DstIP,
		t.DstPort,
	)

	s.InitICMP()
	s.InitUDP()
	defer s.Close()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancelCause(sigCtx)
	t.final.Store(-1)

	workerN := 16
	for i := 0; i < workerN; i++ {
		t.wg.Add(1)
		go t.matchWorker(ctx)
	}
	if t.OSType == 1 {
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			s.ListenOut(ctx, t.readyOut, func(srcPort, seq, ttl int, start time.Time) {
				// 严格按队列头端口匹配；不匹配就丢弃，避免混入其它进程/杂包
				i, ok := t.tryMatchTTLPort(ttl, srcPort)
				if !ok {
					return
				}
				t.storeSent(seq, ttl, i, srcPort, start)
			})
		}()
	} else {
		close(t.readyOut)
	}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		s.ListenICMP(ctx, t.readyICMP, func(msg internal.ReceivedMessage, finish time.Time, data []byte) {
			t.handleICMPMessage(msg, finish, data)
		},
		)
	}()
	t.waitAllReady(ctx)
	t.wg.Add(1)
	go t.PrintFunc(ctx, cancel)

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		// 立即启动 BeginHop 对应的 TTL 组
		t.launchTTL(ctx, s, t.BeginHop)

		for ttl := t.BeginHop + 1; ttl <= t.MaxHops; ttl++ {
			// 之后按 TTLInterval 周期启动后续 TTL 组
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond * time.Duration(t.TTLInterval)):
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

	final := int(t.final.Load())
	if final == -1 {
		final = t.MaxHops
	}
	t.res.reduce(final)

	if cause := context.Cause(ctx); !errors.Is(cause, errNaturalDone) {
		return &t.res, cause
	}
	return &t.res, nil
}

func (t *UDPTracer) handleICMPMessage(msg internal.ReceivedMessage, finish time.Time, data []byte) {
	mpls := extractMPLS(msg)

	seq, err := util.GetUDPSeq(data)
	if err != nil {
		return
	}

	header, err := util.GetICMPResponsePayload(data)
	if err != nil {
		return
	}

	srcPort, dstPort, err := util.GetUDPPorts(header)
	if err != nil {
		return
	}

	if dstPort != t.DstPort {
		return
	}

	// 非阻塞投递；如果队列已满则直接丢弃该任务
	select {
	case t.matchQ <- matchTask{
		srcPort: srcPort, seq: seq, peer: msg.Peer, finish: finish, mpls: mpls,
	}:
	default:
		// 丢弃以避免阻塞抓包循环
	}
}

func (t *UDPTracer) send(ctx context.Context, s *internal.UDPSpec, ttl, i int) error {
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

	_, SrcPort := func() (net.IP, int) {
		if !util.RandomPortEnabled() && t.SrcPort > 0 {
			return nil, t.SrcPort
		}
		return util.LocalIPPort(t.DstIP, t.SrcIP, "udp")
	}()

	ipHeader := &layers.IPv4{
		Version:  4,
		Id:       uint16(seq),
		SrcIP:    t.SrcIP,
		DstIP:    t.DstIP,
		Protocol: layers.IPProtocolUDP,
		TTL:      uint8(ttl),
	}

	udpHeader := &layers.UDP{
		SrcPort: layers.UDPPort(SrcPort),
		DstPort: layers.UDPPort(t.DstPort),
	}

	desiredPayloadSize := t.PktSize
	payload := make([]byte, desiredPayloadSize)

	// 设置随机种子
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for k := range payload {
		payload[k] = byte(r.Intn(256))
	}

	if t.OSType == 1 {
		// 记录 TTL 队列
		t.enqueueTTLPort(ttl, i, SrcPort)
	}

	// 登记 pending，并启动超时守护
	t.markPending(ttl, i)
	go func(seq, ttl, i int) {
		select {
		case <-ctx.Done():
			_ = t.clearPending(ttl, i)
			return
		case <-time.After(t.Timeout):
			// 仍未完成且未超出 final/未达成 ttlComp 才补位
			if !t.clearPending(ttl, i) {
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
			if t.OSType != 1 {
				t.dropSent(seq)
			} else {
				t.dropByAttempt(ttl, i)
			}
		}
	}(seq, ttl, i)

	start, err := s.SendUDP(ctx, ipHeader, udpHeader, payload)
	if err != nil {
		_ = t.clearPending(ttl, i)
		return err
	}

	if t.OSType != 1 {
		t.storeSent(seq, 0, 0, SrcPort, start)
	}
	return nil
}
