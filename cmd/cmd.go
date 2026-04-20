package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/akamensky/argparse"
	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/assets/windivert"
	"github.com/nxtrace/NTrace-core/config"
	fastTrace "github.com/nxtrace/NTrace-core/fast_trace"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/reporter"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracelog"
	"github.com/nxtrace/NTrace-core/tracemap"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
)

func ptrBool(v bool) *bool    { return &v }
func ptrStr(v string) *string { return &v }
func ptrInt(v int) *int       { return &v }

type listenInfo struct {
	Binding string
	Access  string
}

const (
	defaultPacketIntervalMs        = 50
	defaultTracerouteTTLIntervalMs = 300
)

var (
	domainLookupFn = util.DomainLookUpWithContext
)

func normalizeListenAddr(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ":1080"
	}
	if isDigitsOnly(trimmed) {
		return ":" + trimmed
	}
	return trimmed
}

func splitListenAddr(effective string) (host, port string, ok bool) {
	host, port, err := net.SplitHostPort(effective)
	if err == nil {
		if port == "" {
			port = "1080"
		}
		return host, port, true
	}
	if strings.HasPrefix(effective, ":") {
		return "", strings.TrimPrefix(effective, ":"), true
	}
	return "", "", false
}

func formatHTTPListenURL(host, port string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func resolveListenAccessHost(host string) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return guessLocalIPv4()
	}
	return host
}

func buildListenInfo(addr string) listenInfo {
	effective := normalizeListenAddr(addr)
	host, port, ok := splitListenAddr(effective)
	if !ok {
		return listenInfo{Binding: effective}
	}

	rawHost := host
	if rawHost == "" {
		rawHost = "0.0.0.0"
	}

	info := listenInfo{
		Binding: formatHTTPListenURL(rawHost, port),
	}

	accessHost := resolveListenAccessHost(host)
	if accessHost != "" {
		info.Access = formatHTTPListenURL(accessHost, port)
	}

	return info
}

func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeNegativePacketSizeArgs(args []string) []string {
	if len(args) < 3 {
		return args
	}

	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		cur := args[i]
		if cur == "--psize" && i+1 < len(args) && isNegativeInteger(args[i+1]) {
			normalized = append(normalized, "--psize="+args[i+1])
			i++
			continue
		}
		normalized = append(normalized, cur)
	}
	return normalized
}

func isNegativeInteger(s string) bool {
	if !strings.HasPrefix(s, "-") || len(s) < 2 {
		return false
	}
	v, err := strconv.Atoi(s)
	return err == nil && v < 0
}

func guessLocalIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, address := range addrs {
			if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ip4 := ipNet.IP.To4(); ip4 != nil {
					return ip4.String()
				}
			}
		}
	}
	return "127.0.0.1"
}

func defaultLocalListenAddr() string {
	if hasIPv4Loopback() {
		return "127.0.0.1:1080"
	}
	if hasIPv6Loopback() {
		return "[::1]:1080"
	}
	return "127.0.0.1:1080"
}

func hasIPv4Loopback() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, address := range addrs {
		if ipNet, ok := address.(*net.IPNet); ok && ipNet.IP.IsLoopback() {
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return true
			}
		}
	}
	return false
}

func hasIPv6Loopback() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, address := range addrs {
		if ipNet, ok := address.(*net.IPNet); ok && ipNet.IP.IsLoopback() {
			if ip := ipNet.IP; ip.To4() == nil && len(ip) == net.IPv6len {
				return true
			}
		}
	}
	return false
}

// sanitizeUsagePositionalArgs replaces the auto-generated positional argument
// name (e.g. "_positionalArg_nexttrace_33") with a friendlier label in the
// usage string produced by argparse.
func sanitizeUsagePositionalArgs(usage string) string {
	// argparse generates names like "_positionalArg_nexttrace_<N>"
	// We scan for the prefix and replace the whole token with "TARGET".
	const prefix = "_positionalArg_"
	for {
		idx := strings.Index(usage, prefix)
		if idx < 0 {
			break
		}
		// Find the end of the token (next space, newline, or end of string).
		end := idx + len(prefix)
		for end < len(usage) && usage[end] != ' ' && usage[end] != '\n' && usage[end] != '\r' && usage[end] != '\t' && usage[end] != ']' {
			end++
		}
		usage = usage[:idx] + "TARGET" + usage[end:]
	}
	// argparse renders the positional as "--TARGET" in the description list;
	// strip the leading "--" so it reads as a plain positional placeholder.
	usage = strings.ReplaceAll(usage, "--TARGET", "TARGET")
	// Fix the description column alignment for the TARGET entry.
	// argparse gives positional args minimal spacing ("      TARGET  desc"), but named
	// flags are padded to a consistent description column ("      --name              desc").
	// Detect that column from any named-flag line and re-pad the TARGET line to match.
	usage = fixPositionalAlignment(usage)
	return usage
}

// fixPositionalAlignment detects the description column used by named flags in the
// argparse help output and re-pads the TARGET positional entry to match it.
func fixPositionalAlignment(usage string) string {
	// Scan flag lines to find where descriptions start.
	// A flag line looks like "  -X  --name<spaces>Description" or "      --name<spaces>Description".
	// We find the column of the first non-space character after the flag name (past position 8).
	descCol := 0
	for _, line := range strings.Split(usage, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(trimmed, "-") || strings.Contains(line, "TARGET") {
			continue
		}
		inGap := false
		for i := 8; i < len(line); i++ {
			if line[i] == ' ' {
				inGap = true
			} else if inGap {
				descCol = i
				break
			}
		}
		if descCol > 0 {
			break
		}
	}
	if descCol == 0 {
		return usage
	}
	// Find the TARGET description entry: "\n      TARGET  <description>"
	const namePrefix = "      TARGET"
	marker := "\n" + namePrefix
	idx := strings.Index(usage, marker)
	if idx < 0 {
		return usage
	}
	// afterName points to the character right after "      TARGET" on that line.
	afterName := idx + 1 + len(namePrefix)
	// Skip the existing (minimal) spacing.
	end := afterName
	for end < len(usage) && usage[end] == ' ' {
		end++
	}
	needed := descCol - len(namePrefix)
	if needed <= 0 {
		return usage
	}
	return usage[:afterName] + strings.Repeat(" ", needed) + usage[end:]
}

type effectiveMTRModes struct {
	mtr    bool
	report bool
	wide   bool
	raw    bool
}

type tracerouteOutputFlags struct {
	routePath     *bool
	outputPath    *string
	outputDefault *bool
	tablePrint    *bool
	jsonPrint     *bool
	classicPrint  *bool
}

type webUIFlags struct {
	deployListen *string
	deploy       *bool
}

type mtrCLIFlags struct {
	mtrMode    *bool
	reportMode *bool
	wideMode   *bool
	showIPs    *bool
	ipInfoMode *int
}

const windowsInitHelpText = "Extract WinDivert runtime to executable directory"

func registerInitFlag(parser *argparse.Parser) *bool {
	if runtime.GOOS == "windows" {
		return parser.Flag("", "init", &argparse.Options{Help: windowsInitHelpText})
	}
	return ptrBool(false)
}

func registerFastTraceFlag(parser *argparse.Parser) *bool {
	if !defaultMTR {
		return parser.Flag("F", "fast-trace", &argparse.Options{Help: "One-Key Fast Trace to China ISPs"})
	}
	return ptrBool(false)
}

func registerMTUFlag(parser *argparse.Parser) *bool {
	if enableMTU {
		return parser.Flag("", "mtu", &argparse.Options{Help: "Run standalone UDP path-MTU discovery mode with streaming output and GeoIP/RDNS"})
	}
	return ptrBool(false)
}

func registerICMPModeFlag(parser *argparse.Parser) *int {
	if runtime.GOOS == "windows" {
		return parser.Int("", "icmp-mode", &argparse.Options{Help: "Choose the method to listen for ICMP packets (1=Socket, 2=WinDivert; 0=Auto)"})
	}
	return ptrInt(0)
}

func buildQueriesHelp() string {
	if defaultMTR {
		return "MTR only: max probes per hop. 0 = unlimited in TUI/raw; --report defaults to 10 when omitted. Start with 10-20 on unstable paths"
	}
	return "Latency samples per hop. Increase to 5-10 on unstable paths for a steadier view"
}

func buildMaxAttemptsHelp() string {
	return "Advanced: hard cap on probe packets per hop. Leave unset for auto sizing; raise on lossy links if --queries is not enough"
}

func buildParallelRequestsHelp() string {
	return "Advanced: total concurrent in-flight probes across TTLs. Use 1 on multipath/load-balanced paths; 6-18 is a good starting range on stable links"
}

func buildPacketIntervalHelp() string {
	help := "Advanced: per-packet gap [ms] inside the same TTL group. Lower is faster; raise to 100-200ms on rate-limited links"
	if enableMTR {
		help += ". Ignored in MTR mode"
	}
	return help
}

func buildTimeoutHelp() string {
	return "Per-probe timeout [ms]. Raise to 2000-3000 on slow intercontinental or high-loss paths"
}

func buildPayloadSizeHelp() string {
	return "Probe packet size in bytes, inclusive IP and active probe headers. Default is the minimum legal size for the chosen protocol and IP family; raise for MTU or large-packet testing. Negative values randomize each probe up to abs(value)"
}

func buildTOSHelp() string {
	return "Set the IP type-of-service / traffic class value [0-255]"
}

func registerTracerouteOutputFlags(parser *argparse.Parser) tracerouteOutputFlags {
	if !defaultMTR {
		return tracerouteOutputFlags{
			routePath:     parser.Flag("P", "route-path", &argparse.Options{Help: "Print traceroute hop path by ASN and location"}),
			outputPath:    parser.String("o", "output", &argparse.Options{Help: "Write trace result to FILE (RealtimePrinter only)"}),
			outputDefault: parser.Flag("O", "output-default", &argparse.Options{Help: "Write trace result to the default log file (/tmp/trace.log)"}),
			tablePrint:    parser.Flag("", "table", &argparse.Options{Help: "Output trace results as a final summary table (traceroute report mode)"}),
			jsonPrint:     parser.Flag("j", "json", &argparse.Options{Help: "Output trace results as JSON"}),
			classicPrint:  parser.Flag("c", "classic", &argparse.Options{Help: "Classic Output trace results like BestTrace"}),
		}
	}
	return tracerouteOutputFlags{
		routePath:     ptrBool(false),
		outputPath:    ptrStr(""),
		outputDefault: ptrBool(false),
		tablePrint:    ptrBool(false),
		jsonPrint:     ptrBool(false),
		classicPrint:  ptrBool(false),
	}
}

func registerWebUIFlags(parser *argparse.Parser) webUIFlags {
	return registerWebUIFlagsWithAvailability(parser, enableWebUI)
}

func registerWebUIFlagsWithAvailability(parser *argparse.Parser, enabled bool) webUIFlags {
	if enabled {
		return webUIFlags{
			deployListen: parser.String("", "listen", &argparse.Options{Help: "Set listen address for web console (e.g. 127.0.0.1:30080)"}),
			deploy:       parser.Flag("", "deploy", &argparse.Options{Help: "Start the Gin powered web console"}),
		}
	}
	return webUIFlags{
		deployListen: parser.String("", "listen", &argparse.Options{Help: "Set listen address for web console (full build only; unavailable in this binary)"}),
		deploy:       parser.Flag("", "deploy", &argparse.Options{Help: "Start the Gin powered web console (full build only; unavailable in this binary)"}),
	}
}

func registerPacketIntervalFlag(parser *argparse.Parser) *int {
	if !defaultMTR {
		return parser.Int("z", "send-time", &argparse.Options{Default: defaultPacketIntervalMs, Help: buildPacketIntervalHelp()})
	}
	return ptrInt(defaultPacketIntervalMs)
}

func buildRawHelp() string {
	rawHelp := "Machine-friendly output"
	if enableMTR {
		mtrFlags := "--mtr/-r/-w"
		if defaultMTR {
			mtrFlags = "-r/-w"
		}
		rawHelp += ". With MTR (" + mtrFlags + "), enables streaming raw event mode"
	}
	return rawHelp
}

func buildTTLIntervalHelp() string {
	if !enableMTR {
		return "Advanced: TTL-group interval [ms] in normal traceroute. 100-300ms is usually safe; lower is faster but may trigger rate limits"
	}
	if defaultMTR {
		return "Advanced: per-hop probe interval [ms] in MTR mode. 500-1000ms is a good starting point; omitted defaults to 1000ms"
	}
	return "Advanced: TTL-group interval [ms] in normal traceroute. In MTR mode (--mtr/-r/-w, including --raw), this becomes per-hop probe interval. 500-1000ms is a good MTR starting range"
}

func registerTTLIntervalFlag(parser *argparse.Parser) *int {
	return registerTTLIntervalFlagWithMTRSupport(parser, enableMTR)
}

func registerTTLIntervalFlagWithMTRSupport(parser *argparse.Parser, mtrEnabled bool) *int {
	options := &argparse.Options{Help: buildTTLIntervalHelp()}
	if !mtrEnabled {
		options.Default = defaultTracerouteTTLIntervalMs
	}
	return parser.Int("i", "ttl-time", options)
}

func applyTTLIntervalDefault(ttlInterval *int, ttlTimeExplicit, effectiveMTR bool) {
	if ttlInterval == nil || ttlTimeExplicit || effectiveMTR {
		return
	}
	*ttlInterval = defaultTracerouteTTLIntervalMs
}

func registerDisableMaptraceFlag(parser *argparse.Parser) *bool {
	if !defaultMTR {
		return parser.Flag("M", "map", &argparse.Options{Help: "Disable Print Trace Map"})
	}
	return ptrBool(true)
}

func registerGlobalpingFlag(parser *argparse.Parser) *string {
	return registerGlobalpingFlagWithAvailability(parser, enableGlobalping)
}

func registerGlobalpingFlagWithAvailability(parser *argparse.Parser, enabled bool) *string {
	if enabled {
		return parser.String("", "from", &argparse.Options{Help: "Run traceroute via Globalping (https://globalping.io/network) from a specified location. The location field accepts continents, countries, regions, cities, ASNs, ISPs, or cloud regions."})
	}
	return parser.String("", "from", &argparse.Options{Help: "Run traceroute via Globalping (full build only; unavailable in this binary)"})
}

func registerMTRFlags(parser *argparse.Parser) mtrCLIFlags {
	if enableMTR {
		mtrMode := ptrBool(true)
		if !defaultMTR {
			mtrMode = parser.Flag("t", "mtr", &argparse.Options{Help: "Enable MTR (My Traceroute) continuous probing mode"})
		}
		return mtrCLIFlags{
			mtrMode:    mtrMode,
			reportMode: parser.Flag("r", "report", &argparse.Options{Help: "MTR report mode (non-interactive, implies --mtr); can trigger MTR without --mtr"}),
			wideMode:   parser.Flag("w", "wide", &argparse.Options{Help: "MTR wide report mode (implies --mtr --report); alone equals --mtr --report --wide"}),
			showIPs:    parser.Flag("", "show-ips", &argparse.Options{Help: "MTR only: display both PTR hostnames and numeric IPs (PTR first, IP in parentheses)"}),
			ipInfoMode: parser.Int("y", "ipinfo", &argparse.Options{Default: 0, Help: "Set initial MTR TUI host info mode (0-4). TUI only; ignored in --report/--raw. 0:IP/PTR 1:ASN 2:City 3:Owner 4:Full"}),
		}
	}
	return mtrCLIFlags{
		mtrMode:    ptrBool(false),
		reportMode: ptrBool(false),
		wideMode:   ptrBool(false),
		showIPs:    ptrBool(false),
		ipInfoMode: ptrInt(0),
	}
}

func registerFileFlag(parser *argparse.Parser) *string {
	if !defaultMTR {
		return parser.String("", "file", &argparse.Options{Help: "Read IP Address or domain name from file"})
	}
	return ptrStr("")
}

func deriveEffectiveMTRModes(mtrMode, reportMode, wideMode, rawPrint bool) effectiveMTRModes {
	mtr := mtrMode || reportMode || wideMode
	return effectiveMTRModes{
		mtr:    mtr,
		report: reportMode || wideMode,
		wide:   wideMode,
		raw:    mtr && rawPrint,
	}
}

func detectExplicitProbeFlags(parser *argparse.Parser) (queriesExplicit, ttlTimeExplicit, packetSizeExplicit, tosExplicit bool) {
	for _, a := range parser.GetArgs() {
		if !a.GetParsed() {
			continue
		}
		switch a.GetLname() {
		case "queries":
			queriesExplicit = true
		case "ttl-time":
			ttlTimeExplicit = true
		case "psize":
			packetSizeExplicit = true
		case "tos":
			tosExplicit = true
		}
	}
	return queriesExplicit, ttlTimeExplicit, packetSizeExplicit, tosExplicit
}

func resolvePacketSizeArg(packetSize int, explicit bool, method trace.Method, dstIP net.IP) int {
	if explicit {
		return packetSize
	}
	return trace.DefaultPacketSize(method, dstIP)
}

func applyColorMode(noColor bool) {
	color.NoColor = noColor
}

func shouldForceNoColorForMTUNonTTY(mtuMode, jsonPrint, stdoutIsTTY bool) bool {
	return mtuMode && !jsonPrint && !stdoutIsTTY
}

func printStartupBanner(jsonPrint bool, effectiveMTR bool) {
	if !jsonPrint && !effectiveMTR {
		printer.Version()
	}
}

func maybePrintVersion(ver bool) bool {
	if !ver {
		return false
	}
	printer.CopyRight()
	os.Exit(0)
	return true
}

func maybeRunDeployMode(deploy bool, deployListen string) bool {
	if !deploy {
		return false
	}
	if !enableWebUI {
		if err := runDeploy("", nil); err != nil {
			if util.EnvDevMode {
				panic(err)
			}
			log.Fatal(err)
		}
		return true
	}

	capabilitiesCheck()
	listenAddr := strings.TrimSpace(deployListen)
	envAddr := strings.TrimSpace(util.EnvDeployAddr)
	userProvided := listenAddr != "" || envAddr != ""
	if listenAddr == "" {
		listenAddr = envAddr
	}
	if listenAddr == "" {
		listenAddr = defaultLocalListenAddr()
	}

	onReady := func(addr net.Addr) {
		info := buildListenInfo(addr.String())
		fmt.Printf("启动 NextTrace Web 控制台，监听地址: %s\n", info.Binding)
		if !userProvided {
			fmt.Println("远程访问请显式设置 --listen（例如 --listen 0.0.0.0:1080）。")
		}
		if info.Access != "" && info.Access != info.Binding {
			fmt.Printf("如需远程访问，请尝试: %s\n", info.Access)
		}
		fmt.Println("注意：Web 控制台的安全性有限，请在确保安全的前提下使用，如有必要请使用ACL等方式加强安全性")
	}
	if err := runDeploy(listenAddr, onReady); err != nil {
		if util.EnvDevMode {
			panic(err)
		}
		log.Fatal(err)
	}
	return true
}

func handleStartupModes(noColor, jsonPrint bool, modes effectiveMTRModes, ver, deploy bool, deployListen string, init bool, osType int) bool {
	applyColorMode(noColor)
	printStartupBanner(jsonPrint, modes.mtr)
	if maybePrintVersion(ver) {
		return true
	}
	if maybeRunDeployMode(deploy, deployListen) {
		return true
	}
	return maybePrepareWinDivert(init, osType)
}

func resolveOSType() int {
	switch runtime.GOOS {
	case "darwin":
		return 1
	case "windows":
		return 2
	default:
		return 3
	}
}

func maybePrepareWinDivert(init bool, osType int) bool {
	if !init || osType != 2 {
		return false
	}
	if err := windivert.PrepareWinDivertRuntime(); err != nil {
		if util.EnvDevMode {
			panic(err)
		}
		log.Fatal(err)
	}
	fmt.Println("WinDivert runtime is ready.")
	return true
}

func applyDefaultPort(port *int, udp bool) {
	if *port != 0 {
		return
	}
	if udp {
		*port = 33494
		return
	}
	*port = 80
}

func clampProbeSettings(tcp bool, numMeasurements, maxAttempts *int) {
	if tcp {
		return
	}
	if *numMeasurements > 255 {
		fmt.Println("Query 最大值为 255，已自动调整为 255")
		*numMeasurements = 255
	}
	if *maxAttempts > 255 {
		fmt.Println("MaxAttempt 最大值为 255，已自动调整为 255")
		*maxAttempts = 255
	}
}

func resolveTraceMethod(tcp, udp bool) trace.Method {
	switch {
	case tcp:
		return trace.TCPTrace
	case udp:
		return trace.UDPTrace
	default:
		return trace.ICMPTrace
	}
}

func maybeRunFastTraceMode(from string, fastTraceFlag bool, file string, params fastTrace.ParamsFastTrace, method trace.Method) bool {
	if from != "" || (!fastTraceFlag && file == "") {
		return false
	}
	fastTrace.FastTest(method, params)
	if params.OutputPath != "" {
		fmt.Printf("您的追踪日志已经存放在 %s 中\n", params.OutputPath)
	}
	os.Exit(0)
	return true
}

func configureGeoDNS(dot string) {
	if dot != "" {
		util.SetGeoDNSResolver(dot)
	}
}

func normalizeCLITarget(raw string) string {
	domain := raw
	if strings.Contains(domain, "/") {
		domain = "n" + domain
		parts := strings.Split(domain, "/")
		if len(parts) < 3 {
			return ""
		}
		domain = parts[2]
	}
	if strings.Contains(domain, "]") && strings.Contains(domain, "[") {
		inner := strings.SplitN(domain, "]", 2)[0]
		parts := strings.SplitN(inner, "[", 2)
		if len(parts) >= 2 {
			return parts[1]
		}
		return domain
	}
	if strings.Contains(domain, ":") && strings.Count(domain, ":") == 1 {
		return strings.Split(domain, ":")[0]
	}
	return domain
}

func resolveCLITargetOrExit(raw string, usage string) string {
	if raw == "" {
		fmt.Print(usage)
		return ""
	}
	domain := normalizeCLITarget(raw)
	if domain == "" {
		if strings.Contains(raw, "/") {
			fmt.Println("Invalid input")
		} else {
			fmt.Print(usage)
		}
	}
	return domain
}

func applyDN42Mode(enabled bool, dataOrigin *string, disableMaptrace *bool) {
	if !enabled {
		return
	}
	applyDN42DataOrigin(dataOrigin)
	*disableMaptrace = true
}

func applyDN42DataOrigin(dataOrigin *string) {
	config.InitConfig()
	*dataOrigin = "DN42"
}

func prepareRuntimeEnvironment(ctx context.Context, dn42 bool, dataOrigin *string, disableMaptrace *bool, powProvider *string, asyncLeo bool) *wshandle.WsConn {
	capabilitiesCheck()
	applyDN42Mode(dn42, dataOrigin, disableMaptrace)
	return initLeoWebsocket(ctx, dataOrigin, powProvider, asyncLeo)
}

func initLeoWebsocket(ctx context.Context, dataOrigin, powProvider *string, async bool) *wshandle.WsConn {
	if !strings.EqualFold(*dataOrigin, "LEOMOEAPI") {
		return nil
	}
	if !strings.EqualFold(*powProvider, "api.nxtrace.org") {
		util.PowProviderParam = *powProvider
	}
	if util.EnvDataProvider != "" {
		*dataOrigin = util.EnvDataProvider
	}
	if !strings.EqualFold(*dataOrigin, "LEOMOEAPI") {
		return nil
	}

	var leoWs *wshandle.WsConn
	if async {
		leoWs = wshandle.NewWithContextAsync(ctx)
	} else {
		leoWs = wshandle.NewWithContext(ctx)
	}
	return leoWs
}

func closeLeoWebsocket(leoWs *wshandle.WsConn) {
	if leoWs != nil {
		leoWs.Close()
	}
}

func maybeHandleGlobalping(from string, opts *trace.GlobalpingOptions, conf *trace.Config) bool {
	if from == "" {
		return false
	}
	handleGlobalpingTrace(opts, conf)
	return true
}

func lookupTargetIP(ctx context.Context, domain string, ipv4Only, ipv6Only bool, dot string, jsonPrint bool) (net.IP, error) {
	switch {
	case ipv6Only:
		return domainLookupFn(ctx, domain, "6", dot, jsonPrint)
	case ipv4Only:
		return domainLookupFn(ctx, domain, "4", dot, jsonPrint)
	default:
		return domainLookupFn(ctx, domain, "all", dot, jsonPrint)
	}
}

func lookupTargetIPOrExit(ctx context.Context, domain string, ipv4Only, ipv6Only bool, dot string, jsonPrint bool) net.IP {
	ip, err := lookupTargetIP(ctx, domain, ipv4Only, ipv6Only, dot, jsonPrint)
	if err != nil {
		if util.EnvDevMode {
			panic(err)
		}
		log.Fatal(err)
	}
	return ip
}

func resolveConfiguredSrcAddr(dstIP net.IP, srcAddr, srcDev string) (resolved string, explicit bool, err error) {
	return trace.ResolveConfiguredSrcAddr(dstIP, srcAddr, srcDev)
}

func printTraceNav(jsonPrint bool, effectiveMTR bool, ip net.IP, domain, dataOrigin string, maxHops, packetSize int, srcAddr string, method trace.Method) {
	if !jsonPrint && !effectiveMTR {
		printer.PrintTraceRouteNav(ip, domain, dataOrigin, maxHops, packetSize, srcAddr, string(method))
	}
}

func buildTraceConfig(
	osType, icmpMode int,
	dn42 bool,
	srcAddr string,
	sourceDevice string,
	srcPort int,
	beginHop int,
	ip net.IP,
	port int,
	maxHops int,
	packetInterval int,
	ttlInterval int,
	numMeasurements int,
	maxAttempts int,
	parallelRequests int,
	lang string,
	noRDNS bool,
	alwaysRDNS bool,
	dataOrigin string,
	timeout int,
	packetSize int,
	randomPacketSize bool,
	tos int,
	disableMPLS bool,
) trace.Config {
	return trace.Config{
		OSType:           osType,
		ICMPMode:         icmpMode,
		DN42:             dn42,
		SrcAddr:          srcAddr,
		SrcPort:          srcPort,
		SourceDevice:     strings.TrimSpace(sourceDevice),
		BeginHop:         beginHop,
		DstIP:            ip,
		DstPort:          port,
		MaxHops:          maxHops,
		PacketInterval:   packetInterval,
		TTLInterval:      ttlInterval,
		NumMeasurements:  numMeasurements,
		MaxAttempts:      maxAttempts,
		ParallelRequests: parallelRequests,
		Lang:             lang,
		RDNS:             !noRDNS,
		AlwaysWaitRDNS:   alwaysRDNS,
		IPGeoSource:      ipgeo.GetSource(dataOrigin),
		Timeout:          time.Duration(timeout) * time.Millisecond,
		PktSize:          packetSize,
		RandomPacketSize: randomPacketSize,
		TOS:              tos,
		DisableMPLS:      disableMPLS,
	}
}

func maybeRunMTRMode(
	modes effectiveMTRModes,
	method trace.Method,
	conf trace.Config,
	queriesExplicit bool,
	numMeasurements int,
	ttlTimeExplicit bool,
	ttlInterval int,
	domain string,
	dataOrigin string,
	showIPs bool,
	ipInfoMode int,
) bool {
	if !modes.mtr {
		return false
	}
	mtrMaxPerHop, mtrHopIntervalMs := deriveMTRProbeParams(
		modes.report,
		queriesExplicit,
		numMeasurements,
		ttlTimeExplicit,
		ttlInterval,
	)

	switch chooseMTRRunMode(modes.raw, modes.report) {
	case mtrRunRaw:
		runMTRRaw(method, conf, mtrHopIntervalMs, mtrMaxPerHop, dataOrigin)
	case mtrRunReport:
		runMTRReport(method, conf, mtrHopIntervalMs, mtrMaxPerHop, domain, dataOrigin, modes.wide, showIPs)
	default:
		if ipInfoMode < 0 || ipInfoMode > 4 {
			fmt.Fprintf(os.Stderr, "--ipinfo/-y 必须在 0-4 范围内，当前值: %d\n", ipInfoMode)
			os.Exit(1)
		}
		runMTRTUI(method, conf, mtrHopIntervalMs, mtrMaxPerHop, domain, dataOrigin, showIPs, ipInfoMode)
	}
	return true
}

func resolveOutputPath(outputPath string, outputDefault bool) (string, error) {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed != "" && outputDefault {
		return "", errors.New("--output 与 --output-default 不能同时使用")
	}
	if trimmed != "" {
		return trimmed, nil
	}
	if outputDefault {
		return tracelog.DefaultPath, nil
	}
	return "", nil
}

func validateJSONRealtimeOutput(jsonPrint bool, outputPath string) error {
	if jsonPrint && strings.TrimSpace(outputPath) != "" {
		return errors.New("--json 不能与 --output/--output-default 同时使用")
	}
	return nil
}

func setFastIPOutputSuppression(suppress bool) func() {
	prev := util.SuppressFastIPOutput
	util.SuppressFastIPOutput = suppress
	return func() {
		util.SuppressFastIPOutput = prev
	}
}

func configureTracePrinters(conf *trace.Config, tablePrint, classicPrint, rawPrint bool, outputPath string) (func() error, error) {
	if tablePrint {
		return nil, nil
	}
	router := false
	switch {
	case classicPrint:
		conf.RealtimePrinter = printer.ClassicPrinter
	case rawPrint:
		conf.RealtimePrinter = printer.EasyPrinter
	case outputPath != "":
		f, err := tracelog.OpenFile(outputPath)
		if err != nil {
			return nil, err
		}
		conf.RealtimePrinter = tracelog.NewRealtimePrinter(io.MultiWriter(os.Stdout, f))
		return f.Close, nil
	case router:
		conf.RealtimePrinter = printer.RealtimePrinterWithRouter
		fmt.Println("路由表数据源由 BGP.Tools 提供，在此特表感谢")
	default:
		conf.RealtimePrinter = printer.RealtimePrinter
	}
	return nil, nil
}

func applyJSONOutputMode(conf *trace.Config, jsonPrint bool) {
	if jsonPrint {
		conf.RealtimePrinter = nil
		conf.AsyncPrinter = nil
	}
}

func maybeRunUninterruptedRaw(rawPrint bool, method trace.Method, conf trace.Config) {
	if !(util.Uninterrupted && rawPrint) {
		return
	}
	for {
		if _, err := trace.Traceroute(method, conf); err != nil {
			fmt.Println(err)
		}
	}
}

func runTraceOnce(method trace.Method, conf trace.Config) (*trace.Result, bool) {
	res, err := trace.Traceroute(method, conf)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			fmt.Println(err)
		}
		return nil, false
	}
	return res, true
}

func finalizeTraceResult(ctx context.Context, res *trace.Result, tablePrint, tableClearScreen, routePath bool, dstIP net.IP, disableMaptrace, jsonPrint bool, dataOrigin string) {
	if tablePrint {
		printer.TracerouteTablePrinter(res, tableClearScreen)
	}
	if routePath {
		reporter.New(res, dstIP.String()).Print()
	}

	r, err := json.Marshal(res)
	if err != nil {
		fmt.Println(err)
		return
	}
	if !disableMaptrace &&
		(util.StringInSlice(strings.ToUpper(dataOrigin), []string{"LEOMOEAPI", "IPINFO", "IP-API.COM", "IPAPI.COM"})) {
		url, err := tracemap.GetMapUrlWithContext(ctx, string(r))
		if err != nil {
			fmt.Println(err)
			return
		}
		res.TraceMapUrl = url
		if !jsonPrint {
			tracemap.PrintMapUrl(url)
		}
	}
	r, err = json.Marshal(res)
	if err != nil {
		fmt.Println(err)
		return
	}
	if jsonPrint {
		fmt.Println(string(r))
	}
}

func Execute() {
	if handled, exitCode := maybeRunSpeedMode(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(exitCode)
	}

	parser := argparse.NewParser(appBinName, "An open source visual route tracking CLI tool")
	// Override HelpFunc so positional arg names are sanitized in --help output
	parser.HelpFunc = func(c *argparse.Command, msg interface{}) string {
		return sanitizeUsagePositionalArgs(c.Usage(msg))
	}
	init := registerInitFlag(parser)
	ipv4Only := parser.Flag("4", "ipv4", &argparse.Options{Help: "Use IPv4 only"})
	ipv6Only := parser.Flag("6", "ipv6", &argparse.Options{Help: "Use IPv6 only"})
	tcp := parser.Flag("T", "tcp", &argparse.Options{Help: "Use TCP SYN for tracerouting (default dest-port is 80)"})
	udp := parser.Flag("U", "udp", &argparse.Options{Help: "Use UDP SYN for tracerouting (default dest-port is 33494)"})
	mtuMode := registerMTUFlag(parser)
	fastTraceFlag := registerFastTraceFlag(parser)
	port := parser.Int("p", "port", &argparse.Options{Help: "Set the destination port to use. With default of 80 for \"tcp\", 33494 for \"udp\""})
	icmpMode := registerICMPModeFlag(parser)
	numMeasurements := parser.Int("q", "queries", &argparse.Options{Default: 3, Help: buildQueriesHelp()})
	maxAttempts := parser.Int("", "max-attempts", &argparse.Options{Help: buildMaxAttemptsHelp()})
	parallelRequests := parser.Int("", "parallel-requests", &argparse.Options{Default: 18, Help: buildParallelRequestsHelp()})
	maxHops := parser.Int("m", "max-hops", &argparse.Options{Default: 30, Help: "Set the max number of hops (max TTL to be reached)"})
	dataOrigin := parser.Selector("d", "data-provider", []string{"IP.SB", "ip.sb", "IPInfo", "ipinfo", "IPInsight", "ipinsight", "IPAPI.com", "ip-api.com", "IPInfoLocal", "ipinfolocal", "chunzhen", "LeoMoeAPI", "leomoeapi", "ipdb.one", "disable-geoip"}, &argparse.Options{Default: "LeoMoeAPI",
		Help: "Choose IP Geograph Data Provider [IP.SB, IPInfo, IPInsight, IP-API.com, IPInfoLocal, CHUNZHEN, disable-geoip]"})
	powProvider := parser.Selector("", "pow-provider", []string{"api.nxtrace.org", "sakura"}, &argparse.Options{Default: "api.nxtrace.org",
		Help: "Choose PoW Provider [api.nxtrace.org, sakura] For China mainland users, please use sakura"})
	norDNS := parser.Flag("n", "no-rdns", &argparse.Options{Help: "Do not resolve IP addresses to their domain names"})
	alwaysrDNS := parser.Flag("a", "always-rdns", &argparse.Options{Help: "Always resolve IP addresses to their domain names"})
	outputFlags := registerTracerouteOutputFlags(parser)
	routePath := outputFlags.routePath
	outputPath := outputFlags.outputPath
	outputDefault := outputFlags.outputDefault
	tablePrint := outputFlags.tablePrint
	jsonPrint := outputFlags.jsonPrint
	classicPrint := outputFlags.classicPrint
	dn42 := parser.Flag("", "dn42", &argparse.Options{Help: "DN42 Mode"})
	rawPrint := parser.Flag("", "raw", &argparse.Options{Help: buildRawHelp()})
	beginHop := parser.Int("f", "first", &argparse.Options{Default: 1, Help: "Start from the first_ttl hop (instead of 1)"})
	disableMaptrace := registerDisableMaptraceFlag(parser)
	disableMPLS := parser.Flag("e", "disable-mpls", &argparse.Options{Help: "Disable MPLS"})
	ver := parser.Flag("V", "version", &argparse.Options{Help: "Print version info and exit"})
	speedMode := registerSpeedFlag(parser)
	naliMode := registerNaliFlag(parser)
	srcAddr := parser.String("s", "source", &argparse.Options{Help: "Use source address src_addr for outgoing packets"})
	srcPort := parser.Int("", "source-port", &argparse.Options{Help: "Use source port src_port for outgoing packets"})
	srcDev := parser.String("D", "dev", &argparse.Options{Help: "Use the specified network device for explicit source selection. On Windows, this selects the device source address; routing may still choose the egress interface"})

	webFlags := registerWebUIFlags(parser)
	deployListen := webFlags.deployListen
	deploy := webFlags.deploy

	//router := parser.Flag("R", "route", &argparse.Options{Help: "Show Routing Table [Provided By BGP.Tools]"})
	// ── Send-time: hidden in ntr (always ignored in MTR mode) ──
	packetInterval := registerPacketIntervalFlag(parser)
	ttlInterval := registerTTLIntervalFlag(parser)
	timeout := parser.Int("", "timeout", &argparse.Options{Default: 1000, Help: buildTimeoutHelp()})
	packetSize := parser.Int("", "psize", &argparse.Options{Help: buildPayloadSizeHelp()})
	tos := parser.Int("Q", "tos", &argparse.Options{Default: 0, Help: buildTOSHelp()})
	dot := parser.Selector("", "dot-server", []string{"dnssb", "aliyun", "dnspod", "google", "cloudflare"}, &argparse.Options{
		Help: "Use DoT Server for DNS Parse [dnssb, aliyun, dnspod, google, cloudflare]"})
	lang := parser.Selector("g", "language", []string{"en", "cn"}, &argparse.Options{Default: "cn",
		Help: "Choose the language for displaying [en, cn]"})
	noColor := parser.Flag("C", "no-color", &argparse.Options{Help: "Disable Colorful Output"})

	// ── Globalping flag (full only) ──
	from := registerGlobalpingFlag(parser)

	// ── MTR flags (full & ntr only) ──
	mtrFlags := registerMTRFlags(parser)
	mtrMode := mtrFlags.mtrMode
	reportMode := mtrFlags.reportMode
	wideMode := mtrFlags.wideMode
	showIPs := mtrFlags.showIPs
	ipInfoMode := mtrFlags.ipInfoMode

	// ── File: hidden in ntr (conflicts with default MTR mode) ──
	file := registerFileFlag(parser)
	str := parser.StringPositional(&argparse.Options{Help: "Trace target: IPv4 address (e.g. 8.8.8.8), IPv6 address (e.g. 2001:db8::1), domain name (e.g. example.com), or URL (e.g. https://example.com)"})

	err := parser.Parse(normalizeNegativePacketSizeArgs(os.Args))
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(sanitizeUsagePositionalArgs(parser.Usage(err)))
		return
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	util.SrcDev = ""

	mtrModes := deriveEffectiveMTRModes(*mtrMode, *reportMode, *wideMode, *rawPrint)
	if *naliMode {
		applyColorMode(*noColor)
		if maybePrintVersion(*ver) {
			return
		}
		if err := validateNaliModeOptions(buildNaliModeOptions(naliModeOptionInputs{
			parser:        parser,
			ipv4Only:      *ipv4Only,
			ipv6Only:      *ipv6Only,
			tcp:           *tcp,
			udp:           *udp,
			mtu:           *mtuMode,
			mtrModes:      mtrModes,
			raw:           *rawPrint,
			table:         *tablePrint,
			classic:       *classicPrint,
			json:          *jsonPrint,
			outputPath:    *outputPath,
			outputDefault: *outputDefault,
			routePath:     *routePath,
			from:          *from,
			deploy:        *deploy,
			listen:        *deployListen,
			fastTrace:     *fastTraceFlag,
			file:          *file,
			disableMPLS:   *disableMPLS,
			noRDNS:        *norDNS,
			alwaysRDNS:    *alwaysrDNS,
			init:          *init,
			srcAddr:       *srcAddr,
			srcDev:        *srcDev,
		})); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := runNaliMode(rootCtx, naliRunOptions{
			stdin:     os.Stdin,
			stdout:    os.Stdout,
			dn42:      *dn42,
			data:      *dataOrigin,
			dot:       *dot,
			pow:       *powProvider,
			lang:      *lang,
			timeoutMs: *timeout,
			ipv4Only:  *ipv4Only,
			ipv6Only:  *ipv6Only,
			target:    *str,
		}); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}
	resolvedOutputPath, outputErr := resolveOutputPath(*outputPath, *outputDefault)
	if outputErr != nil {
		fmt.Println(outputErr)
		os.Exit(1)
	}
	if err := validateJSONRealtimeOutput(*jsonPrint, resolvedOutputPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if *mtuMode {
		conflictFlags := buildMTUConflictFlags(
			*tcp,
			*rawPrint,
			mtrModes,
			*tablePrint,
			*classicPrint,
			*routePath,
			*outputPath != "",
			*outputDefault,
			*deploy,
			enableGlobalping,
			*from,
			*file,
			*fastTraceFlag,
		)
		if conflict, ok := checkMTUConflicts(conflictFlags); !ok {
			fmt.Printf("--mtu 不能与 %s 同时使用\n", conflict)
			os.Exit(1)
		}
		if err := normalizeMTUProtocolFlags(tcp, udp); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	if mtrModes.mtr {
		conflictFlags := map[string]bool{
			"table":         *tablePrint,
			"classic":       *classicPrint,
			"json":          *jsonPrint,
			"output":        *outputPath != "",
			"outputDefault": *outputDefault,
			"routePath":     *routePath,
			"from":          enableGlobalping && *from != "",
			"fastTrace":     *fastTraceFlag,
			"file":          *file != "",
			"deploy":        enableWebUI && *deploy,
		}
		if conflict, ok := checkMTRConflicts(conflictFlags); !ok {
			fmt.Printf("--mtr 不能与 %s 同时使用\n", conflict)
			os.Exit(1)
		}
	}

	queriesExplicit, ttlTimeExplicit, packetSizeExplicit, tosExplicit := detectExplicitProbeFlags(parser)
	applyTTLIntervalDefault(ttlInterval, ttlTimeExplicit, mtrModes.mtr)
	osType := resolveOSType()
	stdoutIsTTY := CheckTTY(int(os.Stdout.Fd()))
	if shouldForceNoColorForMTUNonTTY(*mtuMode, *jsonPrint, stdoutIsTTY) {
		*noColor = true
	}
	if handleStartupModes(*noColor, *jsonPrint, mtrModes, *ver, *deploy, *deployListen, *init, osType) {
		return
	}
	if *speedMode {
		// maybeRunSpeedMode should handle --speed before parser.Parse runs. This
		// branch is a safety net for parser edge cases such as a "--" terminator.
		fmt.Fprintln(os.Stderr, "internal error: speed mode dispatch failed")
		os.Exit(1)
	}
	restoreFastIPOutput := setFastIPOutputSuppression(*jsonPrint || mtrModes.mtr)
	defer restoreFastIPOutput()

	if *tos < 0 || *tos > 255 {
		fmt.Println("--tos 必须在 0-255 之间")
		os.Exit(1)
	}

	applyDefaultPort(port, *udp)
	clampProbeSettings(*tcp, numMeasurements, maxAttempts)
	configureGeoDNS(*dot)

	if *mtuMode {
		if packetSizeExplicit {
			fmt.Println("--mtu 不支持 --psize")
			os.Exit(1)
		}
		if tosExplicit {
			fmt.Println("--mtu 不支持 --tos")
			os.Exit(1)
		}
		if !checkRuntimePrivileges(true) {
			os.Exit(1)
		}
		domain := resolveCLITargetOrExit(*str, sanitizeUsagePositionalArgs(parser.Usage(err)))
		if domain == "" {
			return
		}
		ip := lookupTargetIPOrExit(rootCtx, domain, *ipv4Only, *ipv6Only, *dot, *jsonPrint)
		// ResolveConfiguredSrcAddr is used for display/source-IP fallback before MTU-specific source normalization.
		resolvedSrcAddr, _, srcResolveErr := trace.ResolveConfiguredSrcAddr(ip, *srcAddr, *srcDev)
		if srcResolveErr != nil {
			fmt.Println(srcResolveErr)
			os.Exit(1)
		}
		// NormalizeExplicitSourceConfig applies explicit --source/--dev selection rules.
		sourceCfg, srcResolveErr := trace.NormalizeExplicitSourceConfig(trace.UDPTrace, trace.Config{
			OSType:       osType,
			DstIP:        ip,
			SrcAddr:      *srcAddr,
			SourceDevice: *srcDev,
		})
		if srcResolveErr != nil {
			fmt.Println(srcResolveErr)
			os.Exit(1)
		}
		if sourceCfg.SrcAddr != "" {
			resolvedSrcAddr = sourceCfg.SrcAddr
		}
		resolvedSrcDev := resolveMTUSourceDevice(osType, *srcAddr, *srcDev, sourceCfg.SourceDevice)
		srcIP, srcErr := resolveMTUSourceIP(ip, resolvedSrcAddr)
		if srcErr != nil {
			fmt.Println(srcErr)
			os.Exit(1)
		}
		leoWs := prepareRuntimeEnvironment(rootCtx, *dn42, dataOrigin, disableMaptrace, powProvider, false)
		defer closeLeoWebsocket(leoWs)
		conf := buildMTUTraceConfig(
			domain,
			ip,
			srcIP,
			resolvedSrcDev,
			*srcPort,
			*port,
			*beginHop,
			*maxHops,
			*numMeasurements,
			*timeout,
			*ttlInterval,
			!*norDNS,
			*alwaysrDNS,
			ipgeo.GetSource(*dataOrigin),
			*lang,
		)
		if err := runStandaloneMTUMode(conf, *jsonPrint); err != nil {
			if !errors.Is(err, context.Canceled) {
				fmt.Println(err)
			}
		}
		return
	}

	method := resolveTraceMethod(*tcp, *udp)
	paramsFastTrace := fastTrace.ParamsFastTrace{
		Context:        rootCtx,
		OSType:         osType,
		ICMPMode:       *icmpMode,
		SrcDev:         *srcDev,
		SrcAddr:        *srcAddr,
		DstPort:        *port,
		BeginHop:       *beginHop,
		MaxHops:        *maxHops,
		MaxAttempts:    *maxAttempts,
		RDNS:           !*norDNS,
		AlwaysWaitRDNS: *alwaysrDNS,
		Lang:           *lang,
		PktSize:        *packetSize,
		PacketSizeSet:  packetSizeExplicit,
		TOS:            *tos,
		Timeout:        time.Duration(*timeout) * time.Millisecond,
		File:           *file,
		Dot:            *dot,
		OutputPath:     resolvedOutputPath,
	}
	if maybeRunFastTraceMode(*from, *fastTraceFlag, *file, paramsFastTrace, method) {
		return
	}

	domain := resolveCLITargetOrExit(*str, sanitizeUsagePositionalArgs(parser.Usage(err)))
	if domain == "" {
		return
	}

	asyncLeo := shouldUseAsyncLeoForMTR(mtrModes, CheckTTY(int(os.Stdin.Fd())), stdoutIsTTY)
	leoWs := prepareRuntimeEnvironment(rootCtx, *dn42, dataOrigin, disableMaptrace, powProvider, asyncLeo)
	defer closeLeoWebsocket(leoWs)

	if *from != "" {
		if packetSizeExplicit {
			fmt.Println("Globalping 模式不支持 --psize")
			os.Exit(1)
		}
		if tosExplicit {
			fmt.Println("Globalping 模式不支持 --tos")
			os.Exit(1)
		}
	}

	if maybeHandleGlobalping(
		*from,
		&trace.GlobalpingOptions{
			Target:  *str,
			From:    *from,
			IPv4:    *ipv4Only,
			IPv6:    *ipv6Only,
			TCP:     *tcp,
			UDP:     *udp,
			Port:    *port,
			Packets: *numMeasurements,
			MaxHops: *maxHops,

			DisableMaptrace: *disableMaptrace,
			DataOrigin:      *dataOrigin,

			TablePrint:   *tablePrint,
			ClassicPrint: *classicPrint,
			RawPrint:     *rawPrint,
			JSONPrint:    *jsonPrint,
			ClearScreen:  stdoutIsTTY,
		},
		&trace.Config{
			Context:         rootCtx,
			OSType:          osType,
			DN42:            *dn42,
			NumMeasurements: *numMeasurements,
			Lang:            *lang,
			RDNS:            !*norDNS,
			AlwaysWaitRDNS:  *alwaysrDNS,
			IPGeoSource:     ipgeo.GetSource(*dataOrigin),
			Timeout:         time.Duration(*timeout) * time.Millisecond,
		},
	) {
		return
	}

	ip := lookupTargetIPOrExit(rootCtx, domain, *ipv4Only, *ipv6Only, *dot, *jsonPrint)

	// ResolveConfiguredSrcAddr is used for display/source-IP fallback before tracer runtime normalization.
	resolvedSrcAddr, _, srcResolveErr := trace.ResolveConfiguredSrcAddr(ip, *srcAddr, *srcDev)
	if srcResolveErr != nil {
		fmt.Println(srcResolveErr)
		os.Exit(1)
	}
	// NormalizeExplicitSourceConfig applies explicit --source/--dev selection rules.
	sourceCfg, srcResolveErr := trace.NormalizeExplicitSourceConfig(method, trace.Config{
		OSType:       osType,
		DstIP:        ip,
		SrcAddr:      *srcAddr,
		SourceDevice: *srcDev,
	})
	if srcResolveErr != nil {
		fmt.Println(srcResolveErr)
		os.Exit(1)
	}
	if sourceCfg.SrcAddr != "" {
		resolvedSrcAddr = sourceCfg.SrcAddr
	}
	resolvedSrcDev := sourceCfg.SourceDevice
	effectivePacketSize := resolvePacketSizeArg(*packetSize, packetSizeExplicit, method, ip)
	printTraceNav(*jsonPrint, mtrModes.mtr, ip, domain, *dataOrigin, *maxHops, effectivePacketSize, resolvedSrcAddr, method)

	packetSizeSpec, packetSizeErr := trace.NormalizePacketSize(method, ip, effectivePacketSize)
	if packetSizeErr != nil {
		fmt.Println(packetSizeErr)
		os.Exit(1)
	}

	util.SrcPort = *srcPort
	util.DstIP = ip.String()
	conf := buildTraceConfig(
		osType,
		*icmpMode,
		*dn42,
		resolvedSrcAddr,
		resolvedSrcDev,
		*srcPort,
		*beginHop,
		ip,
		*port,
		*maxHops,
		*packetInterval,
		*ttlInterval,
		*numMeasurements,
		*maxAttempts,
		*parallelRequests,
		*lang,
		*norDNS,
		*alwaysrDNS,
		*dataOrigin,
		*timeout,
		packetSizeSpec.PayloadSize,
		packetSizeSpec.Random,
		*tos,
		*disableMPLS,
	)
	conf.Context = rootCtx

	if maybeRunMTRMode(mtrModes, method, conf, queriesExplicit, *numMeasurements, ttlTimeExplicit, *ttlInterval, domain, *dataOrigin, *showIPs, *ipInfoMode) {
		return
	}

	outputCleanup, err := configureTracePrinters(&conf, *tablePrint, *classicPrint, *rawPrint, resolvedOutputPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if outputCleanup != nil {
		defer func() {
			if closeErr := outputCleanup(); closeErr != nil {
				fmt.Println(closeErr)
			}
		}()
	}
	applyJSONOutputMode(&conf, *jsonPrint)
	maybeRunUninterruptedRaw(*rawPrint, method, conf)

	res, ok := runTraceOnce(method, conf)
	if !ok {
		return
	}

	finalizeTraceResult(rootCtx, res, *tablePrint, stdoutIsTTY, *routePath, ip, *disableMaptrace, *jsonPrint, *dataOrigin)
}

type mtrRunMode int

const (
	mtrRunTUI mtrRunMode = iota
	mtrRunReport
	mtrRunRaw
)

func chooseMTRRunMode(effectiveMTRRaw, effectiveReport bool) mtrRunMode {
	if effectiveMTRRaw {
		return mtrRunRaw
	}
	if effectiveReport {
		return mtrRunReport
	}
	return mtrRunTUI
}

func shouldUseAsyncLeoForMTR(modes effectiveMTRModes, stdinTTY bool, stdoutTTY bool) bool {
	return modes.mtr && chooseMTRRunMode(modes.raw, modes.report) == mtrRunTUI && stdinTTY && stdoutTTY
}

// deriveMTRProbeParams computes per-hop scheduling parameters for MTR.
//
// maxPerHop priority: explicit -q > report default 10 > TUI/raw default 0 (unlimited).
// hopIntervalMs priority: explicit -i > default 1000.
func deriveMTRProbeParams(
	effectiveReport, queriesExplicit bool, numMeasurements int,
	ttlTimeExplicit bool, ttlInterval int,
) (maxPerHop int, hopIntervalMs int) {
	// maxPerHop
	if queriesExplicit {
		maxPerHop = numMeasurements
	} else if effectiveReport {
		maxPerHop = 10 // report 默认 10
	} else {
		maxPerHop = 0 // TUI/raw → 无限
	}

	// hopIntervalMs
	if ttlTimeExplicit {
		hopIntervalMs = ttlInterval
	} else {
		hopIntervalMs = 1000
	}
	return
}

// deriveMTRRoundParams is the legacy round-based parameter derivation.
// Kept for backward compatibility (Web MTR).
func deriveMTRRoundParams(effectiveReport, queriesExplicit bool, numMeasurements int, ttlTimeExplicit bool, ttlInterval int) (maxRounds int, intervalMs int) {
	if effectiveReport {
		if queriesExplicit {
			maxRounds = numMeasurements
		} else {
			maxRounds = 10 // report 默认 10 轮
		}
	} else if queriesExplicit {
		maxRounds = numMeasurements
	} else {
		maxRounds = 0 // 非 report → 无限
	}

	if ttlTimeExplicit {
		intervalMs = ttlInterval
	} else {
		intervalMs = 1000 // MTR 默认 1000ms
	}
	return
}

func capabilitiesCheck() {
	status := util.TracePrivilegeStatus(appBinName, false)
	if status.Message != "" {
		fmt.Println(status.Message)
	}
}

func checkRuntimePrivileges(requireWindowsAdmin bool) bool {
	status := util.TracePrivilegeStatus(appBinName, requireWindowsAdmin)
	if status.Message != "" {
		fmt.Println(status.Message)
	}
	return !status.Fatal
}
