package tracelog

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/nxtrace/NTrace-core/internal/hoprender"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

var DefaultPath = filepath.Join(os.TempDir(), "trace.log")

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

func OpenFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func WriteHeader(w io.Writer, header string) error {
	if header == "" {
		return nil
	}
	_, err := io.WriteString(w, header)
	return err
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

func WriteRealtime(w io.Writer, res *trace.Result, ttl int) error {
	prefix := fmt.Sprintf("%-2d  ", ttl+1)
	groups := hoprender.GroupHopAttempts(res.Hops[ttl])
	if len(groups) == 0 {
		_, err := fmt.Fprintln(w, prefix+"*")
		return err
	}

	for i, group := range groups {
		line := renderTraceLogLine(res, ttl, group, i > 0)
		if i == 0 {
			line = prefix + line
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func NewRealtimePrinter(w io.Writer) func(res *trace.Result, ttl int) {
	return func(res *trace.Result, ttl int) {
		_ = WriteRealtime(w, res, ttl)
	}
}

func RealtimePrinter(res *trace.Result, ttl int) {
	f, err := OpenFile(DefaultPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "open trace log %q failed: %v\n", DefaultPath, err)
		_ = WriteRealtime(os.Stdout, res, ttl)
		return
	}
	defer func() { _ = f.Close() }()

	w := io.MultiWriter(os.Stdout, f)
	_ = WriteRealtime(w, res, ttl)
}
