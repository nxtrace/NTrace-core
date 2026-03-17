package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	mtutrace "github.com/nxtrace/NTrace-core/trace/mtu"
	"github.com/nxtrace/NTrace-core/util"
)

type mtuConflictFlag struct {
	flag    string
	enabled bool
}

func checkMTUConflicts(flags []mtuConflictFlag) (string, bool) {
	for _, flag := range flags {
		if flag.enabled {
			return flag.flag, false
		}
	}
	return "", true
}

func normalizeMTUProtocolFlags(tcp, udp *bool) error {
	if tcp != nil && *tcp {
		return errors.New("--mtu 仅支持 UDP，请移除 --tcp")
	}
	if udp != nil {
		*udp = true
	}
	return nil
}

func buildMTUConflictFlags(
	tcp, rawPrint bool,
	mtrModes effectiveMTRModes,
	tablePrint, classicPrint, routePath, output, deploy bool,
	globalping bool,
	from, file string,
	fastTrace bool,
) []mtuConflictFlag {
	return []mtuConflictFlag{
		{flag: "--tcp", enabled: tcp},
		{flag: "--mtr", enabled: mtrModes.mtr},
		{flag: "--raw", enabled: rawPrint},
		{flag: "--table", enabled: tablePrint},
		{flag: "--classic", enabled: classicPrint},
		{flag: "--route-path", enabled: routePath},
		{flag: "--output", enabled: output},
		{flag: "--from", enabled: globalping && from != ""},
		{flag: "--fast-trace", enabled: fastTrace},
		{flag: "--file", enabled: file != ""},
		{flag: "--deploy", enabled: deploy},
	}
}

func resolveMTUSourceIP(dstIP net.IP, srcAddr string) (net.IP, error) {
	var srcIP net.IP
	if trimmed := strings.TrimSpace(srcAddr); trimmed != "" {
		srcIP = net.ParseIP(trimmed)
		if srcIP == nil {
			return nil, fmt.Errorf("invalid source IP %q", srcAddr)
		}
	}

	if util.IsIPv6(dstIP) {
		resolved, _ := util.LocalIPPortv6(dstIP, srcIP, "udp6")
		if resolved == nil {
			return nil, fmt.Errorf("unable to determine IPv6 source address for %s", dstIP)
		}
		return resolved, nil
	}

	resolved, _ := util.LocalIPPort(dstIP, srcIP, "udp")
	if resolved == nil {
		return nil, fmt.Errorf("unable to determine IPv4 source address for %s", dstIP)
	}
	return resolved, nil
}

func runStandaloneMTUMode(cfg mtutrace.Config, jsonPrint bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if jsonPrint {
		result, err := mtutrace.Run(ctx, cfg)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
		return nil
	}

	if runtime.GOOS == "darwin" {
		fmt.Println("Warning: macOS --mtu support is experimental.")
	}
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	renderer := newMTUStreamRenderer(os.Stdout, CheckTTY(int(os.Stdout.Fd())))
	var renderErr error
	_, err := mtutrace.RunStream(streamCtx, cfg, func(event mtutrace.StreamEvent) {
		if renderErr != nil {
			return
		}
		if err := renderer.Render(event); err != nil {
			renderErr = err
			cancel()
		}
	})
	if renderErr != nil {
		return renderErr
	}
	return err
}

func printMTUResult(w io.Writer, result *mtutrace.Result) error {
	if result == nil {
		return errors.New("nil mtu result")
	}
	if err := printMTUHeader(w, result.Target, result.ResolvedIP, result.StartMTU, result.ProbeSize); err != nil {
		return err
	}
	for _, hop := range result.Hops {
		if _, err := fmt.Fprintln(w, formatMTUHopLine(hop)); err != nil {
			return err
		}
	}
	return printMTUSummary(w, result.PathMTU)
}

func formatMTUHopLine(hop mtutrace.Hop) string {
	if hop.Event == mtutrace.EventTimeout {
		line := fmt.Sprintf("%2d  *", hop.TTL)
		if hop.PMTU > 0 {
			line += fmt.Sprintf("  pmtu %d", hop.PMTU)
		}
		return line
	}

	target := hop.IP
	if hop.Hostname != "" {
		target = fmt.Sprintf("%s (%s)", hop.Hostname, hop.IP)
	}
	line := fmt.Sprintf("%2d  %s", hop.TTL, target)
	if hop.RTTMs > 0 {
		line += fmt.Sprintf("  %.2fms", hop.RTTMs)
	}
	if hop.PMTU > 0 {
		line += fmt.Sprintf("  pmtu %d", hop.PMTU)
	}
	if geo := formatMTUGeo(hop); geo != "" {
		line += "  " + geo
	}
	return line
}

func formatMTUHopSnapshot(event mtutrace.StreamEvent) string {
	if event.Kind == mtutrace.StreamEventTTLStart {
		return fmt.Sprintf("%2d  ...", event.TTL)
	}
	return formatMTUHopLine(event.Hop)
}

func printMTUHeader(w io.Writer, target, resolvedIP string, startMTU, probeSize int) error {
	_, err := fmt.Fprintf(w, "tracepath to %s (%s), start MTU %d, %d byte packets\n",
		target, resolvedIP, startMTU, probeSize)
	return err
}

func printMTUSummary(w io.Writer, pathMTU int) error {
	_, err := fmt.Fprintf(w, "Path MTU: %d\n", pathMTU)
	return err
}

type mtuStreamRenderer struct {
	w             io.Writer
	isTTY         bool
	headerPrinted bool
	lineActive    bool
}

func newMTUStreamRenderer(w io.Writer, isTTY bool) *mtuStreamRenderer {
	return &mtuStreamRenderer{w: w, isTTY: isTTY}
}

func (r *mtuStreamRenderer) Render(event mtutrace.StreamEvent) error {
	if err := r.ensureHeader(event); err != nil {
		return err
	}

	switch event.Kind {
	case mtutrace.StreamEventTTLStart:
		if !r.isTTY {
			return nil
		}
		return r.renderTTYLine(formatMTUHopSnapshot(event), false)
	case mtutrace.StreamEventTTLUpdate:
		if !r.isTTY {
			return nil
		}
		return r.renderTTYLine(formatMTUHopSnapshot(event), false)
	case mtutrace.StreamEventTTLFinal:
		line := formatMTUHopSnapshot(event)
		if r.isTTY {
			return r.renderTTYLine(line, true)
		}
		_, err := fmt.Fprintln(r.w, line)
		return err
	case mtutrace.StreamEventDone:
		if r.isTTY && r.lineActive {
			if _, err := io.WriteString(r.w, "\n"); err != nil {
				return err
			}
			r.lineActive = false
		}
		return printMTUSummary(r.w, event.PathMTU)
	default:
		return nil
	}
}

func (r *mtuStreamRenderer) ensureHeader(event mtutrace.StreamEvent) error {
	if r.headerPrinted {
		return nil
	}
	if event.Target == "" || event.ResolvedIP == "" {
		return nil
	}
	if err := printMTUHeader(r.w, event.Target, event.ResolvedIP, event.StartMTU, event.ProbeSize); err != nil {
		return err
	}
	r.headerPrinted = true
	return nil
}

func (r *mtuStreamRenderer) renderTTYLine(line string, final bool) error {
	if _, err := fmt.Fprintf(r.w, "\r\033[2K%s", line); err != nil {
		return err
	}
	r.lineActive = !final
	if !final {
		return nil
	}
	_, err := io.WriteString(r.w, "\n")
	return err
}

func buildMTUTraceConfig(
	target string,
	dstIP net.IP,
	srcIP net.IP,
	srcDev string,
	srcPort int,
	dstPort int,
	beginHop int,
	maxHops int,
	queries int,
	timeoutMs int,
	ttlIntervalMs int,
	rdns bool,
	alwaysWaitRDNS bool,
	geoSource ipgeo.Source,
	lang string,
) mtutrace.Config {
	return mtutrace.Config{
		Target:         target,
		DstIP:          dstIP,
		SrcIP:          srcIP,
		SourceDevice:   srcDev,
		SrcPort:        srcPort,
		DstPort:        dstPort,
		BeginHop:       beginHop,
		MaxHops:        maxHops,
		Queries:        queries,
		Timeout:        time.Duration(timeoutMs) * time.Millisecond,
		TTLInterval:    time.Duration(ttlIntervalMs) * time.Millisecond,
		RDNS:           rdns,
		AlwaysWaitRDNS: alwaysWaitRDNS,
		IPGeoSource:    geoSource,
		Lang:           lang,
	}
}

func formatMTUGeo(hop mtutrace.Hop) string {
	if hop.Geo == nil || hop.IP == "" {
		return ""
	}
	if hop.Geo.Asnumber == "" &&
		hop.Geo.Country == "" &&
		hop.Geo.CountryEn == "" &&
		hop.Geo.Prov == "" &&
		hop.Geo.ProvEn == "" &&
		hop.Geo.City == "" &&
		hop.Geo.CityEn == "" &&
		hop.Geo.District == "" &&
		hop.Geo.Owner == "" &&
		hop.Geo.Isp == "" &&
		hop.Geo.Whois == "" {
		return ""
	}
	return printer.FormatIPGeoData(hop.IP, hop.Geo)
}
