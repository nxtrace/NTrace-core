package reporter

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

type Reporter interface {
	Print()
}

func New(rs *trace.Result, ip string) Reporter {
	experimentTag()
	r := reporter{
		routeResult: rs,
		targetIP:    ip,
	}
	return &r
}

type reporter struct {
	targetTTL       uint16
	targetIP        string
	routeReport     map[uint16][]routeReportNode
	routeReportLock sync.Mutex
	routeResult     *trace.Result
	wg              sync.WaitGroup
}

type routeReportNode struct {
	asn string
	isp string
	geo []string
	ix  bool
}

func experimentTag() {
	fmt.Println("Route-Path 功能实验室")
}

func (r *reporter) generateRouteReportNode(ip string, ipGeoData ipgeo.IPGeoData, ttl uint16) {

	var success = true

	defer r.wg.Done()

	rpn := routeReportNode{}
	ptr, err := net.LookupAddr(ip)

	if err == nil {
		if strings.Contains(strings.ToLower(ptr[0]), "ix") {
			rpn.ix = true
		} else {
			rpn.ix = false
		}
	}
	// TODO: 这种写法不好，后面再重构一下
	// 判断反向解析的域名中又或者是IP地理位置数据库中，是否出现了 IX
	if strings.Contains(strings.ToLower(ipGeoData.Isp), "exchange") || strings.Contains(strings.ToLower(ipGeoData.Isp), "ix") || strings.Contains(strings.ToLower(ipGeoData.Owner), "exchange") || strings.Contains(strings.ToLower(ipGeoData.Owner), "ix") {
		rpn.ix = true
	}

	// TODO: 正则判断POP并且提取带宽大小等信息

	// CN2 需要特殊处理，因为他们很多没有ASN
	// 但是目前这种写法是不规范的，属于凭空标记4809的IP
	// TODO: 用更好的方式显示 CN2 骨干网的路由 Path
	if strings.HasPrefix(ip, "59.43") {
		rpn.asn = "4809"
	} else {
		rpn.asn = ipGeoData.Asnumber
	}

	// 无论最后一跳是否为存在地理位置信息（AnyCast），都应该给予显示
	if (ipGeoData.Country == "" || ipGeoData.Country == "LAN Address" || ipGeoData.Country == "-") && ip != r.targetIP {
		success = false
	} else {
		if ipGeoData.City == "" {
			rpn.geo = []string{ipGeoData.Country, ipGeoData.Prov}
		} else {
			rpn.geo = []string{ipGeoData.Country, ipGeoData.City}
		}
	}
	if ipGeoData.Asnumber == "" {
		rpn.asn = "*"
	}

	if ipGeoData.Isp == "" {
		rpn.isp = ipGeoData.Owner
	} else {
		rpn.isp = ipGeoData.Isp
	}

	// 有效记录
	if success {
		// 锁住资源，防止同时写panic
		r.routeReportLock.Lock()
		// 添加到MAP中
		r.routeReport[ttl] = append(r.routeReport[ttl], rpn)
		// 写入完成，解锁释放资源给其他协程
		r.routeReportLock.Unlock()
	}
}

func (r *reporter) InitialBaseData() Reporter {
	reportNodes := map[uint16][]routeReportNode{}

	r.routeReport = reportNodes
	r.targetTTL = uint16(len(r.routeResult.Hops))

	for i := uint16(0); i < r.targetTTL; i++ {
		if i < uint16(len(r.routeResult.Hops)) && len(r.routeResult.Hops[i]) > 0 {
			traceHop := r.routeResult.Hops[i][0]
			if traceHop.Success {
				currentIP := traceHop.Address.String()
				r.wg.Add(1)
				go r.generateRouteReportNode(currentIP, *traceHop.Geo, i)
			}
		}
	}

	// 等待所有的子协程运行完毕
	r.wg.Wait()
	return r
}

func (r *reporter) Print() {
	var beforeActiveTTL uint16 = 0
	r.InitialBaseData()
	// 尝试首个有效 TTL
	for i := uint16(0); i < r.targetTTL; i++ {
		if len(r.routeReport[i]) != 0 {
			beforeActiveTTL = i
			// 找到以后便不再循环
			break
		}
	}

	for i := beforeActiveTTL; i < r.targetTTL; i++ {
		// 计算该TTL内的数据长度，如果为0，则代表没有有效数据
		if len(r.routeReport[i]) == 0 {
			// 跳过改跃点的数据整理
			continue
		}
		nodeReport := r.routeReport[i][0]

		if i == beforeActiveTTL {
			fmt.Printf("AS%s %s「%s『%s", nodeReport.asn, nodeReport.isp, nodeReport.geo[0], nodeReport.geo[1])
		} else {
			nodeReportBefore := r.routeReport[beforeActiveTTL][0]
			// ASN 相同，同个 ISP 内部的数据传递
			if nodeReportBefore.asn == nodeReport.asn {
				// Same ASN but Coutry or City Changed
				if nodeReportBefore.geo[0] != nodeReport.geo[0] {
					fmt.Printf("』→ %s『%s", nodeReport.geo[0], nodeReport.geo[1])
				} else {
					if nodeReportBefore.geo[1] != nodeReport.geo[1] {
						fmt.Printf(" → %s", nodeReport.geo[1])
					}
				}
			} else {
				// ASN 不同，跨 ISP 的数据传递，这里可能会出现 POP、IP Transit、Peer、Exchange
				fmt.Printf("』」")
				if int(i) != len(r.routeReport)+1 {
					// 部分 Shell 客户端可能无法很好的展示这个特殊字符
					// TODO: 寻找其他替代字符
					fmt.Printf("\n ╭╯\n ╰")
				}
				if nodeReport.ix {
					fmt.Printf("AS%s \033[42;37mIXP\033[0m %s「%s『%s", nodeReport.asn, nodeReport.isp, nodeReport.geo[0], nodeReport.geo[1])
				} else {
					fmt.Printf("AS%s %s「%s『%s", nodeReport.asn, nodeReport.isp, nodeReport.geo[0], nodeReport.geo[1])
				}
			}
		}
		// 标记为最新的一个有效跃点
		beforeActiveTTL = i
	}
	fmt.Println("』」")
}
