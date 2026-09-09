package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
	domainLookupFn                    = util.DomainLookUpWithContext
	prepareNextTraceAPIV4FastIPFn     = ipgeo.PrepareNextTraceAPIV4FastIP
	newNextTraceAPIV3WebSocketFn      = wshandle.NewWithContext
	newNextTraceAPIV3WebSocketAsyncFn = wshandle.NewWithContextAsync
	runFastTraceFn                    = fastTrace.FastTest
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

func mcpEndpointURL(info listenInfo) string {
	endpoint := info.Access
	if endpoint == "" {
		endpoint = info.Binding
	}
	return strings.TrimRight(endpoint, "/") + "/mcp"
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

func normalizeDataProviderArgs(args []string) []string {
	normalized := append([]string(nil), args...)
	for i := 0; i < len(normalized); i++ {
		switch {
		case normalized[i] == "--":
			return normalized
		case normalized[i] == "-d" || normalized[i] == "--data-provider":
			if i+1 < len(normalized) {
				normalized[i+1] = ipgeo.CanonicalizeNextTraceAPIProvider(normalized[i+1])
				i++
			}
		case strings.HasPrefix(normalized[i], "-d="):
			value := strings.TrimPrefix(normalized[i], "-d=")
			normalized[i] = "-d=" + ipgeo.CanonicalizeNextTraceAPIProvider(value)
		case strings.HasPrefix(normalized[i], "--data-provider="):
			value := strings.TrimPrefix(normalized[i], "--data-provider=")
			normalized[i] = "--data-provider=" + ipgeo.CanonicalizeNextTraceAPIProvider(value)
		}
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
	deployToken  *string
	mcp          *bool
	deploy       *bool
}

type deployRunOptions struct {
	ListenAddr  string
	EnableMCP   bool
	AuthEnabled bool
	DeployToken string
}

type mtrCLIFlags struct {
	columns    *string
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
	if enableTraceroute {
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

func registerDataProviderFlag(parser *argparse.Parser) *string {
	return parser.Selector("d", "data-provider", []string{"IP.SB", "ip.sb", "IPInfo", "ipinfo", "IPInsight", "ipinsight", "IPAPI.com", "ip-api.com", "IPInfoLocal", "ipinfolocal", "chunzhen", ipgeo.NextTraceAPIProvider, "ipdb.one", "disable-geoip", "DN42", "dn42"}, &argparse.Options{
		Default: ipgeo.NextTraceAPIProvider,
		Help:    "Choose IP Geographic Data Provider [NextTrace-API, IP.SB, IPInfo, IPInsight, IP-API.com, IPInfoLocal, ipdb.one, chunzhen, disable-geoip, DN42]",
	})
}

func registerQueriesFlag(parser *argparse.Parser) *int {
	queries := parser.Int("q", "queries", &argparse.Options{Help: buildQueriesHelp()})
	// Keep the traditional fallback without argparse advertising it for every mode.
	// MTR applies its own defaults when this flag was not explicitly parsed.
	*queries = 3
	return queries
}

func buildQueriesHelp() string {
	help := "MTR only: max probes per hop."
	if enableTraceroute {
		help = "Traceroute: latency samples per hop (default 3). MTR: max probes per hop."
	}
	return help + " 0 = unlimited in TUI/raw/JSON streams. JSON reports require a positive count. When omitted: 10 with --report/--wide (including --raw), otherwise unlimited"
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
	return "Set the full 8-bit IP type-of-service / traffic class [0-255]: DSCP*4+ECN (DSCP 46, ECN 0 = 184)"
}

func registerTracerouteOutputFlags(parser *argparse.Parser) tracerouteOutputFlags {
	return registerTracerouteOutputFlagsWithAvailability(parser, enableTraceroute)
}

func registerTracerouteOutputFlagsWithAvailability(parser *argparse.Parser, enabled bool) tracerouteOutputFlags {
	jsonPrint := parser.Flag("j", "json", &argparse.Options{Help: "Output JSON; with MTR, stream NDJSON unless --report/--wide is selected"})
	if enabled {
		return tracerouteOutputFlags{
			routePath:     parser.Flag("P", "route-path", &argparse.Options{Help: "Print traceroute hop path by ASN and location"}),
			outputPath:    parser.String("o", "output", &argparse.Options{Help: "Write realtime trace output and final stop reason to FILE"}),
			outputDefault: parser.Flag("O", "output-default", &argparse.Options{Help: "Write realtime trace output and final stop reason to the default log file (/tmp/trace.log)"}),
			tablePrint:    parser.Flag("", "table", &argparse.Options{Help: "Output trace results as a final summary table (traceroute report mode)"}),
			jsonPrint:     jsonPrint,
			classicPrint:  parser.Flag("c", "classic", &argparse.Options{Help: "Classic Output trace results like BestTrace"}),
		}
	}
	return tracerouteOutputFlags{
		routePath:     ptrBool(false),
		outputPath:    ptrStr(""),
		outputDefault: ptrBool(false),
		tablePrint:    ptrBool(false),
		jsonPrint:     jsonPrint,
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
			deployToken:  parser.String("", "deploy-token", &argparse.Options{Help: "Set bearer token for --deploy WebUI/API/WebSocket/MCP access"}),
			mcp:          parser.Flag("", "mcp", &argparse.Options{Help: "Enable MCP endpoint under --deploy at /mcp"}),
			deploy:       parser.Flag("", "deploy", &argparse.Options{Help: "Start the Gin powered web console"}),
		}
	}
	return webUIFlags{
		deployListen: ptrStr(""),
		deployToken:  ptrStr(""),
		mcp:          ptrBool(false),
		deploy:       ptrBool(false),
	}
}

func registerPacketIntervalFlag(parser *argparse.Parser) *int {
	if enableTraceroute {
		return parser.Int("z", "send-time", &argparse.Options{Default: defaultPacketIntervalMs, Help: buildPacketIntervalHelp()})
	}
	return ptrInt(defaultPacketIntervalMs)
}

func buildRawHelp() string {
	if !enableTraceroute {
		return "MTR streaming raw event output"
	}
	rawHelp := "Machine-friendly output; use --traceroute --raw to preserve traditional raw output"
	if enableMTR {
		mtrFlags := "--mtr/-r/-w"
		rawHelp += ". With MTR (" + mtrFlags + "), enables streaming raw event mode"
	}
	return rawHelp
}

func buildTTLIntervalHelp() string {
	if !enableMTR {
		return "Advanced: TTL-group interval [ms] in normal traceroute. 100-300ms is usually safe; lower is faster but may trigger rate limits"
	}
	if !enableTraceroute {
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
	if enableTraceroute {
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

func validateGlobalpingAvailability(from string, enabled bool) error {
	if from != "" && !enabled {
		return fmt.Errorf("--from (Globalping) is not available in %s; please use the full nexttrace build", appBinName)
	}
	return nil
}

func registerMTRFlags(parser *argparse.Parser) mtrCLIFlags {
	if enableMTR {
		mtrMode := ptrBool(false)
		if enableTraceroute {
			mtrMode = parser.Flag("t", "mtr", &argparse.Options{Help: "Enable MTR (My Traceroute) continuous probing mode"})
		}
		return mtrCLIFlags{
			mtrMode:    mtrMode,
			columns:    parser.String("", "mtr-columns", &argparse.Options{Help: "MTR text columns in order: loss, snt, received, last, avg, best, wrst, stdev, dropped, gmean, jitter, javg, jmax, jint, space (does not enable MTR)"}),
			reportMode: parser.Flag("r", "report", &argparse.Options{Help: "MTR report mode (non-interactive, implies --mtr); can trigger MTR without --mtr"}),
			wideMode:   parser.Flag("w", "wide", &argparse.Options{Help: "MTR wide report mode (implies --mtr --report); alone equals --mtr --report --wide"}),
			showIPs:    parser.Flag("", "show-ips", &argparse.Options{Help: "MTR only: display both PTR hostnames and numeric IPs (PTR first, IP in parentheses)"}),
			ipInfoMode: parser.Int("y", "ipinfo", &argparse.Options{Default: 0, Help: "Set initial MTR TUI host info mode (0-4). TUI only; ignored in --report/--raw/--json. 0:IP/PTR 1:ASN 2:City 3:Owner 4:Full"}),
		}
	}
	return mtrCLIFlags{
		mtrMode:    ptrBool(false),
		columns:    new(string),
		reportMode: ptrBool(false),
		wideMode:   ptrBool(false),
		showIPs:    ptrBool(false),
		ipInfoMode: ptrInt(0),
	}
}

func registerFileFlag(parser *argparse.Parser) *string {
	if enableTraceroute {
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

type deployAuthPlan struct {
	Enabled       bool
	Token         string
	AutoGenerated bool
}

func maybeRunDeployMode(deploy bool, deployListen string, enableMCP bool, deployToken string) bool {
	if !deploy {
		return false
	}
	if !enableWebUI {
		if err := runDeploy(deployRunOptions{}, nil); err != nil {
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
	authPlan, err := resolveDeployAuthPlan(listenAddr, deployToken)
	if err != nil {
		if util.EnvDevMode {
			panic(err)
		}
		log.Fatal(err)
	}

	onReady := func(addr net.Addr) {
		info := buildListenInfo(addr.String())
		fmt.Printf("启动 NextTrace Web 控制台，监听地址: %s\n", info.Binding)
		if enableMCP {
			fmt.Printf("MCP Endpoint: %s\n", mcpEndpointURL(info))
		}
		if authPlan.Enabled {
			if authPlan.AutoGenerated {
				fmt.Printf("Deploy token: %s\n", authPlan.Token)
			} else {
				fmt.Println("Deploy token 鉴权已启用")
			}
		} else {
			fmt.Println("Deploy token 鉴权未启用（仅限本机监听的默认行为）")
		}
		if !userProvided {
			fmt.Println("远程访问请显式设置 --listen（例如 --listen 0.0.0.0:1080）。")
		}
		if info.Access != "" && info.Access != info.Binding {
			fmt.Printf("如需远程访问，请尝试: %s\n", info.Access)
		}
		fmt.Println("注意：Web 控制台的安全性有限，请在确保安全的前提下使用，如有必要请使用ACL等方式加强安全性")
	}
	if err := runDeploy(deployRunOptions{
		ListenAddr:  listenAddr,
		EnableMCP:   enableMCP,
		AuthEnabled: authPlan.Enabled,
		DeployToken: authPlan.Token,
	}, onReady); err != nil {
		if util.EnvDevMode {
			panic(err)
		}
		log.Fatal(err)
	}
	return true
}

func resolveDeployAuthPlan(listenAddr, cliToken string) (deployAuthPlan, error) {
	token := strings.TrimSpace(cliToken)
	if token == "" {
		token = strings.TrimSpace(util.EnvDeployToken)
	}
	if token != "" {
		return deployAuthPlan{Enabled: true, Token: token}, nil
	}
	if deployListenRequiresToken(listenAddr) {
		generated, err := generateDeployToken()
		if err != nil {
			return deployAuthPlan{}, err
		}
		return deployAuthPlan{Enabled: true, Token: generated, AutoGenerated: true}, nil
	}
	return deployAuthPlan{}, nil
}

func generateDeployToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate deploy token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func deployListenRequiresToken(listenAddr string) bool {
	host, _, ok := splitListenAddr(normalizeListenAddr(listenAddr))
	if !ok {
		return true
	}
	if host == "" {
		return true
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

func handleStartupModes(noColor, jsonPrint bool, modes effectiveMTRModes, ver, deploy bool, deployListen string, enableMCP bool, deployToken string, init bool, osType int) bool {
	applyColorMode(noColor)
	printStartupBanner(jsonPrint, modes.mtr)
	if maybePrintVersion(ver) {
		return true
	}
	if maybeRunDeployMode(deploy, deployListen, enableMCP, deployToken) {
		return true
	}
	return maybePrepareWinDivert(init, osType)
}

func validateDeployMCPMode(deploy, mcp bool) error {
	if mcp && !deploy {
		return errors.New("--mcp 必须与 --deploy 同时使用")
	}
	return nil
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
	runFastTraceFn(method, params)
	if params.OutputPath != "" {
		fmt.Printf("您的追踪日志已经存放在 %s 中\n", params.OutputPath)
	}
	return true
}

func runFastTraceModeWithRuntime(ctx context.Context, dn42 bool, dataOrigin *string, disableMaptrace *bool, powProvider *string, from string, fastTraceFlag bool, file string, params fastTrace.ParamsFastTrace, method trace.Method) bool {
	if from != "" || (!fastTraceFlag && file == "") {
		return false
	}
	nextTraceAPIV3WS, runtimePrepared := prepareFastTraceRuntimeEnvironment(ctx, dn42, dataOrigin, disableMaptrace, powProvider)
	defer closeNextTraceAPIV3WebSocket(nextTraceAPIV3WS)
	params.RuntimePrepared = runtimePrepared
	params.DataProvider = *dataOrigin
	descriptor := ipgeo.GetSourceDescriptorWithGeoDNS(*dataOrigin, params.Dot)
	params.IPGeoSource = trace.CachedGeoSource(descriptor)
	params.IPGeoDescriptor = func() ipgeo.SourceDescriptor { return descriptor }
	params.DN42 = isDN42Provider(*dataOrigin)
	return maybeRunFastTraceMode(from, fastTraceFlag, file, params, method)
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
	if !enabled && !isDN42Provider(*dataOrigin) {
		return
	}
	applyDN42DataOrigin(dataOrigin)
	*disableMaptrace = true
}

func isDN42Provider(provider string) bool {
	return strings.EqualFold(strings.TrimSpace(provider), "DN42")
}

func applyDN42DataOrigin(dataOrigin *string) {
	config.InitConfig()
	*dataOrigin = "DN42"
}

func prepareRuntimeEnvironment(ctx context.Context, dn42 bool, dataOrigin *string, disableMaptrace *bool, powProvider *string, asyncNextTraceAPIV3 bool) *wshandle.WsConn {
	capabilitiesCheck()
	dn42Configured := dn42 || isDN42Provider(*dataOrigin)
	applyDN42Mode(dn42, dataOrigin, disableMaptrace)
	conn := initNextTraceAPIV3WebSocket(ctx, dataOrigin, powProvider, asyncNextTraceAPIV3)
	if !dn42Configured {
		applyDN42Mode(false, dataOrigin, disableMaptrace)
	}
	return conn
}

func prepareFastTraceRuntimeEnvironment(ctx context.Context, dn42 bool, dataOrigin *string, disableMaptrace *bool, powProvider *string) (*wshandle.WsConn, bool) {
	capabilitiesCheck()
	dn42Configured := dn42 || isDN42Provider(*dataOrigin)
	applyDN42Mode(dn42, dataOrigin, disableMaptrace)
	conn, prepared := initNextTraceAPIRuntime(ctx, dataOrigin, powProvider, false)
	if !dn42Configured {
		applyDN42Mode(false, dataOrigin, disableMaptrace)
	}
	return conn, prepared
}

func initNextTraceAPIV3WebSocket(ctx context.Context, dataOrigin, powProvider *string, async bool) *wshandle.WsConn {
	nextTraceAPIV3WS, _ := initNextTraceAPIRuntime(ctx, dataOrigin, powProvider, async)
	return nextTraceAPIV3WS
}

func initNextTraceAPIRuntime(ctx context.Context, dataOrigin, powProvider *string, async bool) (*wshandle.WsConn, bool) {
	*dataOrigin = ipgeo.CanonicalizeNextTraceAPIProvider(*dataOrigin)
	if !ipgeo.IsNextTraceAPIProvider(*dataOrigin) {
		return nil, false
	}
	if !strings.EqualFold(*powProvider, "api.nxtrace.org") {
		util.PowProviderParam = *powProvider
	}
	if util.EnvDataProvider != "" {
		*dataOrigin = ipgeo.CanonicalizeNextTraceAPIProvider(util.EnvDataProvider)
	}
	if !ipgeo.IsNextTraceAPIProvider(*dataOrigin) {
		return nil, false
	}
	if ipgeo.NextTraceAPIV4TokenConfigured() {
		if err := prepareNextTraceAPIV4FastIPFn(ctx, true); err == nil {
			return nil, true
		}
	}

	var nextTraceAPIV3WS *wshandle.WsConn
	if async {
		nextTraceAPIV3WS = newNextTraceAPIV3WebSocketAsyncFn(ctx)
	} else {
		nextTraceAPIV3WS = newNextTraceAPIV3WebSocketFn(ctx)
	}
	return nextTraceAPIV3WS, nextTraceAPIV3WS != nil
}

func closeNextTraceAPIV3WebSocket(nextTraceAPIV3WS *wshandle.WsConn) {
	if nextTraceAPIV3WS != nil {
		nextTraceAPIV3WS.Close()
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

func isContextStop(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func lookupTargetIPOrExit(ctx context.Context, domain string, ipv4Only, ipv6Only bool, dot string, jsonPrint bool) (net.IP, bool) {
	ip, err := lookupTargetIP(ctx, domain, ipv4Only, ipv6Only, dot, jsonPrint)
	if err != nil {
		if isContextStop(err) {
			return nil, false
		}
		if util.EnvDevMode {
			panic(err)
		}
		log.Fatal(err)
	}
	return ip, true
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
	dn42 = dn42 || isDN42Provider(dataOrigin)
	descriptorSession := ipgeo.GetSourceDescriptorSession(dataOrigin)
	session := trace.CachedGeoSourceSession(descriptorSession)
	return trace.Config{
		OSType:             osType,
		ICMPMode:           icmpMode,
		DN42:               dn42,
		SrcAddr:            srcAddr,
		SrcPort:            srcPort,
		SourceDevice:       strings.TrimSpace(sourceDevice),
		BeginHop:           beginHop,
		DstIP:              ip,
		DstPort:            port,
		MaxHops:            maxHops,
		PacketInterval:     packetInterval,
		TTLInterval:        ttlInterval,
		NumMeasurements:    numMeasurements,
		MaxAttempts:        maxAttempts,
		ParallelRequests:   parallelRequests,
		Lang:               lang,
		RDNS:               !noRDNS,
		AlwaysWaitRDNS:     alwaysRDNS,
		IPGeoSource:        session.Source,
		IPGeoDescriptor:    descriptorSession.Current,
		RefreshIPGeoSource: session.Refresh,
		Timeout:            time.Duration(timeout) * time.Millisecond,
		PktSize:            packetSize,
		RandomPacketSize:   randomPacketSize,
		TOS:                tos,
		DisableMPLS:        disableMPLS,
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
	columns ...printer.MTRColumn,
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

	var err error
	switch chooseMTRRunMode(modes.raw, modes.report) {
	case mtrRunRaw:
		err = runMTRRaw(method, conf, mtrHopIntervalMs, mtrMaxPerHop, dataOrigin, nil)
	case mtrRunReport:
		err = runMTRReport(method, conf, mtrHopIntervalMs, mtrMaxPerHop, domain, dataOrigin, modes.wide, showIPs, nil, columns...)
	default:
		if ipInfoMode < 0 || ipInfoMode > 4 {
			fmt.Fprintf(os.Stderr, "--ipinfo/-y 必须在 0-4 范围内，当前值: %d\n", ipInfoMode)
			os.Exit(1)
		}
		err = runMTRTUI(method, conf, mtrHopIntervalMs, mtrMaxPerHop, domain, dataOrigin, showIPs, ipInfoMode, nil, columns...)
	}
	// Mode-local defers restore the terminal and close listeners before exit.
	exitOnTraceRunError(err)
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

func setFastIPOutputSuppression(suppress bool) func() {
	prev := util.SuppressFastIPOutput
	util.SuppressFastIPOutput = suppress
	return func() {
		util.SuppressFastIPOutput = prev
	}
}

type traceOutputMode uint8

const (
	traceOutputRealtime traceOutputMode = iota
	traceOutputFile
	traceOutputRaw
	traceOutputClassic
	traceOutputTable
	traceOutputJSON
)

func selectTraceOutputMode(tablePrint, classicPrint, jsonPrint, rawPrint bool, outputPath string) traceOutputMode {
	switch {
	case jsonPrint:
		return traceOutputJSON
	case tablePrint:
		return traceOutputTable
	case classicPrint:
		return traceOutputClassic
	case rawPrint:
		return traceOutputRaw
	case strings.TrimSpace(outputPath) != "":
		return traceOutputFile
	default:
		return traceOutputRealtime
	}
}

func (mode traceOutputMode) printsStopReason() bool {
	return mode == traceOutputRealtime || mode == traceOutputFile || mode == traceOutputTable
}

func (mode traceOutputMode) label() string {
	switch mode {
	case traceOutputRaw:
		return "raw"
	case traceOutputClassic:
		return "classic"
	case traceOutputTable:
		return "table"
	case traceOutputJSON:
		return "JSON"
	default:
		return ""
	}
}

func writeIgnoredTraceOutputWarning(w io.Writer, mode traceOutputMode, outputPath string) error {
	if strings.TrimSpace(outputPath) == "" || mode == traceOutputFile {
		return nil
	}
	label := mode.label()
	if label == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "warning: trace output file is ignored because %s output has higher priority\n", label)
	return err
}

type traceOutputPlan struct {
	mode traceOutputMode
	file io.WriteCloser
}

func (plan *traceOutputPlan) printStopReason(reason *trace.StopReason) error {
	if plan == nil || !plan.mode.printsStopReason() {
		return nil
	}
	terminalErr := printer.PrintTraceStopReason(reason)
	var fileErr error
	if plan.mode == traceOutputFile && plan.file != nil {
		fileErr = printer.WriteTraceStopReason(plan.file, reason)
	}
	return errors.Join(terminalErr, fileErr)
}

func (plan *traceOutputPlan) close() error {
	if plan == nil || plan.file == nil {
		return nil
	}
	return plan.file.Close()
}

func configureTracePrinters(conf *trace.Config, mode traceOutputMode, outputPath string) (*traceOutputPlan, error) {
	plan := &traceOutputPlan{mode: mode}
	switch mode {
	case traceOutputJSON:
		conf.RealtimePrinter = nil
		conf.AsyncPrinter = nil
	case traceOutputTable:
		conf.RealtimePrinter = nil
	case traceOutputClassic:
		conf.RealtimePrinter = printer.ClassicPrinter
	case traceOutputRaw:
		conf.RealtimePrinter = printer.EasyPrinter
	case traceOutputFile:
		f, err := tracelog.OpenFile(outputPath)
		if err != nil {
			return nil, err
		}
		conf.RealtimePrinter = func(res *trace.Result, ttl int) {
			printer.RealtimePrinter(res, ttl)
			if err := tracelog.WriteRealtime(f, res, ttl); err != nil {
				log.Printf("write trace output: %v", err)
			}
		}
		plan.file = f
	case traceOutputRealtime:
		conf.RealtimePrinter = printer.RealtimePrinter
	}
	return plan, nil
}

func maybeRunUninterruptedRaw(rawPrint bool, method trace.Method, conf trace.Config) bool {
	if !(util.Uninterrupted && rawPrint) {
		return false
	}
	for {
		if conf.Context != nil {
			if err := conf.Context.Err(); err != nil {
				return true
			}
		}
		if _, err := trace.Traceroute(method, conf); err != nil {
			if errors.Is(err, context.Canceled) {
				return true
			}
			if trace.IsInitializationError(err) {
				exitOnTraceRunError(err)
			}
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
	}
}

func runTraceOnce(method trace.Method, conf trace.Config) (*trace.Result, bool) {
	res, err := trace.Traceroute(method, conf)
	if err != nil {
		exitOnTraceRunError(err)
		return nil, false
	}
	return res, true
}

func exitOnTraceRunError(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// traceMapPayload preserves the historical external tracemap request shape.
type traceMapPayload struct {
	Hops        [][]trace.Hop `json:"Hops"`
	TraceMapURL string        `json:"TraceMapUrl"`
}

func marshalTraceMapPayload(res *trace.Result) ([]byte, error) {
	return json.Marshal(traceMapPayload{
		Hops:        res.Hops,
		TraceMapURL: res.TraceMapUrl,
	})
}

func finalizeTraceResult(ctx context.Context, res *trace.Result, outputPlan *traceOutputPlan, tableClearScreen, routePath bool, dstIP net.IP, disableMaptrace bool, dataOrigin string) {
	if outputPlan == nil {
		outputPlan = &traceOutputPlan{mode: traceOutputRealtime}
	}
	if outputPlan.mode == traceOutputTable {
		printer.TracerouteTablePrinter(res, tableClearScreen)
	}
	if routePath {
		reporter.New(res, dstIP.String()).Print()
	}
	if err := outputPlan.printStopReason(res.StopReason); err != nil {
		log.Printf("print trace stop reason: %v", err)
	}

	r, err := marshalTraceMapPayload(res)
	if err != nil {
		fmt.Println(err)
		return
	}
	if !disableMaptrace && supportsMapTrace(dataOrigin) {
		url, err := tracemap.GetMapUrlWithContext(ctx, string(r))
		if err != nil {
			fmt.Println(err)
			return
		}
		res.TraceMapUrl = url
		if outputPlan.mode != traceOutputJSON {
			tracemap.PrintMapUrl(url)
		}
	}
	r, err = json.Marshal(res)
	if err != nil {
		fmt.Println(err)
		return
	}
	if outputPlan.mode == traceOutputJSON {
		fmt.Println(string(r))
	}
}

func supportsMapTrace(dataOrigin string) bool {
	if ipgeo.IsNextTraceAPIProvider(dataOrigin) {
		return true
	}
	return util.StringInSlice(strings.ToUpper(dataOrigin), []string{"IPINFO", "IP-API.COM", "IPAPI.COM"})
}

func Execute() {
	if handled, exitCode := maybeRunMTRReplayMode(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(exitCode)
	}
	if handled, exitCode := maybeRunDoctorMode(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(exitCode)
	}
	mtrJSONRequested := requestsMTRJSON(os.Args[1:])
	mtrRecordingRequested := containsMTRRecordFlag(os.Args[1:])
	if !mtrJSONRequested && !mtrRecordingRequested {
		if handled, exitCode := maybeRunDNSMode(os.Args[1:], os.Stdout, os.Stderr); handled {
			os.Exit(exitCode)
		}
		if handled, exitCode := maybeRunSpeedMode(os.Args[1:], os.Stdout, os.Stderr); handled {
			os.Exit(exitCode)
		}
	}
	parser := argparse.NewParser(appBinName, "An open source visual route tracking CLI tool")
	// Override HelpFunc so positional arg names are sanitized in --help output
	parser.HelpFunc = func(c *argparse.Command, msg interface{}) string {
		return sanitizeUsagePositionalArgs(c.Usage(msg)) + "\n  --doctor  Check local probe prerequisites without sending probes; see --doctor --help\n  --mtr-replay FILE  Open a saved MTR session offline; see --mtr-replay --help\n"
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
	numMeasurements := registerQueriesFlag(parser)
	maxAttempts := parser.Int("", "max-attempts", &argparse.Options{Help: buildMaxAttemptsHelp()})
	parallelRequests := parser.Int("", "parallel-requests", &argparse.Options{Default: 18, Help: buildParallelRequestsHelp()})
	maxHops := parser.Int("m", "max-hops", &argparse.Options{Default: 30, Help: "Set the max number of hops (max TTL to be reached)"})
	dataOrigin := registerDataProviderFlag(parser)
	powProvider := parser.Selector("", "pow-provider", []string{"api.nxtrace.org", "sakura"}, &argparse.Options{Default: "api.nxtrace.org",
		Help: "Choose PoW Provider for NextTrace API v3 [api.nxtrace.org, sakura] For China mainland users, please use sakura"})
	norDNS := parser.Flag("n", "no-rdns", &argparse.Options{Help: "Do not resolve IP addresses to their domain names"})
	alwaysrDNS := parser.Flag("a", "always-rdns", &argparse.Options{Help: "Always resolve IP addresses to their domain names"})
	tracerouteMode := registerTracerouteFlag(parser)
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
	setupNextTraceAPIV4Token := parser.Flag("x", "setup-api-v4-token", &argparse.Options{Help: "Store a session-only NextTrace API v4 token in a temporary file"})
	dnsMode := registerDNSFlag(parser)
	speedMode := registerSpeedFlag(parser)
	naliMode := registerNaliFlag(parser)
	srcAddr := parser.String("s", "source", &argparse.Options{Help: "Use source address src_addr for outgoing packets"})
	fwmark := parser.String("", "fwmark", &argparse.Options{Help: "Linux probe socket mark (decimal or 0x hex); traceroute/MTR only"})
	srcPort := parser.Int("", "source-port", &argparse.Options{Help: "Use source port src_port for outgoing packets"})
	srcDev := parser.String("D", "dev", &argparse.Options{Help: "Use the specified network device for explicit source selection. On Windows, this selects the device source address; routing may still choose the egress interface"})

	webFlags := registerWebUIFlags(parser)
	deployListen := webFlags.deployListen
	deployToken := webFlags.deployToken
	deployMCP := webFlags.mcp
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

	// ── MTR flags (all flavors) ──
	mtrFlags := registerMTRFlags(parser)
	mtrRecord := parser.String("", "mtr-record", &argparse.Options{Help: "Save this MTR session to a new file for offline replay"})
	mtrMode := mtrFlags.mtrMode
	reportMode := mtrFlags.reportMode
	wideMode := mtrFlags.wideMode
	showIPs := mtrFlags.showIPs
	ipInfoMode := mtrFlags.ipInfoMode

	// ── File: hidden in MTR-only ntr ──
	file := registerFileFlag(parser)
	str := parser.StringPositional(&argparse.Options{Help: "Trace target: IPv4 address (e.g. 8.8.8.8), IPv6 address (e.g. 2001:db8::1), domain name (e.g. example.com), or URL (e.g. https://example.com)"})

	err := parser.Parse(normalizeNegativePacketSizeArgs(normalizeDataProviderArgs(os.Args)))
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		if mtrJSONRequested || mtrRecordingRequested || containsFWMarkFlag(os.Args[1:]) {
			fmt.Fprint(os.Stderr, sanitizeUsagePositionalArgs(parser.Usage(err)))
			os.Exit(2)
		}
		fmt.Print(sanitizeUsagePositionalArgs(parser.Usage(err)))
		return
	}
	fwmarkSet := parsedFlag(parser, "fwmark")
	mark, markErr := parseFWMark(*fwmark, fwmarkSet)
	if markErr == nil {
		markErr = validateFWMarkMode(fwmarkSet, map[string]bool{
			"--init": *init, "--mtu": *mtuMode, "--nali": *naliMode,
			"--deploy": *deploy, "--mcp": *deployMCP, "--dns": *dnsMode,
			"--speed": *speedMode, "--fast-trace": *fastTraceFlag,
			"--file": *file != "", "--from": *from != "",
			"--setup-api-v4-token": *setupNextTraceAPIV4Token,
		})
	}
	if markErr != nil {
		fmt.Fprintln(os.Stderr, markErr)
		os.Exit(2)
	}
	if mtrJSONRequested && (*setupNextTraceAPIV4Token || *dnsMode || *speedMode) {
		fmt.Fprintln(os.Stderr, "MTR JSON cannot be combined with a standalone mode")
		os.Exit(2)
	}
	if mtrRecordingRequested && (*setupNextTraceAPIV4Token || *dnsMode || *speedMode) {
		fmt.Fprintln(os.Stderr, "--mtr-record cannot be combined with a standalone mode")
		os.Exit(2)
	}
	if *dnsMode {
		fmt.Fprintln(os.Stderr, "-l/--dns must be the first argument")
		os.Exit(1)
	}
	if err := validateGlobalpingAvailability(*from, enableGlobalping); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if mtrJSONRequested || mtrRecordingRequested {
			os.Exit(2)
		}
		os.Exit(1)
	}
	if *setupNextTraceAPIV4Token && parsedFlag(parser, "mtr-columns") {
		fmt.Fprintln(os.Stderr, "--mtr-columns cannot be combined with token setup")
		os.Exit(2)
	}
	if *setupNextTraceAPIV4Token {
		if err := runNextTraceAPIV4TokenSetup(nextTraceAPIV4TokenSetupOptions{
			stdin:  os.Stdin,
			stdout: os.Stdout,
			stderr: os.Stderr,
		}); err != nil {
			os.Exit(handleNextTraceAPIV4TokenSetupError(os.Stderr, err))
		}
		return
	}
	util.SrcDev = ""

	standalone := ""
	switch {
	case *mtuMode:
		standalone = "--mtu"
	case *naliMode:
		standalone = "--nali"
	case *deploy:
		standalone = "--deploy"
	}
	mtrModes, modeErr := resolveTraceModes(traceModeOptions{
		traceroute: *tracerouteMode,
		mtr:        *mtrMode,
		report:     *reportMode,
		wide:       *wideMode,
		raw:        *rawPrint,
		traditional: (*jsonPrint && enableTraceroute) || *tablePrint || *classicPrint || *outputPath != "" ||
			*outputDefault || *routePath || *fastTraceFlag || *file != "" || *from != "",
		standalone: standalone,
	}, defaultMTR)
	if modeErr != nil {
		fmt.Fprintln(os.Stderr, modeErr)
		if mtrJSONRequested || mtrRecordingRequested {
			os.Exit(2)
		}
		os.Exit(1)
	}
	columnsStandalone := standalone
	if *init {
		columnsStandalone = "--init"
	}
	if err := validateMTRRecordMode(*mtrRecord, parsedFlag(parser, "mtr-record"), mtrModes, columnsStandalone, *deployMCP); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	mtrColumns, columnsErr := resolveMTRColumns(*mtrFlags.columns, parsedFlag(parser, "mtr-columns"), mtrModes, *jsonPrint, columnsStandalone)
	if columnsErr != nil {
		fmt.Fprintln(os.Stderr, columnsErr)
		os.Exit(2)
	}
	if mtrModes.mtr && (*jsonPrint || mtrRecordingRequested) {
		if maybePrintVersion(*ver) {
			return
		}
		conflicts := map[string]bool{
			"table": *tablePrint, "classic": *classicPrint, "output": *outputPath != "", "outputDefault": *outputDefault,
			"routePath": *routePath, "from": *from != "", "fastTrace": *fastTraceFlag, "file": *file != "", "deploy": *deploy,
		}
		conflict, ok := checkMTRConflicts(conflicts)
		if !ok || (*rawPrint && *jsonPrint) || *mtuMode || *naliMode || *init || *deployMCP {
			if ok {
				conflict = "--raw or a standalone mode"
			}
			fmt.Fprintf(os.Stderr, "MTR cannot be combined with %s\n", conflict)
			os.Exit(2)
		}
		queriesSet, intervalSet, packetSet, _ := detectExplicitProbeFlags(parser)
		count, interval := deriveMTRProbeParams(mtrModes.report, queriesSet, *numMeasurements, intervalSet, *ttlInterval)
		opts := mtrJSONOptions{
			Target: *str, Method: resolveTraceMethod(*tcp, *udp), Report: mtrModes.report,
			RecordPath:    *mtrRecord,
			RecordDisplay: mtrRecordingDisplay{hostMode: *ipInfoMode, showIPs: *showIPs, wide: mtrModes.wide, columns: *mtrFlags.columns},
			MaxPerHop:     count, HopIntervalMs: interval, PacketSize: *packetSize, PacketSizeExplicit: packetSet,
			IPv4Only: *ipv4Only, IPv6Only: *ipv6Only, DataProvider: *dataOrigin, PowProvider: *powProvider, DotServer: *dot,
			Config: trace.Config{
				OSType: resolveOSType(), ICMPMode: *icmpMode, DN42: *dn42,
				SrcAddr: *srcAddr, SourceDevice: *srcDev, SrcPort: *srcPort, DstPort: *port,
				FWMark: mark, FWMarkSet: fwmarkSet,
				BeginHop: *beginHop, MaxHops: *maxHops, ParallelRequests: *parallelRequests,
				Timeout: time.Duration(*timeout) * time.Millisecond, TOS: *tos, Lang: *lang,
				RDNS: !*norDNS, AlwaysWaitRDNS: *alwaysrDNS, DisableMPLS: *disableMPLS,
			},
		}
		var code int
		if *jsonPrint {
			code = runMTRJSONCLI(opts)
		} else {
			applyColorMode(*noColor)
			code = runMTRRecordedCLI(opts, mtrModes, *showIPs, *ipInfoMode, *mtrFlags.columns, mtrColumns)
		}
		os.Exit(code)
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
	outputMode := selectTraceOutputMode(*tablePrint, *classicPrint, *jsonPrint, *rawPrint, resolvedOutputPath)
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
			"from":          *from != "",
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
	if err := validateDeployMCPMode(*deploy, *deployMCP); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if handleStartupModes(*noColor, *jsonPrint, mtrModes, *ver, *deploy, *deployListen, *deployMCP, *deployToken, *init, osType) {
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
		ip, ok := lookupTargetIPOrExit(rootCtx, domain, *ipv4Only, *ipv6Only, *dot, *jsonPrint)
		if !ok {
			return
		}
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
		nextTraceAPIV3WS := prepareRuntimeEnvironment(rootCtx, *dn42, dataOrigin, disableMaptrace, powProvider, false)
		defer closeNextTraceAPIV3WebSocket(nextTraceAPIV3WS)
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
			isDN42Provider(*dataOrigin),
			trace.CachedGeoSource(ipgeo.GetSourceDescriptorWithGeoDNS(*dataOrigin, *dot)),
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
	if runFastTraceModeWithRuntime(rootCtx, *dn42, dataOrigin, disableMaptrace, powProvider, *from, *fastTraceFlag, *file, paramsFastTrace, method) {
		return
	}

	domain := resolveCLITargetOrExit(*str, sanitizeUsagePositionalArgs(parser.Usage(err)))
	if domain == "" {
		return
	}

	asyncNextTraceAPIV3 := shouldUseAsyncNextTraceAPIV3ForMTR(mtrModes, CheckTTY(int(os.Stdin.Fd())), stdoutIsTTY)
	nextTraceAPIV3WS := prepareRuntimeEnvironment(rootCtx, *dn42, dataOrigin, disableMaptrace, powProvider, asyncNextTraceAPIV3)
	defer closeNextTraceAPIV3WebSocket(nextTraceAPIV3WS)

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

	globalpingConfig := trace.Config{
		Context:         rootCtx,
		OSType:          osType,
		DN42:            *dn42 || isDN42Provider(*dataOrigin),
		NumMeasurements: *numMeasurements,
		Lang:            *lang,
		RDNS:            !*norDNS,
		AlwaysWaitRDNS:  *alwaysrDNS,
		Timeout:         time.Duration(*timeout) * time.Millisecond,
	}
	if *from != "" {
		globalpingDescriptor := ipgeo.GetSourceDescriptorWithGeoDNS(*dataOrigin, *dot)
		globalpingConfig.IPGeoSource = trace.CachedGeoSource(globalpingDescriptor)
		globalpingConfig.IPGeoDescriptor = func() ipgeo.SourceDescriptor { return globalpingDescriptor }
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
		&globalpingConfig,
	) {
		return
	}

	ip, ok := lookupTargetIPOrExit(rootCtx, domain, *ipv4Only, *ipv6Only, *dot, *jsonPrint)
	if !ok {
		return
	}

	sourceCfg, srcResolveErr := resolveCLIProbeSource(method, trace.Config{
		Context: rootCtx, OSType: osType, DstIP: ip, SrcAddr: *srcAddr, SourceDevice: *srcDev,
		SrcPort: *srcPort, DstPort: *port, TOS: *tos, Timeout: time.Duration(*timeout) * time.Millisecond,
		FWMark: mark, FWMarkSet: fwmarkSet,
	})
	if srcResolveErr != nil {
		fmt.Fprintln(os.Stderr, srcResolveErr)
		os.Exit(1)
	}
	resolvedSrcAddr, resolvedSrcDev := sourceCfg.SrcAddr, sourceCfg.SourceDevice
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
	conf.FWMark, conf.FWMarkSet = mark, fwmarkSet
	conf.SrcPort = sourceCfg.SrcPort

	if maybeRunMTRMode(mtrModes, method, conf, queriesExplicit, *numMeasurements, ttlTimeExplicit, *ttlInterval, domain, *dataOrigin, *showIPs, *ipInfoMode, mtrColumns...) {
		return
	}
	if err := writeIgnoredTraceOutputWarning(os.Stderr, outputMode, resolvedOutputPath); err != nil {
		log.Printf("write trace output warning: %v", err)
	}

	outputPlan, err := configureTracePrinters(&conf, outputMode, resolvedOutputPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := outputPlan.close(); closeErr != nil {
			log.Printf("close trace output: %v", closeErr)
		}
	}()
	if maybeRunUninterruptedRaw(outputMode == traceOutputRaw, method, conf) {
		return
	}

	res, ok := runTraceOnce(method, conf)
	if !ok {
		return
	}

	finalizeTraceResult(rootCtx, res, outputPlan, stdoutIsTTY, *routePath, ip, *disableMaptrace, *dataOrigin)
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

func shouldUseAsyncNextTraceAPIV3ForMTR(modes effectiveMTRModes, stdinTTY bool, stdoutTTY bool) bool {
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
