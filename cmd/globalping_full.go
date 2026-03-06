//go:build !flavor_tiny && !flavor_ntr

package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracemap"
	"github.com/nxtrace/NTrace-core/util"
)

func handleGlobalpingTrace(opts *trace.GlobalpingOptions, config *trace.Config) {
	res, measurement, err := trace.GlobalpingTraceroute(opts, config)
	if err != nil {
		fmt.Println(err)
		return
	}

	if !opts.DisableMaptrace &&
		(util.StringInSlice(strings.ToUpper(opts.DataOrigin), []string{"LEOMOEAPI", "IPINFO", "IP-API.COM", "IPAPI.COM"})) {
		r, err := json.Marshal(res)
		if err != nil {
			fmt.Println(err)
			return
		}
		url, err := tracemap.GetMapUrl(string(r))
		if err != nil {
			fmt.Println(err)
			return
		}
		res.TraceMapUrl = url
	}

	if opts.JSONPrint {
		r, err := json.Marshal(res)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(string(r))
		return
	}

	if measurement == nil || len(measurement.Results) == 0 {
		fmt.Println(globalpingNoResultMessage(config.Lang))
		return
	}

	fmt.Fprintln(color.Output, color.New(color.FgGreen, color.Bold).Sprintf("> %s", trace.GlobalpingFormatLocation(&measurement.Results[0])))

	if opts.TablePrint {
		printer.TracerouteTablePrinter(res)
	} else {
		for i := range res.Hops {
			if opts.ClassicPrint {
				printer.ClassicPrinter(res, i)
			} else if opts.RawPrint {
				printer.EasyPrinter(res, i)
			} else {
				printer.RealtimePrinter(res, i)
			}
		}
	}

	if res.TraceMapUrl != "" {
		tracemap.PrintMapUrl(res.TraceMapUrl)
	}
}

func globalpingNoResultMessage(lang string) string {
	if strings.EqualFold(strings.TrimSpace(lang), "en") {
		return "Globalping returned no usable probe results; skipping output."
	}
	return "Globalping 未返回可用的探测结果，已跳过输出。"
}
