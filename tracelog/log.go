package tracelog

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/nxtrace/NTrace-core/internal/hoprender"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

func formatTraceLogWhois(whois string) string {
	whoisFormat := strings.Split(whois, "-")
	if len(whoisFormat) > 1 {
		whoisFormat[0] = strings.Join(whoisFormat[:2], "-")
	}
	if whoisFormat[0] == "" {
		return ""
	}
	return "[" + whoisFormat[0] + "]"
}

func traceLogLocationLine(hop *trace.Hop, ip string) string {
	if hop.Geo.Country == "" {
		hop.Geo.Country = "LAN Address"
	}
	format := " %s %s %s %s %-6s\n    %-39s   "
	if net.ParseIP(ip).To4() == nil {
		format = " %s %s %s %s %-6s\n    %-35s "
	}
	return fmt.Sprintf(format, hop.Geo.Country, hop.Geo.Prov, hop.Geo.City, hop.Geo.District, hop.Geo.Owner, hop.Hostname)
}

func traceLogTimingLine(values []string) string {
	return strings.Join(values, "/ ")
}

func renderTraceLogLine(res *trace.Result, ttl int, group hoprender.Group, blockDisplay bool) string {
	var builder strings.Builder
	if blockDisplay {
		builder.WriteString(fmt.Sprintf("%4s", ""))
	}

	ip := group.IP
	if net.ParseIP(ip).To4() == nil {
		builder.WriteString(fmt.Sprintf("%-25s ", ip))
	} else {
		builder.WriteString(fmt.Sprintf("%-15s ", ip))
	}

	hop := &res.Hops[ttl][group.Index]
	if hop.Geo == nil {
		hop.Geo = &ipgeo.IPGeoData{}
	}

	if hop.Geo.Asnumber != "" {
		builder.WriteString(fmt.Sprintf("AS%-7s", hop.Geo.Asnumber))
	} else {
		builder.WriteString(fmt.Sprintf(" %-8s", "*"))
	}
	if net.ParseIP(ip).To4() != nil {
		builder.WriteString(fmt.Sprintf("%-16s", formatTraceLogWhois(hop.Geo.Whois)))
	}

	builder.WriteString(traceLogLocationLine(hop, ip))
	builder.WriteString(traceLogTimingLine(group.Timings))
	return builder.String()
}

func RealtimePrinter(res *trace.Result, ttl int) {
	f, err := os.OpenFile("/tmp/trace.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(f)

	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)
	log.SetFlags(0)
	prefix := fmt.Sprintf("%-2d  ", ttl+1)
	groups := hoprender.GroupHopAttempts(res.Hops[ttl])
	if len(groups) == 0 {
		log.Print(prefix + "*")
		return
	}

	for i, group := range groups {
		line := renderTraceLogLine(res, ttl, group, i > 0)
		if i == 0 {
			line = prefix + line
		}
		log.Print(line)
	}
}
