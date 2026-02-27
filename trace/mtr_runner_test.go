package trace

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/trace/internal"
)

// ---------------------------------------------------------------------------
// Mock prober（实现 mtrProber 接口）
// ---------------------------------------------------------------------------

type mockProber struct {
	roundFn func(ctx context.Context) (*Result, error)
	closed  int32
}

func (m *mockProber) probeRound(ctx context.Context) (*Result, error) {
	return m.roundFn(ctx)
}

func (m *mockProber) close() {
	atomic.AddInt32(&m.closed, 1)
}

func constantResultProber(res *Result) *mockProber {
	return &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			return res, nil
		},
	}
}

// 快速退避配置，避免测试阻塞
var fastBackoff = &mtrBackoffCfg{
	Initial:   time.Millisecond,
	Max:       5 * time.Millisecond,
	MaxConsec: 5,
}

// ---------------------------------------------------------------------------
// 测试用例：通过 mtrLoop + mockProber 覆盖 RunMTR 主循环逻辑
// ---------------------------------------------------------------------------

func TestMTRLoopMaxRounds(t *testing.T) {
	maxRounds := 5
	res := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
		[]Hop{mkHop(2, "2.2.2.2", 20*time.Millisecond)},
	)
	prober := constantResultProber(res)
	agg := NewMTRAggregator()

	var snapshots int
	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds: maxRounds,
		Interval:  time.Millisecond,
	}, agg, func(iter int, stats []MTRHopStat) {
		snapshots++
	}, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshots != maxRounds {
		t.Errorf("expected %d snapshots, got %d", maxRounds, snapshots)
	}
	if atomic.LoadInt32(&prober.closed) != 1 {
		t.Error("prober.close() was not called")
	}
}

func TestMTRLoopCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var count int32
	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			n := atomic.AddInt32(&count, 1)
			if n >= 3 {
				cancel()
			}
			return mkResult(
				[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
			), nil
		},
	}
	agg := NewMTRAggregator()

	err := mtrLoop(ctx, prober, Config{}, MTROptions{
		Interval: time.Millisecond,
	}, agg, func(_ int, _ []MTRHopStat) {}, false, fastBackoff)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	c := atomic.LoadInt32(&count)
	if c < 3 {
		t.Errorf("expected at least 3 probe rounds, got %d", c)
	}
}

func TestMTRLoopErrorBackoff(t *testing.T) {
	errProbe := errors.New("temporary error")
	var callTimes []time.Time

	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			callTimes = append(callTimes, time.Now())
			return nil, errProbe
		},
	}
	agg := NewMTRAggregator()

	bo := &mtrBackoffCfg{
		Initial:   10 * time.Millisecond,
		Max:       50 * time.Millisecond,
		MaxConsec: 5,
	}

	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		Interval: time.Millisecond,
	}, agg, func(_ int, _ []MTRHopStat) {}, false, bo)

	if err == nil {
		t.Fatal("expected error from consecutive failures")
	}
	if len(callTimes) != bo.MaxConsec {
		t.Errorf("expected %d probe calls, got %d", bo.MaxConsec, len(callTimes))
	}

	// 验证退避间隔递增（至少前几两次差值应递增）
	if len(callTimes) >= 3 {
		gap1 := callTimes[1].Sub(callTimes[0])
		gap2 := callTimes[2].Sub(callTimes[1])
		if gap2 <= gap1/2 {
			t.Errorf("expected increasing backoff gaps, gap1=%v gap2=%v", gap1, gap2)
		}
	}
}

func TestMTRLoopErrorRecovery(t *testing.T) {
	var count int32
	errProbe := errors.New("temporary error")

	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			n := atomic.AddInt32(&count, 1)
			if n <= 3 {
				return nil, errProbe
			}
			return mkResult(
				[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
			), nil
		},
	}
	agg := NewMTRAggregator()

	var snapshots int
	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds: 2,
		Interval:  time.Millisecond,
	}, agg, func(_ int, _ []MTRHopStat) {
		snapshots++
	}, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshots != 2 {
		t.Errorf("expected 2 successful snapshots after recovery, got %d", snapshots)
	}
}

func TestMTRLoopTimeoutHops(t *testing.T) {
	res := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
		[]Hop{mkTimeoutHop(2)},
		[]Hop{mkHop(3, "3.3.3.3", 30*time.Millisecond)},
	)
	prober := constantResultProber(res)
	agg := NewMTRAggregator()

	var finalStats []MTRHopStat
	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds: 1,
		Interval:  time.Millisecond,
	}, agg, func(_ int, stats []MTRHopStat) {
		finalStats = stats
	}, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(finalStats) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(finalStats))
	}
	if finalStats[1].Loss != 100 {
		t.Errorf("expected 100%% loss for timeout hop, got %f", finalStats[1].Loss)
	}
}

func TestMTRLoopSnapshotIterations(t *testing.T) {
	var round int32
	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			n := atomic.AddInt32(&round, 1)
			return mkResult(
				[]Hop{mkHop(1, "1.1.1.1", time.Duration(n)*time.Millisecond)},
			), nil
		},
	}
	agg := NewMTRAggregator()

	var iterations []int
	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds: 3,
		Interval:  time.Millisecond,
	}, agg, func(iter int, _ []MTRHopStat) {
		iterations = append(iterations, iter)
	}, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{1, 2, 3}
	if len(iterations) != len(expected) {
		t.Fatalf("expected %d iterations, got %d", len(expected), len(iterations))
	}
	for i, v := range expected {
		if iterations[i] != v {
			t.Errorf("iteration %d: expected %d, got %d", i, v, iterations[i])
		}
	}
}

func TestMTRLoopCloseCalledOnError(t *testing.T) {
	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			return nil, errors.New("always fail")
		},
	}
	agg := NewMTRAggregator()

	_ = mtrLoop(context.Background(), prober, Config{}, MTROptions{
		Interval: time.Millisecond,
	}, agg, nil, false, fastBackoff)

	if atomic.LoadInt32(&prober.closed) != 1 {
		t.Error("prober.close() was not called on error exit")
	}
}

func TestMTRLoopCloseCalledOnSuccess(t *testing.T) {
	prober := constantResultProber(mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
	))
	agg := NewMTRAggregator()

	_ = mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds: 1,
		Interval:  time.Millisecond,
	}, agg, nil, false, fastBackoff)

	if atomic.LoadInt32(&prober.closed) != 1 {
		t.Error("prober.close() was not called on normal exit")
	}
}

func TestMTRLoopNilOnSnapshot(t *testing.T) {
	prober := constantResultProber(mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
	))
	agg := NewMTRAggregator()

	// 确保 onSnapshot=nil 不 panic
	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds: 2,
		Interval:  time.Millisecond,
	}, agg, nil, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Bug-fix 验证测试
// ---------------------------------------------------------------------------

// TestMTRLoopCancelDuringIntervalCallsSnapshot 验证在 interval 等待期间
// ctx 取消时仍然会调用 onSnapshot（Bug fix #2）。
func TestMTRLoopCancelDuringIntervalCallsSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var round int32
	var lastSnapshotIter int

	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			n := atomic.AddInt32(&round, 1)
			if n >= 2 {
				// 第二轮成功后，在间隔等待期间取消
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
			}
			return mkResult(
				[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
			), nil
		},
	}
	agg := NewMTRAggregator()

	err := mtrLoop(ctx, prober, Config{}, MTROptions{
		Interval: 5 * time.Second, // 足够长，确保在间隔中取消
	}, agg, func(iter int, _ []MTRHopStat) {
		lastSnapshotIter = iter
	}, false, fastBackoff)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// 关键：取消路径也应调用 onSnapshot
	if lastSnapshotIter < 2 {
		t.Errorf("expected at least 2 snapshot calls (last iter=%d)", lastSnapshotIter)
	}
}

// TestMTRLoopCancelDuringBackoffCallsSnapshot 验证在错误退避等待期间
// ctx 取消时也会调用 onSnapshot（Bug fix #2 扩展）。
func TestMTRLoopCancelDuringBackoffCallsSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// 第一轮成功，第二轮失败，然后在退避期间取消
	var count int32
	var snapshotCalled int32

	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			n := atomic.AddInt32(&count, 1)
			if n == 1 {
				return mkResult(
					[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
				), nil
			}
			// 第二轮失败，在退避等待期间取消
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
			return nil, errors.New("fail")
		},
	}
	agg := NewMTRAggregator()

	bo := &mtrBackoffCfg{
		Initial:   5 * time.Second, // 长退避，确保在退避中取消
		Max:       10 * time.Second,
		MaxConsec: 5,
	}

	err := mtrLoop(ctx, prober, Config{}, MTROptions{
		Interval: time.Millisecond,
	}, agg, func(iter int, _ []MTRHopStat) {
		atomic.AddInt32(&snapshotCalled, 1)
	}, false, bo)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// 退避取消路径也应调用 onSnapshot
	if atomic.LoadInt32(&snapshotCalled) < 1 {
		t.Error("expected onSnapshot to be called during backoff cancel")
	}
}

// TestMTRLoopPause 验证 IsPaused 暂停探测行为。
func TestMTRLoopPause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var pauseFlag int32 // 1 = paused
	var round int32

	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			n := atomic.AddInt32(&round, 1)
			if n == 2 {
				// 第二轮后暂停
				atomic.StoreInt32(&pauseFlag, 1)
				// 0.5s 后恢复
				go func() {
					time.Sleep(100 * time.Millisecond)
					atomic.StoreInt32(&pauseFlag, 0)
				}()
			}
			return mkResult(
				[]Hop{mkHop(1, "1.1.1.1", time.Duration(n)*time.Millisecond)},
			), nil
		},
	}
	agg := NewMTRAggregator()

	var snapshots int
	err := mtrLoop(ctx, prober, Config{}, MTROptions{
		MaxRounds: 4,
		Interval:  time.Millisecond,
		IsPaused:  func() bool { return atomic.LoadInt32(&pauseFlag) == 1 },
	}, agg, func(iter int, _ []MTRHopStat) {
		snapshots++
	}, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshots != 4 {
		t.Errorf("expected 4 snapshots, got %d", snapshots)
	}
}

// ---------------------------------------------------------------------------
// onICMP 直接单测：seq 回卷 + 迟到回包 + RTT 合理性检查
// ---------------------------------------------------------------------------

// newTestICMPEngine 构造一个最小可测试的 mtrICMPEngine（不创建真实 socket）。
func newTestICMPEngine(timeout time.Duration) *mtrICMPEngine {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &mtrICMPEngine{
		config:   Config{Timeout: timeout},
		notifyCh: make(chan struct{}, 1),
		sentAt:   make(map[int]mtrProbeMeta),
		replied:  make(map[int]*mtrProbeReply),
	}
}

// TestOnICMP_NormalReply 正常回包应被接受。
func TestOnICMP_NormalReply(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	atomic.StoreUint32(&e.roundID, 1)

	now := time.Now()
	seq := 42
	e.sentAt[seq] = mtrProbeMeta{ttl: 3, start: now, roundID: 1}

	peer := &net.IPAddr{IP: net.ParseIP("8.8.8.8")}
	e.onICMP(internal.ReceivedMessage{Peer: peer}, now.Add(15*time.Millisecond), seq)

	if _, ok := e.replied[seq]; !ok {
		t.Fatal("normal reply should be accepted")
	}
	if e.replied[seq].rtt != 15*time.Millisecond {
		t.Errorf("expected RTT 15ms, got %v", e.replied[seq].rtt)
	}
}

// TestOnICMP_StaleRoundReply 旧轮次回包（roundID 不匹配）应被丢弃。
func TestOnICMP_StaleRoundReply(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	atomic.StoreUint32(&e.roundID, 5)

	now := time.Now()
	seq := 100
	// 旧轮次 roundID=3，当前轮次=5
	e.sentAt[seq] = mtrProbeMeta{ttl: 2, start: now, roundID: 3}

	peer := &net.IPAddr{IP: net.ParseIP("1.2.3.4")}
	e.onICMP(internal.ReceivedMessage{Peer: peer}, now.Add(10*time.Millisecond), seq)

	if _, ok := e.replied[seq]; ok {
		t.Fatal("stale round reply should be discarded")
	}
	// sentAt 条目也应被删除
	if _, ok := e.sentAt[seq]; ok {
		t.Fatal("stale sentAt entry should be cleaned up")
	}
}

// TestOnICMP_SeqWrapStaleReply 模拟 seq 16 位回卷后迟到回包场景。
//
// 场景：
//  1. 轮次 N 发送 seq=100，记录在 sentAt
//  2. 经过 65536 次递增，seq 回卷到 100
//  3. 轮次 N+K 重新使用 seq=100，sentAt[100] 已更新为新轮次数据
//  4. 轮次 N 的迟到回包到达，finish 时间远晚于新轮次的发送时间
//
// 预期：RTT > timeout，被 RTT 合理性检查丢弃。
func TestOnICMP_SeqWrapStaleReply(t *testing.T) {
	timeout := 2 * time.Second
	e := newTestICMPEngine(timeout)
	atomic.StoreUint32(&e.roundID, 2000)

	// 模拟新轮次刚刚发送 seq=100（1ms 前）
	newSendTime := time.Now()
	seq := 100
	e.sentAt[seq] = mtrProbeMeta{
		ttl:     5,
		start:   newSendTime,
		roundID: 2000,
	}

	// 迟到回包：来自 ~36 分钟前的旧轮次，到达时间是 "现在"
	// RTT = now - newSendTime 中间插入一个巨大偏移来模拟跨轮错配
	staleFinish := newSendTime.Add(5 * time.Second) // RTT 5s >> timeout 2s

	peer := &net.IPAddr{IP: net.ParseIP("10.0.0.1")}
	e.onICMP(internal.ReceivedMessage{Peer: peer}, staleFinish, seq)

	if _, ok := e.replied[seq]; ok {
		t.Fatal("stale reply with RTT > timeout should be discarded (seq wraparound)")
	}
	if _, ok := e.sentAt[seq]; ok {
		t.Fatal("sentAt entry should be cleaned up after stale discard")
	}
}

// TestOnICMP_NegativeRTT 时间倒退（finish < start）的回包应被丢弃。
func TestOnICMP_NegativeRTT(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	atomic.StoreUint32(&e.roundID, 1)

	now := time.Now()
	seq := 200
	e.sentAt[seq] = mtrProbeMeta{ttl: 1, start: now, roundID: 1}

	// finish 早于 start → RTT < 0
	peer := &net.IPAddr{IP: net.ParseIP("10.0.0.2")}
	e.onICMP(internal.ReceivedMessage{Peer: peer}, now.Add(-100*time.Millisecond), seq)

	if _, ok := e.replied[seq]; ok {
		t.Fatal("negative RTT reply should be discarded")
	}
}

// TestOnICMP_ExactTimeoutBoundary RTT 恰好等于 timeout 的回包仍应被接受。
// 比较使用 > 而非 >=，刚好到达 timeout 的回包不算超时。
func TestOnICMP_ExactTimeoutBoundary(t *testing.T) {
	timeout := 2 * time.Second
	e := newTestICMPEngine(timeout)
	atomic.StoreUint32(&e.roundID, 1)

	now := time.Now()
	seq := 300
	e.sentAt[seq] = mtrProbeMeta{ttl: 1, start: now, roundID: 1}

	// RTT == timeout（不是 >，而是恰好等于）
	peer := &net.IPAddr{IP: net.ParseIP("10.0.0.3")}
	e.onICMP(internal.ReceivedMessage{Peer: peer}, now.Add(timeout), seq)

	// RTT == timeout，不满足 rtt > maxRTT，应被接受
	if _, ok := e.replied[seq]; !ok {
		t.Fatal("reply with RTT == timeout should still be accepted")
	}
}

// TestOnICMP_UnknownSeq 未知 seq 的回包应被静默忽略。
func TestOnICMP_UnknownSeq(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	atomic.StoreUint32(&e.roundID, 1)

	peer := &net.IPAddr{IP: net.ParseIP("10.0.0.4")}
	e.onICMP(internal.ReceivedMessage{Peer: peer}, time.Now(), 999)

	if len(e.replied) != 0 {
		t.Fatal("unknown seq should not produce any reply")
	}
}

// ---------------------------------------------------------------------------
// seqWillWrap 单测
// ---------------------------------------------------------------------------

func TestSeqWillWrap_NoWrap(t *testing.T) {
	// 当前 seq=0，发 30 个探针，远不到 0xFFFF
	if seqWillWrap(0, 30) {
		t.Fatal("seq=0, probeCount=30 should not wrap")
	}
}

func TestSeqWillWrap_JustBelowBoundary(t *testing.T) {
	// 当前低 16 位 = 0xFFFF - 30 = 0xFFE1，发 30 个刚好不回卷
	counter := uint32(0xFFFF - 30)
	if seqWillWrap(counter, 30) {
		t.Fatal("exactly fitting should not trigger wraparound")
	}
}

func TestSeqWillWrap_OneOver(t *testing.T) {
	// 当前低 16 位 = 0xFFFF - 29 = 0xFFE2，发 30 个会越界
	counter := uint32(0xFFFF - 29)
	if !seqWillWrap(counter, 30) {
		t.Fatal("should detect imminent wraparound")
	}
}

func TestSeqWillWrap_AtMax(t *testing.T) {
	// 当前低 16 位 = 0xFFFF，发 1 个就越界
	if !seqWillWrap(0xFFFF, 1) {
		t.Fatal("seq=0xFFFF + 1 probe must trigger wraparound")
	}
}

func TestSeqWillWrap_HighBitsIgnored(t *testing.T) {
	// seqCounter 高 16 位非零，低 16 位安全
	counter := uint32(0x0003_0001) // 低 16 位 = 1
	if seqWillWrap(counter, 30) {
		t.Fatal("high bits should be masked; low 16 bits = 1 with 30 probes is safe")
	}
}

func TestSeqWillWrap_HighBitsWrap(t *testing.T) {
	// seqCounter 高位非零，低 16 位接近边界
	counter := uint32(0x0005_FFF0) // 低 16 位 = 0xFFF0
	if !seqWillWrap(counter, 20) {
		t.Fatal("0xFFF0 + 20 > 0xFFFF, should detect wraparound")
	}
}

func TestSeqWillWrap_ZeroProbes(t *testing.T) {
	if seqWillWrap(0xFFFF, 0) {
		t.Fatal("probeCount=0 should never trigger wraparound")
	}
}

func TestSeqWillWrap_NegativeProbes(t *testing.T) {
	// beginHop > maxHops → probeCount 为负，不应回卷
	if seqWillWrap(0xFFFF, -5) {
		t.Fatal("negative probeCount should never trigger wraparound")
	}
}

// TestProbeRound_BeginHopExceedsMaxHops 验证 beginHop > maxHops 时：
//   - seqWillWrap 不误判（不触发 rotateEngine）
//   - probeRound 返回 maxHops 长度的 Hops，但全部为 nil（两个循环均被跳过）
//   - seqCounter 不递增（未发送任何探针）
func TestProbeRound_BeginHopExceedsMaxHops(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	e.config.BeginHop = 10
	e.config.MaxHops = 5
	// seqCounter 接近边界 — 若 probeCount 保护缺失会触发 rotateEngine（此处无 spec 会 panic）
	atomic.StoreUint32(&e.seqCounter, 0xFFF0)

	seqBefore := atomic.LoadUint32(&e.seqCounter)

	res, err := e.probeRound(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Hops 长度应为 maxHops
	if len(res.Hops) != 5 {
		t.Fatalf("expected 5 hop slots, got %d", len(res.Hops))
	}

	// 两个 for ttl:=10; ttl<=5 循环都被跳过，Hops 全部为 nil
	for i, hops := range res.Hops {
		if hops != nil {
			t.Errorf("Hops[%d] should be nil (loop skipped), got %v", i, hops)
		}
	}

	// 未发送任何探针，seqCounter 应不变
	if atomic.LoadUint32(&e.seqCounter) != seqBefore {
		t.Errorf("seqCounter should not change, was %d now %d", seqBefore, atomic.LoadUint32(&e.seqCounter))
	}
}

// ---------------------------------------------------------------------------
// 重置统计（r 键）测试
// ---------------------------------------------------------------------------

// TestMTRLoop_RestartStatistics 验证 IsResetRequested 触发统计重置。
func TestMTRLoop_RestartStatistics(t *testing.T) {
	var round int32
	var resetOnce int32

	prober := &mockProber{
		roundFn: func(_ context.Context) (*Result, error) {
			n := atomic.AddInt32(&round, 1)
			return mkResult(
				[]Hop{mkHop(1, "1.1.1.1", time.Duration(n)*time.Millisecond)},
			), nil
		},
	}
	agg := NewMTRAggregator()

	var iterations []int
	var sntValues []int

	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds: 4,
		Interval:  time.Millisecond,
		IsResetRequested: func() bool {
			// 第 2 轮后触发一次重置
			r := atomic.LoadInt32(&round)
			if r == 2 && atomic.CompareAndSwapInt32(&resetOnce, 0, 1) {
				return true
			}
			return false
		},
	}, agg, func(iter int, stats []MTRHopStat) {
		iterations = append(iterations, iter)
		if len(stats) > 0 {
			sntValues = append(sntValues, stats[0].Snt)
		}
	}, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 重置后 iteration 从 0 重开，所以需要达到 4 轮才结束
	// 轮次序列：round 1 → iter 1, round 2 → iter 2, [reset → iter=0],
	//           round 3 → iter 1, round 4 → iter 2, round 5 → iter 3, round 6 → iter 4
	// 最终必须 iteration 中出现 4
	found := false
	for _, v := range iterations {
		if v == 4 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected iteration to reach 4 after reset, got %v", iterations)
	}

	// 验证重置后 Snt 从 1 重新开始
	sntOneCount := 0
	for _, s := range sntValues {
		if s == 1 {
			sntOneCount++
		}
	}
	// Snt=1 应至少出现 2 次（初始第一轮 + 重置后第一轮）
	if sntOneCount < 2 {
		t.Errorf("expected Snt=1 at least twice (initial + after reset), got %d occurrences in %v", sntOneCount, sntValues)
	}
}

// TestResetClearsKnownFinalTTL 验证 resetFinalTTL 清除已知目的地 TTL 缓存。
func TestResetClearsKnownFinalTTL(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	atomic.StoreInt32(&e.knownFinalTTL, 5)

	e.resetFinalTTL()

	if got := atomic.LoadInt32(&e.knownFinalTTL); got != -1 {
		t.Errorf("expected knownFinalTTL=-1 after reset, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// 目的地停止测试
// ---------------------------------------------------------------------------

// TestOnICMP_DetectsDestination 验证 onICMP 在 peer==DstIP 时设置 roundFinalTTL。
func TestOnICMP_DetectsDestination(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	e.config.DstIP = net.ParseIP("8.8.8.8")
	atomic.StoreUint32(&e.roundID, 1)
	atomic.StoreInt32(&e.roundFinalTTL, -1)
	atomic.StoreInt32(&e.knownFinalTTL, -1)

	now := time.Now()
	seq := 42
	e.sentAt[seq] = mtrProbeMeta{ttl: 5, start: now, roundID: 1}

	peer := &net.IPAddr{IP: net.ParseIP("8.8.8.8")}
	e.onICMP(internal.ReceivedMessage{Peer: peer}, now.Add(15*time.Millisecond), seq)

	if got := atomic.LoadInt32(&e.roundFinalTTL); got != 5 {
		t.Errorf("expected roundFinalTTL=5, got %d", got)
	}
}

// TestOnICMP_NonDestinationDoesNotSetFinal 验证非目的地 hop 不设置 roundFinalTTL。
func TestOnICMP_NonDestinationDoesNotSetFinal(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	e.config.DstIP = net.ParseIP("8.8.8.8")
	atomic.StoreUint32(&e.roundID, 1)
	atomic.StoreInt32(&e.roundFinalTTL, -1)

	now := time.Now()
	seq := 42
	e.sentAt[seq] = mtrProbeMeta{ttl: 3, start: now, roundID: 1}

	// 中间 hop，不是目的地
	peer := &net.IPAddr{IP: net.ParseIP("10.0.0.1")}
	e.onICMP(internal.ReceivedMessage{Peer: peer}, now.Add(10*time.Millisecond), seq)

	if got := atomic.LoadInt32(&e.roundFinalTTL); got != -1 {
		t.Errorf("expected roundFinalTTL=-1 for non-destination, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// peekPartialResult 单测
// ---------------------------------------------------------------------------

func TestPeekPartialResult_EmptyBeforeRound(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	// 未初始化 curTtlSeq → 返回 nil
	if got := e.peekPartialResult(); got != nil {
		t.Fatalf("expected nil before round, got %+v", got)
	}
}

func TestPeekPartialResult_PartialReplies(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	atomic.StoreUint32(&e.roundID, 1)
	atomic.StoreInt32(&e.roundFinalTTL, -1)

	// 模拟 probeRound 已初始化 peek 状态
	e.curBeginHop = 1
	e.curEffectiveMax = 3
	e.curTtlSeq = map[int]int{1: 10, 2: 11, 3: 12}

	// TTL 1 已收到响应，TTL 2/3 尚未
	e.replied[10] = &mtrProbeReply{
		peer: &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
		rtt:  5 * time.Millisecond,
	}

	res := e.peekPartialResult()
	if res == nil {
		t.Fatal("expected non-nil partial result")
	}
	if len(res.Hops) != 3 {
		t.Fatalf("expected 3 hop slots, got %d", len(res.Hops))
	}
	// TTL 1: 成功
	if len(res.Hops[0]) != 1 || !res.Hops[0][0].Success {
		t.Error("TTL 1 should be successful")
	}
	// TTL 2: 超时（尚未响应）
	if len(res.Hops[1]) != 1 || res.Hops[1][0].Success {
		t.Error("TTL 2 should be timeout (not replied)")
	}
	// TTL 3: 超时
	if len(res.Hops[2]) != 1 || res.Hops[2][0].Success {
		t.Error("TTL 3 should be timeout")
	}
}

func TestPeekPartialResult_UnsentTTLsAreNil(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	atomic.StoreUint32(&e.roundID, 1)
	atomic.StoreInt32(&e.roundFinalTTL, -1)

	// 模拟发送进行到一半：TTL 1-2 已发送，TTL 3-5 尚未
	e.curBeginHop = 1
	e.curEffectiveMax = 5
	e.curTtlSeq = map[int]int{1: 10, 2: 11} // 3-5 不存在

	// TTL 1 已回复
	e.replied[10] = &mtrProbeReply{
		peer: &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
		rtt:  5 * time.Millisecond,
	}

	res := e.peekPartialResult()
	if res == nil {
		t.Fatal("expected non-nil partial result")
	}
	if len(res.Hops) != 5 {
		t.Fatalf("expected 5 hop slots, got %d", len(res.Hops))
	}

	// TTL 1: 已发送+已回复 → 成功
	if res.Hops[0] == nil || !res.Hops[0][0].Success {
		t.Error("TTL 1 should be successful")
	}
	// TTL 2: 已发送+未回复 → 超时（非 nil）
	if res.Hops[1] == nil || res.Hops[1][0].Success {
		t.Error("TTL 2 should be timeout (sent but not replied)")
	}
	// TTL 3-5: 未发送 → nil（聚合器不计入 Snt/Loss）
	for i := 2; i < 5; i++ {
		if res.Hops[i] != nil {
			t.Errorf("TTL %d should be nil (unsent), got %+v", i+1, res.Hops[i])
		}
	}
}

func TestPeekPartialResult_TrimsByRoundFinalTTL(t *testing.T) {
	e := newTestICMPEngine(2 * time.Second)
	atomic.StoreUint32(&e.roundID, 1)
	atomic.StoreInt32(&e.roundFinalTTL, 2) // 本轮已检测到目的地在 TTL 2

	e.curBeginHop = 1
	e.curEffectiveMax = 5
	e.curTtlSeq = map[int]int{1: 10, 2: 11, 3: 12, 4: 13, 5: 14}

	res := e.peekPartialResult()
	if res == nil {
		t.Fatal("expected non-nil partial result")
	}
	// 应被裁剪到 TTL 2
	if len(res.Hops) != 2 {
		t.Errorf("expected 2 hop slots (trimmed by roundFinalTTL), got %d", len(res.Hops))
	}
}

// ---------------------------------------------------------------------------
// mtrLoop 流式预览测试
// ---------------------------------------------------------------------------

// mockPeekerProber 同时实现 mtrProber + mtrPeeker。
type mockPeekerProber struct {
	mockProber
	peekFn func() *Result
}

func (m *mockPeekerProber) peekPartialResult() *Result {
	if m.peekFn != nil {
		return m.peekFn()
	}
	return nil
}

func TestMTRLoop_StreamingProgress(t *testing.T) {
	// probeRound 耗时 300ms，ProgressThrottle 50ms
	// 在一轮中应产生多次预览 + 1 次最终快照
	partialRes := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 5*time.Millisecond)},
	)

	prober := &mockPeekerProber{
		mockProber: mockProber{
			roundFn: func(ctx context.Context) (*Result, error) {
				select {
				case <-time.After(300 * time.Millisecond):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return partialRes, nil
			},
		},
		peekFn: func() *Result {
			return partialRes
		},
	}
	agg := NewMTRAggregator()

	var snapshotCount int32

	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds:        1,
		Interval:         time.Millisecond,
		ProgressThrottle: 50 * time.Millisecond,
	}, agg, func(_ int, _ []MTRHopStat) {
		atomic.AddInt32(&snapshotCount, 1)
	}, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 300ms / 50ms ≈ 6 次预览 + 1 次最终 ≈ 7，至少应有 2 次
	count := atomic.LoadInt32(&snapshotCount)
	if count < 2 {
		t.Errorf("expected at least 2 snapshots (preview+final), got %d", count)
	}
}

func TestMTRLoop_NonPeekerNoStreaming(t *testing.T) {
	// 普通 mockProber 不实现 mtrPeeker，应正常工作（无预览）
	prober := constantResultProber(mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 5*time.Millisecond)},
	))
	agg := NewMTRAggregator()

	var snapshotCount int32
	err := mtrLoop(context.Background(), prober, Config{}, MTROptions{
		MaxRounds:        3,
		Interval:         time.Millisecond,
		ProgressThrottle: time.Millisecond,
	}, agg, func(_ int, _ []MTRHopStat) {
		atomic.AddInt32(&snapshotCount, 1)
	}, false, fastBackoff)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 非 peeker 模式：每轮仅 1 次快照，共 3 次
	if got := atomic.LoadInt32(&snapshotCount); got != 3 {
		t.Errorf("expected exactly 3 snapshots, got %d", got)
	}
}
