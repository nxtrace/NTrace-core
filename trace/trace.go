package trace

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/util"
	"golang.org/x/sync/singleflight"
)

var (
	ErrInvalidMethod      = errors.New("invalid method")
	ErrTracerouteExecuted = errors.New("traceroute already executed")
	ErrHopLimitTimeout    = errors.New("hop timeout")
	geoCache              = sync.Map{}
	ipGeoSF               singleflight.Group
	rdnsSF                singleflight.Group
)

type Config struct {
	SrcAddr          string
	SrcPort          int
	BeginHop         int
	MaxHops          int
	NumMeasurements  int
	MaxAttempts      int
	ParallelRequests int
	Timeout          time.Duration
	DestIP           net.IP
	DestPort         int
	Quic             bool
	IPGeoSource      ipgeo.Source
	RDns             bool
	AlwaysWaitRDNS   bool
	PacketInterval   int
	TTLInterval      int
	Lang             string
	DN42             bool
	RealtimePrinter  func(res *Result, ttl int)
	AsyncPrinter     func(res *Result)
	PktSize          int
	Maptrace         bool
	DontFragment     bool
}

type Method string

const (
	ICMPTrace Method = "icmp"
	UDPTrace  Method = "udp"
	TCPTrace  Method = "tcp"
)

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
	if config.MaxAttempts <= 0 && util.EnvMaxAttempts != "" {
		if env, err := strconv.Atoi(util.EnvMaxAttempts); err == nil {
			config.MaxAttempts = env
		} else {
			log.Printf("ignore invalid NEXTTRACE_MAXATTEMPTS=%q: %v", util.EnvMaxAttempts, err)
		}
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
		if config.DestIP.To4() != nil {
			tracer = &ICMPTracer{Config: config}
		} else {
			tracer = &ICMPTracerv6{Config: config}
		}

	case UDPTrace:
		if config.DestIP.To4() != nil {
			tracer = &UDPTracer{Config: config}
		} else {
			tracer = &UDPTracerIPv6{Config: config}
		}
	case TCPTrace:
		if config.DestIP.To4() != nil {
			tracer = &TCPTracer{Config: config}
		} else {
			tracer = &TCPTracerIPv6{Config: config}
		}
	default:
		return &Result{}, ErrInvalidMethod
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
	TraceMapUrl string
}

// 判定 Hop 是否“有效”
func isValidHop(h Hop) bool {
	return h.Success && h.Address != nil
}

// 新版 add：带审计/限容
// - N = numMeasurements（每个 TTL 组的最小输出条数）
// - M = maxAttempts（每个 TTL 组的最大尝试条数）
// 规则：前 N-1 条无条件放行；第 N 条进行审计（已有有效 / 当次有效 / 达到最后一次尝试 任一成立即放行）；超过 N 条一律忽略
func (s *Result) add(hop Hop, attemptIdx, numMeasurements, maxAttempts int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	k := hop.TTL - 1
	bucket := s.Hops[k]

	n := numMeasurements

	switch {
	case len(bucket) < n-1:
		// 前 N-1：无条件放行
		s.Hops[k] = append(bucket, hop)
		return
	case len(bucket) == n-1:
		// 正在决定第 N 条：审计
		// 放行条件（三选一）：
		// (1) 前 N-1 中已存在有效值
		// (2) 当前 hop 为有效值
		// (3) 已到最后一次尝试
		hasValid := false
		for _, h := range bucket {
			if isValidHop(h) {
				hasValid = true
				break
			}
		}
		if hasValid || isValidHop(hop) || (attemptIdx+1 >= maxAttempts) {
			s.Hops[k] = append(bucket, hop) // 填满第 N 个
		}
		// 否则丢弃，等待后续更优候选（长度仍保持 N-1）
		return
	default:
		// 已经有 N 条：忽略后续尝试
		return
	}
}

// 旧版 addLegacy
func (s *Result) addLegacy(hop Hop) {
	s.lock.Lock()
	defer s.lock.Unlock()

	k := hop.TTL - 1
	for len(s.Hops) < hop.TTL {
		s.Hops = append(s.Hops, make([]Hop, 0))
	}
	s.Hops[k] = append(s.Hops[k], hop)
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

func (h *Hop) fetchIPData(c Config) (err error) {
	ipStr := h.Address.String()
	// DN42
	if c.DN42 {
		var combined string
		if c.RDns && h.Hostname == "" {
			// singleflight 避免同一 IP 并发重复查询 PTR
			v, _, _ := rdnsSF.Do(ipStr, func() (any, error) {
				return util.LookupAddr(ipStr)
			})
			if ptrs, _ := v.([]string); len(ptrs) > 0 {
				h.Hostname = ptrs[0][:len(ptrs[0])-1]
				combined = ipStr + "," + h.Hostname
			}
		}
		if combined == "" {
			combined = ipStr
		}

		if c.IPGeoSource != nil {
			// 如果缓存中已有结果，直接使用
			if cacheVal, ok := geoCache.Load(combined); ok {
				h.Geo = cacheVal.(*ipgeo.IPGeoData)
				return nil
			}
			// singleflight 合并相同 key 的并发查询
			v, err, _ := ipGeoSF.Do(combined, func() (any, error) {
				return c.IPGeoSource(combined, c.Timeout, c.Lang, c.Maptrace)
			})
			if err != nil {
				return err
			}
			h.Geo = v.(*ipgeo.IPGeoData)
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
			h.Geo = cacheVal.(*ipgeo.IPGeoData)
			ipGeoCh <- nil
			return
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
		h.Geo = v.(*ipgeo.IPGeoData)
		// 如果缓存中无结果，进行查询并将结果存入缓存
		geoCache.Store(ipStr, h.Geo)
		ipGeoCh <- nil
	}()

	rdnsStarted := c.RDns && h.Hostname == ""
	rdnsCh := make(chan []string, 1)
	if rdnsStarted {
		go func() {
			v, _, _ := rdnsSF.Do(ipStr, func() (any, error) {
				return util.LookupAddr(ipStr)
			})
			if ptrs, _ := v.([]string); len(ptrs) > 0 {
				rdnsCh <- ptrs
			} else {
				rdnsCh <- nil
			}
		}()
	}

	if c.AlwaysWaitRDNS {
		// 必须等 PTR（1s 超时），然后再确保 IPGeo 完成
		if rdnsStarted {
			select {
			case ptrs := <-rdnsCh:
				if len(ptrs) > 0 {
					h.Hostname = ptrs[0]
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
	if rdnsStarted {
		select {
		case err := <-ipGeoCh:
			// 地理信息先完成：不再等待 PTR
			return err
		case ptrs := <-rdnsCh:
			if len(ptrs) > 0 {
				h.Hostname = ptrs[0][:len(ptrs[0])-1]
			}
			// 然后等待 IPGeo 完成
			return <-ipGeoCh
		}
	}
	// 未启动 rDNS，只需等待地理信息
	return <-ipGeoCh
}

func extractMPLS(msg ReceivedMessage, data []byte) []string {
	if util.DisableMPLS != "" {
		return nil
	}

	if psize != 52 {
		return nil
	}

	extensionOffset := 20 + 8 + psize

	if len(data) <= extensionOffset {
		return nil
	}

	extensionBody := data[extensionOffset:]
	if len(extensionBody) < 8 || len(extensionBody)%8 != 0 {
		return nil
	}

	tmp := fmt.Sprintf("%x", msg.Msg[:*msg.N])

	index := strings.Index(tmp, strings.Repeat("01", psize-4)+"00004fff")
	if index == -1 {
		return nil
	}
	tmp = tmp[index+psize*2:]
	//由于限制长度了
	index1 := strings.Index(tmp, "00002000")
	l := len(tmp[index1+4:])/8 - 2
	//fmt.Printf("l:%d\n", l)

	if l < 1 {
		return nil
	}
	//去掉扩展头和MPLS头
	tmp = tmp[index1+4+8*2:]
	//fmt.Print(tmp)

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

		//if i > 0 {
		//	retStr += "\n    "
		//}

		retStrList = append(retStrList, fmt.Sprintf("[MPLS: Lbl %d, TC %d, S %s, TTL %d]", label, tc, s, ttlMpls))
	}

	//label, err := strconv.ParseInt(tmp[len(tmp)-8:len(tmp)-3], 16, 32)
	//if err != nil {
	//	return ""
	//}
	//
	//strSlice := fmt.Sprintf("%s", []byte(tmp[len(tmp)-3:len(tmp)-2]))
	////fmt.Printf("\nstrSlice: %s\n", strSlice)
	//
	//num, err := strconv.ParseUint(strSlice, 16, 64)
	//if err != nil {
	//	return ""
	//}
	//binaryStr := fmt.Sprintf("%04s", strconv.FormatUint(num, 2))
	//
	////fmt.Printf("\nbinaryStr: %s\n", binaryStr)
	//tc, err := strconv.ParseInt(binaryStr[:3], 2, 32)
	//if err != nil {
	//	return ""
	//}
	//s := binaryStr[3:]
	//
	//ttlMpls, err := strconv.ParseInt(tmp[len(tmp)-2:], 16, 32)
	//if err != nil {
	//	return ""
	//}
	//
	//retStr := fmt.Sprintf("Lbl %d, TC %d, S %s, TTL %d", label, tc, s, ttlMpls)

	return retStrList
}
