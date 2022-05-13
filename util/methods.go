package util

import (
	"net"

	"github.com/xgadget-lab/nexttrace/methods"
	"github.com/xgadget-lab/nexttrace/util/printer"
)

type IPGeoData struct {
	Asnumber string `json:"asnumber"`
	Country  string `json:"country"`
	Prov     string `json:"prov"`
	City     string `json:"city"`
	District string `json:"district"`
	Owner    string `json:"owner"`
	Isp      string `json:"isp"`
}

type PrinterConfig struct {
	IP          net.IP
	DataOrigin  string
	DisplayMode string
	Rdnsenable  bool
	Results     map[uint16][]methods.TracerouteHop
}

func Printer(config *PrinterConfig) {
	switch config.DisplayMode {
	case "table":
		printer.TracerouteTablePrinter(config.IP, config.Results, config.DataOrigin, config.Rdnsenable)
	case "classic":
		printer.TraceroutePrinter(config.IP, config.Results, config.DataOrigin, config.Rdnsenable)
	case "json":
		//TracerouteJSONPrinter(config.Results, config.DataOrigin)
	default:
		printer.TraceroutePrinter(config.IP, config.Results, config.DataOrigin, config.Rdnsenable)
	}
}
