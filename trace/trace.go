package trace

import (
	"encoding/hex"
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
	return result, err
}

type Result struct {
	Hops        [][]Hop
	lock        sync.RWMutex
	tailDone    []bool
	TraceMapUrl string
}

// 判定 Hop 是否“有效”
func isValidHop(h Hop) bool {
	return h.Success && h.Address != nil
}

// add 带审计/限容
// - N = numMeasurements（每个 TTL 组的最小输出条数）
// - M = maxAttempts（每个 TTL 组的最大尝试条数）
// 规则：对同一 TTL，attemptIdx < N-1 无条件放行（索引 i 从 0 开始）；第 N 条进行审计（已有有效 / 当次有效 / 达到最后一次尝试 任一成立即放行）；超过 N 条一律忽略
func (s *Result) add(hop Hop, attemptIdx, numMeasurements, maxAttempts int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	k := hop.TTL - 1
	bucket := s.Hops[k]
	n := numMeasurements

	switch {
	case attemptIdx < n-1:
		// attemptIdx < N-1：无条件放行
		s.Hops[k] = append(bucket, hop)
		return
	case attemptIdx >= n-1:
		// 正在决定第 N 条：审计
		// 放行条件（三选一）：
		// (1) 前 N-1 中已存在有效值
		// (2) 当前 hop 为有效值
		// (3) 已到最后一次尝试
		if len(bucket) >= n {
			return
		}

		if s.tailDone[k] {
			return
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
		}
		// 否则丢弃，等待后续更优候选（长度仍保持 N-1）
		return
	}
}

func (s *Result) reduce(final int) {
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
			v, err, _ := ipGeoSF.Do(combined, func() (any, error) {
				timeout := c.Timeout
				if timeout < 2*time.Second {
					timeout = 2 * time.Second
				}
				return c.IPGeoSource(combined, timeout, c.Lang, c.Maptrace)
			})
			if err != nil {
				return err
			}
			geo, ok := v.(*ipgeo.IPGeoData)
			if !ok || geo == nil {
				return errors.New("ipgeo: nil or bad type from singleflight (DN42)")
			}
			h.Geo = geo
			// 如果缓存中无结果，进行查询并将结果存入缓存
			geoCache.Store(combined, h.Geo)
		}
		return nil
	}
	// 地理信息查询：快速路径 -> 缓存 -> singleflight
	ipGeoCh := make(chan error, 1)
	go func() {
		if c.IPGeoSource == nil || h.Geo != nil {
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
		timeout := c.Timeout
		if timeout < 2*time.Second {
			timeout = 2 * time.Second
		}
		v, err, _ := ipGeoSF.Do(ipStr, func() (any, error) {
			return c.IPGeoSource(ipStr, timeout, c.Lang, c.Maptrace)
		})
		if err != nil {
			ipGeoCh <- err
			return
		}
		geo, ok := v.(*ipgeo.IPGeoData)
		if !ok || geo == nil {
			ipGeoCh <- errors.New("ipgeo: nil or bad type from singleflight")
			return
		}
		h.Geo = geo
		geoCache.Store(ipStr, h.Geo)
		ipGeoCh <- nil
	}()

	rDNSStarted := c.RDNS && h.Hostname == ""
	rDNSCh := make(chan []string, 1)
	if rDNSStarted {
		go func() {
			v, _, _ := rDNSSF.Do(ipStr, func() (any, error) {
				return util.LookupAddr(ipStr)
			})
			if ptrs, _ := v.([]string); len(ptrs) > 0 {
				rDNSCh <- ptrs
			} else {
				rDNSCh <- nil
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
		if err := <-ipGeoCh; err != nil {
			return err
		}
		return nil
	}
	// 非强制等待 PTR：依据率先完成者决定是否还等 PTR
	if rDNSStarted {
		select {
		case err := <-ipGeoCh:
			// 地理信息先完成：不再等待 PTR
			return err
		case ptrs := <-rDNSCh:
			if len(ptrs) > 0 {
				h.Hostname = CanonicalHostname(ptrs[0])
			}
			// 然后等待 IPGeo 完成
			return <-ipGeoCh
		}
	}
	// 未启动 rDNS，只需等待地理信息
	return <-ipGeoCh
}

func extractMPLS(msg internal.ReceivedMessage) []string {
	if util.DisableMPLS {
		return nil
	}

	tmp := fmt.Sprintf("%x", msg.Msg)

	var findValid func(tmp, sub string, from int) string
	findValid = func(tmp, sub string, from int) string {
		index := strings.LastIndex(tmp[:from], sub)
		if index == -1 {
			return ""
		}
		if len(tmp[index:])%8 == 0 || len(tmp[index:])/8 < 3 {
			return tmp[index:]
		}
		return findValid(tmp, sub, index)
	}
	tmp = findValid(tmp, "2000", len(tmp))
	if tmp == "" {
		return nil
	}
	xdata, err := hex.DecodeString(tmp)
	if err != nil {
		return nil
	}
	xdata[2], xdata[3] = 0, 0
	// 判断此处这个ICMP Multi-Part Extensions的CLass为MPLS Label Stack Class (1)
	if tmp[12:14] != "01" {
		//fmt.Println("CLASS:", tmp[12:14], ",不是MPLS标签栈类")
		return nil
	}
	l := len(tmp)/8 - 2
	//去掉扩展头和MPLS头
	tmp = tmp[8*2:]

	var retStrList []string
	for i := 0; i < l; i++ {
		label, err := strconv.ParseInt(tmp[i*8+0:i*8+5], 16, 32)
		if err != nil {
			return nil
		}

		strSlice := fmt.Sprintf("%s", []byte(tmp[i*8+5:i*8+6]))
		//fmt.Printf("\nstrSlice: %s\n", strSlice)

		num, err := strconv.ParseUint(strSlice, 16, 64)
		if err != nil {
			return nil
		}
		binaryStr := fmt.Sprintf("%04s", strconv.FormatUint(num, 2))

		//fmt.Printf("\nbinaryStr: %s\n", binaryStr)
		tc, err := strconv.ParseInt(binaryStr[:3], 2, 32)
		if err != nil {
			return nil
		}
		s := binaryStr[3:]

		ttlMpls, err := strconv.ParseInt(tmp[i*8+6:i*8+8], 16, 32)
		if err != nil {
			return nil
		}

		retStrList = append(retStrList, fmt.Sprintf("[MPLS: Lbl %d, TC %d, S %s, TTL %d]", label, tc, s, ttlMpls))
	}

	return retStrList
}
