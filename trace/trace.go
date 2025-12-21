package trace

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/sync/singleflight"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace/internal"
	"github.com/nxtrace/NTrace-core/util"
)

var (
	errHopLimitTimeout    = errors.New("hop timeout")
	errInvalidMethod      = errors.New("invalid method")
	errNaturalDone        = errors.New("trace natural done")
	errTracerouteExecuted = errors.New("traceroute already executed")
	geoCache              = sync.Map{}
	ipGeoSF               singleflight.Group
	rDNSSF                singleflight.Group
)

type Config struct {
	OSType           int
	ICMPMode         int
	SrcAddr          string
	SrcPort          int
	BeginHop         int
	MaxHops          int
	NumMeasurements  int
	MaxAttempts      int
	ParallelRequests int
	Timeout          time.Duration
	DstIP            net.IP
	DstPort          int
	Quic             bool
	IPGeoSource      ipgeo.Source
	RDNS             bool
	AlwaysWaitRDNS   bool
	PacketInterval   int
	TTLInterval      int
	Lang             string
	DN42             bool
	RealtimePrinter  func(res *Result, ttl int)
	AsyncPrinter     func(res *Result)
	PktSize          int
	Maptrace         bool
}

type Method string

const (
	ICMPTrace Method = "icmp"
	UDPTrace  Method = "udp"
	TCPTrace  Method = "tcp"
)

type attemptKey struct {
	ttl int
	i   int
}

type attemptPort struct {
	srcPort int
	i       int
}

type sentInfo struct {
	ttl     int
	i       int
	srcPort int
	start   time.Time
}

type matchTask struct {
	srcPort int
	seq     int
	peer    net.Addr
	finish  time.Time
	mpls    []string
}

type Tracer interface {
	Execute() (*Result, error)
}

func Traceroute(method Method, config Config) (*Result, error) {
	var tracer Tracer

	if config.MaxHops == 0 {
		config.MaxHops = 30
	}

	if config.NumMeasurements == 0 {
		config.NumMeasurements = 3
	}

	if config.ParallelRequests == 0 {
		config.ParallelRequests = config.NumMeasurements * 5
	}

	// 若 CLI 未给或给了非正数，则尝试用环境变量
	if config.ICMPMode <= 0 && util.EnvICMPMode > 0 {
		config.ICMPMode = util.EnvICMPMode
	}

	switch config.ICMPMode {
	case 0, 1, 2:
		// 合法，保持不变
	default:
		// 非法输入一律回退到 Auto
		config.ICMPMode = 0
	}

	// 若 CLI 未给或给了非正数，则尝试用环境变量
	if config.MaxAttempts <= 0 && util.EnvMaxAttempts > 0 {
		config.MaxAttempts = util.EnvMaxAttempts
	}

	if config.MaxAttempts <= 0 || config.MaxAttempts < config.NumMeasurements {
		n := config.NumMeasurements
		switch {
		case n <= 2 || n >= 10:
			config.MaxAttempts = n // 1–2 或 ≥10 → n
		case n <= 6:
			config.MaxAttempts = n + 3 // 3–6 → n+3
		default:
			config.MaxAttempts = 10 // 7–9 → 10
		}
	}

	switch method {
	case ICMPTrace:
		if config.DstIP.To4() != nil {
			tracer = &ICMPTracer{Config: config}
		} else {
			tracer = &ICMPTracerv6{Config: config}
		}
	case UDPTrace:
		if config.DstIP.To4() != nil {
			tracer = &UDPTracer{Config: config}
		} else {
			tracer = &UDPTracerIPv6{Config: config}
		}
	case TCPTrace:
		if config.DstIP.To4() != nil {
			tracer = &TCPTracer{Config: config}
		} else {
			tracer = &TCPTracerIPv6{Config: config}
		}
	default:
		return &Result{}, errInvalidMethod
	}

	result, err := tracer.Execute()
	if err != nil && errors.Is(err, syscall.EPERM) {
		err = fmt.Errorf("%w, please run as root", err)
	}
	if result != nil {
		// 等待所有异步 Geo 查询完成，最多等 30 秒
		done := make(chan struct{})
		go func() {
			result.geoWG.Wait()
			close(done)
		}()
		select {
		case <-done:
			// 正常完成
		case <-time.After(30 * time.Second):
			// 超时，不再等待，直接返回当前结果
		}
	}
	return result, err
}

type Result struct {
	Hops        [][]Hop
	lock        sync.RWMutex
	tailDone    []bool
	TraceMapUrl string
	geoWait     time.Duration
	geoWG       sync.WaitGroup
}

const PendingGeoSource = "pending"
const timeoutGeoSource = "timeout"

func isPendingGeo(geo *ipgeo.IPGeoData) bool {
	return geo != nil && geo.Source == PendingGeoSource
}

func pendingGeo() *ipgeo.IPGeoData {
	return &ipgeo.IPGeoData{Source: PendingGeoSource}
}

func timeoutGeo() *ipgeo.IPGeoData {
	return &ipgeo.IPGeoData{
		Country:   "网络故障",
		CountryEn: "Network Error",
		Source:    timeoutGeoSource,
	}
}

func geoWaitForMeasurements(numMeasurements int) time.Duration {
	if numMeasurements <= 0 {
		numMeasurements = 1
	}
	maxRetries := numMeasurements - 1
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 5 {
		maxRetries = 5
	}

	total := 0
	for attempt := 0; attempt <= maxRetries; attempt++ {
		timeout := 2 + attempt
		if timeout > 6 {
			timeout = 6
		}
		total += timeout
	}
	return time.Duration(total) * time.Second
}

// 判定 Hop 是否“有效”
func isValidHop(h Hop) bool {
	return h.Success && h.Address != nil
}

// add 带审计/限容
// - N = numMeasurements（每个 TTL 组的最小输出条数）
// - M = maxAttempts（每个 TTL 组的最大尝试条数）
// 规则：对同一 TTL，attemptIdx < N-1 无条件放行（索引 i 从 0 开始）；第 N 条进行审计（已有有效 / 当次有效 / 达到最后一次尝试 任一成立即放行）；超过 N 条一律忽略
func (s *Result) add(hop Hop, attemptIdx, numMeasurements, maxAttempts int) (bool, int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	k := hop.TTL - 1
	bucket := s.Hops[k]
	n := numMeasurements

	switch {
	case attemptIdx < n-1:
		// attemptIdx < N-1：无条件放行
		s.Hops[k] = append(bucket, hop)
		return true, len(s.Hops[k]) - 1
	case attemptIdx >= n-1:
		// 正在决定第 N 条：审计
		// 放行条件（三选一）：
		// (1) 前 N-1 中已存在有效值
		// (2) 当前 hop 为有效值
		// (3) 已到最后一次尝试
		if len(bucket) >= n {
			return false, -1
		}

		if s.tailDone[k] {
			return false, -1
		}

		hasValid := false
		for _, h := range bucket {
			if isValidHop(h) {
				hasValid = true
				break
			}
		}
		if hasValid || isValidHop(hop) || attemptIdx >= maxAttempts-1 {
			s.Hops[k] = append(bucket, hop) // 填满第 N 个
			s.tailDone[k] = true
			return true, len(s.Hops[k]) - 1
		}
		// 否则丢弃，等待后续更优候选（长度仍保持 N-1）
		return false, -1
	}
	return false, -1
}

func (s *Result) setGeoWait(numMeasurements int) {
	s.geoWait = geoWaitForMeasurements(numMeasurements)
}

func (s *Result) reduce(final int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if final > 0 && final < len(s.Hops) {
		s.Hops = s.Hops[:final]
	}
}

type Hop struct {
	Success  bool
	Address  net.Addr
	Hostname string
	TTL      int
	RTT      time.Duration
	Error    error
	Geo      *ipgeo.IPGeoData
	Lang     string
	MPLS     []string
}

func isLDHASCII(label string) bool {
	for i := 0; i < len(label); i++ {
		b := label[i]
		if b > 0x7F {
			return false
		}
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' {
			continue
		}
		return false
	}
	return true
}

func CanonicalHostname(s string) string {
	if s == "" {
		return ""
	}
	// 去掉尾点
	if strings.HasSuffix(s, ".") {
		s = strings.TrimSuffix(s, ".")
	}
	// 按标签逐个处理，确保仅对需要的标签做 IDNA 转换
	parts := strings.Split(s, ".")
	for i, label := range parts {
		if label == "" {
			continue
		}
		if isLDHASCII(label) {
			// 纯 ASCII 且仅含 LDH：保留原大小写，不做大小写折叠
			continue
		}
		// 含非 ASCII 或不满足 LDH：对该标签做 IDNA 转 ASCII
		if ascii, err := idna.Lookup.ToASCII(label); err == nil && ascii != "" {
			parts[i] = ascii
		}
		// 若转换失败，保留原标签
	}
	return strings.Join(parts, ".")
}

func (s *Result) updateHop(ttl, idx int, updated Hop) {
	s.lock.Lock()
	defer s.lock.Unlock()

	k := ttl - 1
	if k < 0 || k >= len(s.Hops) {
		return
	}
	if idx < 0 || idx >= len(s.Hops[k]) {
		return
	}

	h := &s.Hops[k][idx]
	if updated.Hostname != "" {
		h.Hostname = updated.Hostname
	}
	if updated.Geo != nil {
		h.Geo = updated.Geo
	}
	if updated.Lang != "" {
		h.Lang = updated.Lang
	}
}

func (s *Result) waitGeo(ctx context.Context, ttlIdx int) {
	if s.geoWait <= 0 {
		return
	}
	if ttlIdx < 0 {
		return
	}

	deadline := time.Now().Add(s.geoWait)
	for {
		if !s.hasPendingGeo(ttlIdx) {
			return
		}
		if time.Now().After(deadline) {
			s.markPendingGeoTimeout(ttlIdx)
			return
		}
		if ctx == nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (s *Result) hasPendingGeo(ttlIdx int) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if ttlIdx < 0 || ttlIdx >= len(s.Hops) {
		return false
	}

	for _, hop := range s.Hops[ttlIdx] {
		if hop.Address == nil {
			continue
		}
		if isPendingGeo(hop.Geo) {
			return true
		}
	}
	return false
}

func (s *Result) markPendingGeoTimeout(ttlIdx int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if ttlIdx < 0 || ttlIdx >= len(s.Hops) {
		return
	}

	for i := range s.Hops[ttlIdx] {
		hop := &s.Hops[ttlIdx][i]
		if hop.Address == nil || !isPendingGeo(hop.Geo) {
			continue
		}
		hop.Geo = timeoutGeo()
	}
}

func (s *Result) addWithGeoAsync(hop Hop, attemptIdx, numMeasurements, maxAttempts int, cfg Config) {
	if hop.Geo == nil {
		hop.Geo = pendingGeo()
	} else if hop.Geo.Source == "" {
		hop.Geo.Source = PendingGeoSource
	}
	if hop.Lang == "" {
		hop.Lang = cfg.Lang
	}

	added, idx := s.add(hop, attemptIdx, numMeasurements, maxAttempts)
	if !added {
		return
	}

	s.geoWG.Add(1)
	go func(ttl, idx int, h Hop) {
		defer s.geoWG.Done()
		_ = h.fetchIPData(cfg)
		s.updateHop(ttl, idx, h)
	}(hop.TTL, idx, hop)
}

func (h *Hop) fetchIPData(c Config) error {
	ipStr := h.Address.String()
	// DN42
	if c.DN42 {
		var combined string
		if c.RDNS && h.Hostname == "" {
			// singleflight 避免同一 IP 并发重复查询 PTR
			v, _, _ := rDNSSF.Do(ipStr, func() (any, error) {
				return util.LookupAddr(ipStr)
			})
			if ptrs, _ := v.([]string); len(ptrs) > 0 {
				h.Hostname = CanonicalHostname(ptrs[0])
				combined = ipStr + "," + h.Hostname
			}
		}
		if combined == "" {
			combined = ipStr
		}

		if c.IPGeoSource != nil {
			// 如果缓存中已有结果，直接使用
			if cacheVal, ok := geoCache.Load(combined); ok {
				if g, ok := cacheVal.(*ipgeo.IPGeoData); ok && g != nil {
					h.Geo = g
					return nil
				}
			}
			// singleflight 合并相同 key 的并发查询
			maxRetries := c.NumMeasurements - 1
			if maxRetries < 0 {
				maxRetries = 0
			}
			if maxRetries > 5 {
				maxRetries = 5
			}

			var lastErr error
			for attempt := 0; attempt <= maxRetries; attempt++ {
				// 超时：2s 起，每次 +1s，上限 6s
				timeout := time.Duration(2+attempt) * time.Second
				if timeout > 6*time.Second {
					timeout = 6 * time.Second
				}

				v, err, _ := ipGeoSF.Do(combined, func() (any, error) {
					return c.IPGeoSource(combined, timeout, c.Lang, c.Maptrace)
				})
				if err != nil {
					lastErr = err
					continue
				}

				geo, ok := v.(*ipgeo.IPGeoData)
				if !ok || geo == nil {
					lastErr = errors.New("ipgeo: nil or bad type from singleflight (DN42)")
					continue
				}

				// 成功：写入结果与缓存，结束
				h.Geo = geo
				geoCache.Store(combined, h.Geo)
				return nil
			}
			// 所有尝试均失败
			if lastErr == nil {
				lastErr = errors.New("ipgeo: lookup failed without specific error (DN42)")
			}
			h.Geo = timeoutGeo()
			return lastErr
		}
		return nil
	}
	// 地理信息查询：快速路径 -> 缓存 -> singleflight
	ipGeoCh := make(chan error, 1)
	go func() {
		if c.IPGeoSource == nil || (h.Geo != nil && !isPendingGeo(h.Geo)) {
			ipGeoCh <- nil
			return
		}
		h.Lang = c.Lang
		// (1) 本地快速路径
		if g, ok := ipgeo.Filter(ipStr); ok {
			h.Geo = g
			ipGeoCh <- nil
			return
		}
		// (2) 如果缓存中已有结果，直接使用
		if cacheVal, ok := geoCache.Load(ipStr); ok {
			if g, ok := cacheVal.(*ipgeo.IPGeoData); ok && g != nil {
				h.Geo = g
				ipGeoCh <- nil
				return
			}
		}
		// (3) singleflight 去重
		maxRetries := c.NumMeasurements - 1
		if maxRetries < 0 {
			maxRetries = 0
		}
		if maxRetries > 5 {
			maxRetries = 5
		}

		var lastErr error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			// 超时：2s 起，每次 +1s，上限 6s
			timeout := time.Duration(2+attempt) * time.Second
			if timeout > 6*time.Second {
				timeout = 6 * time.Second
			}

			v, err, _ := ipGeoSF.Do(ipStr, func() (any, error) {
				return c.IPGeoSource(ipStr, timeout, c.Lang, c.Maptrace)
			})
			if err != nil {
				lastErr = err
				continue
			}

			geo, ok := v.(*ipgeo.IPGeoData)
			if !ok || geo == nil {
				lastErr = errors.New("ipgeo: nil or bad type from singleflight")
				continue
			}

			// 成功：写入结果与缓存，结束
			h.Geo = geo
			geoCache.Store(ipStr, h.Geo)
			ipGeoCh <- nil
			return
		}
		// 所有尝试均失败
		if lastErr == nil {
			lastErr = errors.New("ipgeo: lookup failed without specific error")
		}
		ipGeoCh <- lastErr
	}()

	rDNSStarted := c.RDNS && h.Hostname == ""
	rDNSCh := make(chan []string, 1)
	if rDNSStarted {
		go func() {
			v, _, _ := rDNSSF.Do(ipStr, func() (any, error) {
				return util.LookupAddr(ipStr)
			})
			var ptrs []string
			if p, _ := v.([]string); len(p) > 0 {
				ptrs = p
			}
			// 非阻塞发送：没人收或通道已满就丢弃，保证不阻塞
			select {
			case rDNSCh <- ptrs:
			default:
			}
		}()
	}

	if c.AlwaysWaitRDNS {
		// 必须等 PTR（1s 超时），然后再确保 IPGeo 完成
		if rDNSStarted {
			select {
			case ptrs := <-rDNSCh:
				if len(ptrs) > 0 {
					h.Hostname = CanonicalHostname(ptrs[0])
				}
			case <-time.After(1 * time.Second):
				// 超时不阻塞
			}
		}
		err := <-ipGeoCh
		if err != nil {
			h.Geo = timeoutGeo()
		}
		return err
	}
	// 非强制等待 PTR：依据率先完成者决定是否还等 PTR
	if rDNSStarted {
		select {
		case err := <-ipGeoCh:
			// 地理信息先完成：不再等待 PTR
			if err != nil {
				h.Geo = timeoutGeo()
			}
			return err
		case ptrs := <-rDNSCh:
			if len(ptrs) > 0 {
				h.Hostname = CanonicalHostname(ptrs[0])
			}
			// 然后等待 IPGeo 完成
			err := <-ipGeoCh
			if err != nil {
				h.Geo = timeoutGeo()
			}
			return err
		}
	}
	// 未启动 rDNS，只需等待地理信息
	err := <-ipGeoCh
	if err != nil {
		h.Geo = timeoutGeo()
	}
	return err
}

// parse 安全解析十六进制子串 s 为无符号整数
func parse(s string, bitSize int) (uint64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 16, bitSize)
	if err != nil {
		return 0, false
	}
	return v, true
}

// findValid 在十六进制字符串 hexStr 中截取从 ICMP 扩展头开始的部分
func findValid(hexStr string) string {
	n := len(hexStr)
	// 至少要能容纳 4B 扩展头，且 hexStr 的长度必须为偶数
	if n < 8 || n%2 != 0 {
		return ""
	}

	// 从尾到头以 4B 为单位扫描（1B = 2 hex digits）
	for i := n - 8; i >= 0; i -= 8 {
		// 直接匹配 "2000"
		if hexStr[i:i+4] != "2000" {
			continue
		}

		// 处理扩展头 4B 后的剩余部分
		remHex := n - (i + 8) // 剩余的 hex 个数
		if remHex <= 0 {
			continue
		}

		remBytes := remHex / 2
		// 剩余部分长度必须 ≥ 8B
		if remBytes >= 8 {
			return hexStr[i:]
		}

		// 否则继续向左寻找更早的 "2000"
	}
	return ""
}

func extractMPLS(msg internal.ReceivedMessage) []string {
	if util.DisableMPLS {
		return nil
	}

	// 将整包转换为十六进制字符串
	hexStr := fmt.Sprintf("%x", msg.Msg)

	// 调用 findValid 截取从 ICMP 扩展头开始的字符串
	extStr := findValid(hexStr)
	if extStr == "" {
		return nil
	}

	var mplsLSEList []string
	n := len(extStr)

	// 先逐对象检查 Class 是否为 MPLS Label Stack Class (1)
	for j := 8; j+8 <= n; {
		// 对象头：Length(2B) | Class(1B) | C-Type(1B)
		lengthU, ok := parse(extStr[j:j+4], 16)
		if !ok || lengthU < 4 {
			return nil
		}
		objLenBytes := int(lengthU)
		objLenHex := objLenBytes * 2
		if j+objLenHex > n {
			return nil
		}

		// 读取 Class 的值
		classU, ok := parse(extStr[j+4:j+6], 8)
		if !ok {
			return nil
		}
		class := int(classU)

		if class == 1 {
			// 去掉扩展头与 MPLS 对象头，只保留 MPLS 对象负载
			payloadStart := j + 8
			payloadEnd := j + objLenHex
			if payloadEnd <= payloadStart {
				return nil
			}
			mplsPayload := extStr[payloadStart:payloadEnd] // 仅 LSE 区域
			if len(mplsPayload)%8 != 0 {                   // 每个 LSE = 4B = 8 hex digits
				return nil
			}

			// 逐个 LSE 解析并追加到 mplsLSEList
			for off := 0; off+8 <= len(mplsPayload); off += 8 {
				vU, ok := parse(mplsPayload[off:off+8], 32)
				if !ok {
					return nil
				}
				v := uint32(vU)

				lbl := (v >> 12) & 0xFFFFF // 20 bits
				tc := (v >> 9) & 0x7       // 3 bits
				s := (v >> 8) & 0x1        // 1 bit
				ttl := v & 0xFF            // 8 bits

				mplsLSEList = append(mplsLSEList, fmt.Sprintf("[MPLS: Lbl %d, TC %d, S %d, TTL %d]", lbl, tc, s, ttl))
			}
		}

		// 跳到下一个对象
		j += objLenHex
	}
	return mplsLSEList
}
