package reporter

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/trace"
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
	targetIP    string
	routeReport map[uint16][]routeReportNode
	routeResult *trace.Result
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

func (r *reporter) generateRouteReportNode(ip string, ipGeoData ipgeo.IPGeoData) (routeReportNode, error) {
	rpn := routeReportNode{}
	ptr, err := net.LookupAddr(ip)
	if err == nil {
		if strings.Contains(strings.ToLower(ptr[0]), "ix") {
			rpn.ix = true
		} else {
			rpn.ix = false
		}
	}

	if strings.Contains(strings.ToLower(ipGeoData.Isp), "exchange") || strings.Contains(strings.ToLower(ipGeoData.Isp), "ix") || strings.Contains(strings.ToLower(ipGeoData.Owner), "exchange") || strings.Contains(strings.ToLower(ipGeoData.Owner), "ix") {
		rpn.ix = true
	}
	if strings.HasPrefix(ip, "59.43") {
		rpn.asn = "4809"
	} else {
		rpn.asn = ipGeoData.Asnumber
	}
	// 无论最后一跳是否为存在地理位置信息（AnyCast），都应该给予显示
	if ipGeoData.Country == "" || ipGeoData.City == "" && ip != r.targetIP {
		return rpn, errors.New("GeoData Search Failed")
	} else {
		if ipGeoData.City == "" {
			rpn.geo = []string{ipGeoData.Country, ipGeoData.Country}
		} else {
			rpn.geo = []string{ipGeoData.Country, ipGeoData.City}
		}
	}
	if ipGeoData.Isp == "" {
		rpn.isp = ipGeoData.Owner
	} else {
		rpn.isp = ipGeoData.Isp
	}
	return rpn, nil
}

func (r *reporter) InitialBaseData() Reporter {
	var nodeIndex uint16 = 1
	reportNodes := map[uint16][]routeReportNode{}
	for i := uint16(0); int(i) < len(r.routeResult.Hops); i++ {
		traceHop := r.routeResult.Hops[i][0]
		if traceHop.Success {
			currentIP := traceHop.Address.String()
			rpn, err := r.generateRouteReportNode(currentIP, *traceHop.Geo)
			if err == nil {
				reportNodes[nodeIndex] = append(reportNodes[nodeIndex], rpn)
				nodeIndex += 1
			}
		}
	}
	r.routeReport = reportNodes
	return r
}

func (r *reporter) Print() {
	r.InitialBaseData()
	for i := uint16(1); int(i) < len(r.routeReport)+1; i++ {
		nodeReport := r.routeReport[i][0]
		if i == 1 {
			fmt.Printf("AS%s %s「%s『%s", nodeReport.asn, nodeReport.isp, nodeReport.geo[0], nodeReport.geo[1])
		} else {
			nodeReportBefore := r.routeReport[i-1][0]
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
				fmt.Printf("』」")
				if int(i) != len(r.routeReport)+1 {
					fmt.Printf("\n ╭╯\n ╰")
				}
				if nodeReport.ix {
					fmt.Printf("AS%s \033[42;37mIXP\033[0m %s「%s『%s", nodeReport.asn, nodeReport.isp, nodeReport.geo[0], nodeReport.geo[1])
				} else {
					fmt.Printf("AS%s %s「%s『%s", nodeReport.asn, nodeReport.isp, nodeReport.geo[0], nodeReport.geo[1])
				}
			}
		}
	}
	fmt.Println("』」")
}
