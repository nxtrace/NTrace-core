package trace

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/util"
)

var (
	ErrInvalidMethod      = errors.New("invalid method")
	ErrTracerouteExecuted = errors.New("traceroute already executed")
	ErrHopLimitTimeout    = errors.New("hop timeout")
	geoCache              = sync.Map{}
)

type Config struct {
	SrcAddr          string
	BeginHop         int
	MaxHops          int
	NumMeasurements  int
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
			//TODO: IPv6 UDP trace 在做了，指新建文件夹（
			return nil, errors.New("IPv6 UDP Traceroute is not supported")
		}
	case TCPTrace:
		if config.DestIP.To4() != nil {
			tracer = &TCPTracer{Config: config}
		} else {
			tracer = &TCPTracerv6{Config: config}
			// return nil, errors.New("IPv6 TCP Traceroute is not supported")
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
	lock        sync.Mutex
	TraceMapUrl string
}

func (s *Result) add(hop Hop) {
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

	// DN42
	if c.DN42 {
		var ip string
		// 首先查找 PTR 记录
		r, err := util.LookupAddr(h.Address.String())
		if err == nil && len(r) > 0 {
			h.Hostname = r[0][:len(r[0])-1]
			ip = h.Address.String() + "," + h.Hostname
		}
		h.Geo, _ = c.IPGeoSource(ip, c.Timeout, c.Lang, c.Maptrace)
		return nil
	}

	// Debug Area
	// c.AlwaysWaitRDNS = true

	// Initialize a rDNS Channel
	rDNSChan := make(chan []string)
	fetchDoneChan := make(chan bool)

	if c.RDns && h.Hostname == "" {
		// Create a rDNS Query go-routine
		go func() {
			r, err := util.LookupAddr(h.Address.String())
			if err != nil {
				// No PTR Record
				rDNSChan <- nil
			} else {
				// One PTR Record is found
				rDNSChan <- r
			}
		}()
	}

	// Create Data Fetcher go-routine
	go func() {
		// Start to fetch IP Geolocation data
		if c.IPGeoSource != nil && h.Geo == nil {
			res := false
			h.Lang = c.Lang
			h.Geo, res = ipgeo.Filter(h.Address.String())
			if !res {
				timeout := c.Timeout
				if c.Timeout < 2*time.Second {
					timeout = 2 * time.Second
				}
				//h.Geo, err = c.IPGeoSource(h.Address.String(), timeout, c.Lang, c.Maptrace)
				if cacheVal, ok := geoCache.Load(h.Address.String()); ok {
					// 如果缓存中已有结果，直接使用
					h.Geo = cacheVal.(*ipgeo.IPGeoData)
				} else {
					// 如果缓存中无结果，进行查询并将结果存入缓存
					h.Geo, err = c.IPGeoSource(h.Address.String(), timeout, c.Lang, c.Maptrace)
					if err == nil {
						geoCache.Store(h.Address.String(), h.Geo)
					}
				}
			}
		}
		// Fetch Done
		fetchDoneChan <- true
	}()

	// Select Close Flag
	var selectClose bool
	if !c.AlwaysWaitRDNS {
		select {
		case <-fetchDoneChan:
			// When fetch done signal received, stop waiting PTR record
		case ptr := <-rDNSChan:
			// process result
			if err == nil && len(ptr) > 0 {
				h.Hostname = ptr[0][:len(ptr[0])-1]
			}
			selectClose = true
		}
	} else {
		select {
		case ptr := <-rDNSChan:
			// process result
			if err == nil && len(ptr) > 0 {
				h.Hostname = ptr[0]
			}
		// 1 second timeout
		case <-time.After(time.Second * 1):
		}
		selectClose = true
	}

	// When Select Close, fetchDoneChan Received will also be closed
	if selectClose {
		// New a receiver to prevent channel congestion
		<-fetchDoneChan
	}

	return
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
