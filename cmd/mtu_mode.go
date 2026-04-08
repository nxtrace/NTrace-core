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
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	mtutrace "github.com/nxtrace/NTrace-core/trace/mtu"
	"github.com/nxtrace/NTrace-core/util"
)

const mtuWindowsOSType = 2

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
	tablePrint, classicPrint, routePath, outputPath, outputDefault, deploy bool,
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
		{flag: "--output", enabled: outputPath},
		{flag: "--output-default", enabled: outputDefault},
		{flag: "--from", enabled: globalping && from != ""},
		{flag: "--fast-trace", enabled: fastTrace},
		{flag: "--file", enabled: file != ""},
		{flag: "--deploy", enabled: deploy},
	}
}

func resolveMTUSourceIP(dstIP net.IP, srcAddr string) (net.IP, error) {
	if trimmed := strings.TrimSpace(srcAddr); trimmed != "" {
		srcIP := net.ParseIP(trimmed)
		if srcIP == nil {
			return nil, fmt.Errorf("invalid source IP %q", srcAddr)
		}
		if util.IsIPv6(dstIP) {
			if !util.IsIPv6(srcIP) {
				return nil, fmt.Errorf("source IP %q does not match IPv6 destination %s", srcAddr, dstIP)
			}
			return srcIP, nil
		}
		if srcIP.To4() == nil {
			return nil, fmt.Errorf("source IP %q does not match IPv4 destination %s", srcAddr, dstIP)
		}
		return srcIP.To4(), nil
	}

	if util.IsIPv6(dstIP) {
		resolved, _ := util.LocalIPPortv6(dstIP, nil, "udp6")
		if resolved == nil {
			return nil, fmt.Errorf("unable to determine IPv6 source address for %s", dstIP)
		}
		return resolved, nil
	}

	resolved, _ := util.LocalIPPort(dstIP, nil, "udp")
	if resolved == nil {
		return nil, fmt.Errorf("unable to determine IPv4 source address for %s", dstIP)
	}
	return resolved, nil
}

func resolveMTUSourceDevice(osType int, requestedSrcAddr, requestedSrcDev, normalizedSrcDev string) string {
	if normalizedSrcDev != "" {
		return normalizedSrcDev
	}
	if osType != mtuWindowsOSType {
		return ""
	}
	if strings.TrimSpace(requestedSrcAddr) != "" {
		return ""
	}
	return strings.TrimSpace(requestedSrcDev)
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
	return printMTUResultWithStyle(w, result, newMTUTextStyle(false))
}

func printMTUResultWithStyle(w io.Writer, result *mtutrace.Result, style mtuTextStyle) error {
	if result == nil {
		return errors.New("nil mtu result")
	}
	if err := printMTUHeader(w, result.Target, result.ResolvedIP, result.StartMTU, result.ProbeSize, style); err != nil {
		return err
	}
	for _, hop := range result.Hops {
		if _, err := fmt.Fprintln(w, formatMTUHopLineWithStyle(hop, style)); err != nil {
			return err
		}
	}
	return printMTUSummary(w, result.PathMTU, style)
}

func formatMTUHopLine(hop mtutrace.Hop) string {
	return formatMTUHopLineWithStyle(hop, newMTUTextStyle(false))
}

func formatMTUHopLineWithStyle(hop mtutrace.Hop, style mtuTextStyle) string {
	if hop.Event == mtutrace.EventTimeout {
		line := fmt.Sprintf("%s  %s", style.ttl(hop.TTL), style.timeout())
		if hop.PMTU > 0 {
			line += "  " + style.pmtu(hop.PMTU)
		}
		return line
	}

	target := hop.IP
	if hop.Hostname != "" {
		target = fmt.Sprintf("%s (%s)", hop.Hostname, hop.IP)
	}
	line := fmt.Sprintf("%s  %s", style.ttl(hop.TTL), style.hopTarget(hop.Event, target))
	if hop.RTTMs > 0 {
		line += fmt.Sprintf("  %.2fms", hop.RTTMs)
	}
	if hop.PMTU > 0 {
		line += "  " + style.pmtu(hop.PMTU)
	}
	if geo := formatMTUGeo(hop); geo != "" {
		line += "  " + geo
	}
	return line
}

func formatMTUHopSnapshot(event mtutrace.StreamEvent) string {
	return formatMTUHopSnapshotWithStyle(event, newMTUTextStyle(false))
}

func formatMTUHopSnapshotWithStyle(event mtutrace.StreamEvent, style mtuTextStyle) string {
	if event.Kind == mtutrace.StreamEventTTLStart {
		return fmt.Sprintf("%s  %s", style.ttl(event.TTL), style.placeholder())
	}
	return formatMTUHopLineWithStyle(event.Hop, style)
}

func printMTUHeader(w io.Writer, target, resolvedIP string, startMTU, probeSize int, style mtuTextStyle) error {
	_, err := fmt.Fprintln(w, style.header(fmt.Sprintf("tracepath to %s (%s), start MTU %d, %d byte packets",
		target, resolvedIP, startMTU, probeSize)))
	return err
}

func printMTUSummary(w io.Writer, pathMTU int, style mtuTextStyle) error {
	_, err := fmt.Fprintln(w, style.summary(pathMTU))
	return err
}

type mtuStreamRenderer struct {
	w             io.Writer
	isTTY         bool
	style         mtuTextStyle
	headerPrinted bool
	lineActive    bool
}

func newMTUStreamRenderer(w io.Writer, isTTY bool) *mtuStreamRenderer {
	return &mtuStreamRenderer{
		w:     w,
		isTTY: isTTY,
		style: newMTUTextStyle(isTTY && !color.NoColor),
	}
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
		return r.renderTTYLine(formatMTUHopSnapshotWithStyle(event, r.style), false)
	case mtutrace.StreamEventTTLUpdate:
		if !r.isTTY {
			return nil
		}
		return r.renderTTYLine(formatMTUHopSnapshotWithStyle(event, r.style), false)
	case mtutrace.StreamEventTTLFinal:
		line := formatMTUHopSnapshotWithStyle(event, r.style)
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
		return printMTUSummary(r.w, event.PathMTU, r.style)
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
	if err := printMTUHeader(r.w, event.Target, event.ResolvedIP, event.StartMTU, event.ProbeSize, r.style); err != nil {
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

type mtuTextStyle struct {
	enabled bool
}

func newMTUTextStyle(enabled bool) mtuTextStyle {
	return mtuTextStyle{enabled: enabled}
}

func (s mtuTextStyle) apply(text string, attrs ...color.Attribute) string {
	if !s.enabled {
		return text
	}
	return color.New(attrs...).Sprint(text)
}

func (s mtuTextStyle) header(text string) string {
	return s.apply(text, color.FgCyan, color.Bold)
}

func (s mtuTextStyle) ttl(ttl int) string {
	return s.apply(fmt.Sprintf("%2d", ttl), color.Faint)
}

func (s mtuTextStyle) placeholder() string {
	return s.apply("...", color.FgHiBlack)
}

func (s mtuTextStyle) timeout() string {
	return s.apply("*", color.FgRed, color.Bold)
}

func (s mtuTextStyle) hopTarget(event mtutrace.Event, target string) string {
	switch event {
	case mtutrace.EventDestination:
		return s.apply(target, color.FgGreen, color.Bold)
	default:
		return s.apply(target, color.FgYellow)
	}
}

func (s mtuTextStyle) pmtu(pmtu int) string {
	return s.apply(fmt.Sprintf("pmtu %d", pmtu), color.FgCyan, color.Bold)
}

func (s mtuTextStyle) summary(pathMTU int) string {
	return s.apply(fmt.Sprintf("Path MTU: %d", pathMTU), color.FgGreen, color.Bold)
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
