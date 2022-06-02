package printer

import (
	"fmt"
	"strings"

	"github.com/xgadget-lab/nexttrace/ipgeo"

	"github.com/xgadget-lab/nexttrace/trace"

	"github.com/fatih/color"
	"github.com/rodaine/table"
)

type rowData struct {
	Hop      string
	IP       string
	Latency  string
	Asnumber string
	Country  string
	Prov     string
	City     string
	District string
	Owner    string
}

func TracerouteTablePrinter(res *trace.Result) {
	// 初始化表格
	tbl := New()
	for _, hop := range res.Hops {
		for k, h := range hop {
			data := tableDataGenerator(h)
			if k > 0 {
				data.Hop = ""
			}
			if data.Country == "" && data.Prov == "" && data.City == "" {
				tbl.AddRow(data.Hop, data.IP, data.Latency, data.Asnumber, "", data.Owner)
			} else {
				if data.City != "" {
					tbl.AddRow(data.Hop, data.IP, data.Latency, data.Asnumber, data.Country+", "+data.Prov+", "+data.City, data.Owner)
				} else {
					tbl.AddRow(data.Hop, data.IP, data.Latency, data.Asnumber, data.Country + ", " + data.Prov, data.Owner)
				}

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

	tbl := table.New("Hop", "IP", "Lantency", "ASN", "Location", "Owner")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	return tbl
}

func tableDataGenerator(h trace.Hop) *rowData {
	if h.Address == nil {
		return &rowData{
			Hop: fmt.Sprint(h.TTL),
			IP:  "*",
		}
	} else {
		lantency := fmt.Sprintf("%.2fms", h.RTT.Seconds()*1000)
		IP := h.Address.String()

		if strings.HasPrefix(IP, "9.") {
			return &rowData{
				Hop:     fmt.Sprint(h.TTL),
				IP:      IP,
				Latency: lantency,
				Country: "LAN Address",
				Prov:    "LAN Address",
				Owner:   "",
			}
		} else if strings.HasPrefix(IP, "11.") {
			return &rowData{
				Hop:     fmt.Sprint(h.TTL),
				IP:      IP,
				Latency: lantency,
				Country: "LAN Address",
				Prov:    "LAN Address",
				Owner:   "",
			}
		}

		if h.Hostname != "" {
			IP = fmt.Sprint(h.Hostname, " (", IP, ") ")
		}

		if h.Geo == nil {
			h.Geo = &ipgeo.IPGeoData{}
		}

		r := &rowData{
			Hop:      fmt.Sprint(h.TTL),
			IP:       IP,
			Latency:  lantency,
			Asnumber: h.Geo.Asnumber,
			Country:  h.Geo.Country,
			Prov:     h.Geo.Prov,
			City:     h.Geo.City,
			District: h.Geo.District,
			Owner:    h.Geo.Owner,
		}

		if h.Geo == nil {
			return r
		}

		if h.Geo.Owner == "" {
			h.Geo.Owner = h.Geo.Isp
		}
		r.Asnumber = h.Geo.Asnumber
		r.Country = h.Geo.Country
		r.Prov = h.Geo.Prov
		r.City = h.Geo.City
		r.District = h.Geo.District
		r.Owner = h.Geo.Owner
		return r
	}
}
