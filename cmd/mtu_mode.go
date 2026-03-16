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

	result, err := mtutrace.Run(ctx, cfg)
	if err != nil {
		return err
	}

	if jsonPrint {
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
	return printMTUResult(os.Stdout, result)
}

func printMTUResult(w io.Writer, result *mtutrace.Result) error {
	if result == nil {
		return errors.New("nil mtu result")
	}
	if _, err := fmt.Fprintf(w, "tracepath to %s (%s), start MTU %d, %d byte packets\n",
		result.Target, result.ResolvedIP, result.StartMTU, result.ProbeSize); err != nil {
		return err
	}
	for _, hop := range result.Hops {
		if _, err := fmt.Fprintln(w, formatMTUHopLine(hop)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "Path MTU: %d\n", result.PathMTU)
	return err
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
	return line
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
) mtutrace.Config {
	return mtutrace.Config{
		Target:       target,
		DstIP:        dstIP,
		SrcIP:        srcIP,
		SourceDevice: srcDev,
		SrcPort:      srcPort,
		DstPort:      dstPort,
		BeginHop:     beginHop,
		MaxHops:      maxHops,
		Queries:      queries,
		Timeout:      time.Duration(timeoutMs) * time.Millisecond,
		TTLInterval:  time.Duration(ttlIntervalMs) * time.Millisecond,
		RDNS:         rdns,
	}
}
