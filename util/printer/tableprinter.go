package printer

import (
	"fmt"
	"net"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/methods"
)

type rowData struct {
	Hop      int64
	IP       string
	Latency  string
	Asnumber string
	Country  string
	Prov     string
	City     string
	District string
	Owner    string
}

func TracerouteTablePrinter(ip net.IP, res map[uint16][]methods.TracerouteHop, dataOrigin string) {
	// 初始化表格
	tbl := New()
	for hi := uint16(1); hi < 30; hi++ {
		for _, v := range res[hi] {
			data := tableDataGenerator(v)
			tbl.AddRow(data.Hop, data.IP, data.Latency, data.Asnumber, data.Country, data.Prov, data.City, data.Owner)
			if v.Address != nil && ip.String() == v.Address.String() {
				hi = 31
			}
		}
	}
	// 打印表格
	tbl.Print()
}

func New() table.Table {
	// 初始化表格
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("Hop", "IP", "Lantency", "ASN", "Country", "Province", "City", "Owner")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	return tbl
}

func tableDataGenerator(v2 methods.TracerouteHop) *rowData {
	if v2.Address == nil {
		return &rowData{}
	} else {
		// 初始化变量
		var iPGeoData *ipgeo.IPGeoData
		var err error
		var lantency, IP string

		ipStr := v2.Address.String()

		// TODO: 判断 err 返回，并且在CLI终端提示错误
		if dataOrigin == "LeoMoeAPI" {
			iPGeoData, err = ipgeo.LeoIP(ipStr)
		} else if dataOrigin == "IP.SB" {
			iPGeoData, err = ipgeo.IPSB(ipStr)
		} else if dataOrigin == "IPInfo" {
			iPGeoData, err = ipgeo.IPInfo(ipStr)
		} else if dataOrigin == "IPInsight" {
			iPGeoData, err = ipgeo.IPInSight(ipStr)
		} else {
			iPGeoData, err = ipgeo.LeoIP(ipStr)
		}

		if err != nil {
			fmt.Print("Error: ", err)
			return &rowData{}
		} else {

			ptr, err := net.LookupAddr(ipStr)

			lantency = fmt.Sprintf("%.2fms", v2.RTT.Seconds()*1000)

			if err != nil {
				IP = fmt.Sprint(ipStr)
			} else {
				IP = fmt.Sprint(ptr[0], " (", ipStr, ") ")
			}

			if iPGeoData.Owner == "" {
				iPGeoData.Owner = iPGeoData.Isp
			}

			return &rowData{
				Hop:      int64(v2.TTL),
				IP:       IP,
				Latency:  lantency,
				Asnumber: iPGeoData.Asnumber,
				Country:  iPGeoData.Country,
				Prov:     iPGeoData.Prov,
				City:     iPGeoData.City,
				District: iPGeoData.District,
				Owner:    iPGeoData.Owner,
			}
		}
	}
}
