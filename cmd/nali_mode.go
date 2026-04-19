package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/akamensky/argparse"

	"github.com/nxtrace/NTrace-core/internal/nali"
	"github.com/nxtrace/NTrace-core/ipgeo"
)

func registerNaliFlag(parser *argparse.Parser) *bool {
	return registerNaliFlagWithAvailability(parser, enableNali)
}

func registerNaliFlagWithAvailability(parser *argparse.Parser, enabled bool) *bool {
	if enabled {
		return parser.Flag("", "nali", &argparse.Options{Help: "Annotate IP literals in text using NextTrace GeoIP data"})
	}
	return ptrBool(false)
}

type naliModeOptions struct {
	ipv4Only         bool
	ipv6Only         bool
	tcp              bool
	udp              bool
	mtu              bool
	mtr              bool
	raw              bool
	table            bool
	classic          bool
	json             bool
	output           bool
	outputDefault    bool
	routePath        bool
	from             bool
	deploy           bool
	listen           bool
	fastTrace        bool
	file             bool
	disableMPLS      bool
	noRDNS           bool
	alwaysRDNS       bool
	init             bool
	icmpMode         bool
	port             bool
	queries          bool
	maxAttempts      bool
	parallelRequests bool
	maxHops          bool
	beginHop         bool
	packetInterval   bool
	ttlInterval      bool
	packetSize       bool
	tos              bool
	source           bool
	sourcePort       bool
	sourceDevice     bool
}

type naliModeOptionInputs struct {
	parser        *argparse.Parser
	ipv4Only      bool
	ipv6Only      bool
	tcp           bool
	udp           bool
	mtu           bool
	mtrModes      effectiveMTRModes
	raw           bool
	table         bool
	classic       bool
	json          bool
	outputPath    string
	outputDefault bool
	routePath     bool
	from          string
	deploy        bool
	listen        string
	fastTrace     bool
	file          string
	disableMPLS   bool
	noRDNS        bool
	alwaysRDNS    bool
	init          bool
	srcAddr       string
	srcDev        string
}

type naliRunOptions struct {
	stdin     io.Reader
	stdout    io.Writer
	dn42      bool
	data      string
	dot       string
	pow       string
	lang      string
	timeoutMs int
	ipv4Only  bool
	ipv6Only  bool
	target    string
}

func validateNaliModeOptions(opts naliModeOptions) error {
	if opts.json {
		return fmt.Errorf("--nali 不支持 --json")
	}
	if opts.ipv4Only && opts.ipv6Only {
		return fmt.Errorf("-4/--ipv4 不能与 -6/--ipv6 同时使用")
	}
	for _, conflict := range []struct {
		name    string
		enabled bool
	}{
		{"--mtu", opts.mtu},
		{"--mtr/-r/--report/-w/--wide", opts.mtr},
		{"--raw", opts.raw},
		{"--table", opts.table},
		{"--classic", opts.classic},
		{"--output", opts.output},
		{"--output-default", opts.outputDefault},
		{"--route-path", opts.routePath},
		{"--from", opts.from},
		{"--deploy", opts.deploy},
		{"--listen", opts.listen},
		{"--fast-trace", opts.fastTrace},
		{"--file", opts.file},
		{"--tcp", opts.tcp},
		{"--udp", opts.udp},
		{"--port", opts.port},
		{"--icmp-mode", opts.icmpMode},
		{"--queries", opts.queries},
		{"--max-attempts", opts.maxAttempts},
		{"--parallel-requests", opts.parallelRequests},
		{"--max-hops", opts.maxHops},
		{"--first", opts.beginHop},
		{"--send-time", opts.packetInterval},
		{"--ttl-time", opts.ttlInterval},
		{"--psize", opts.packetSize},
		{"--tos", opts.tos},
		{"--source", opts.source},
		{"--source-port", opts.sourcePort},
		{"--dev", opts.sourceDevice},
		{"--disable-mpls", opts.disableMPLS},
		{"--no-rdns", opts.noRDNS},
		{"--always-rdns", opts.alwaysRDNS},
		{"--init", opts.init},
	} {
		if conflict.enabled {
			return fmt.Errorf("--nali 不能与 %s 同时使用", conflict.name)
		}
	}
	return nil
}

func buildNaliModeOptions(input naliModeOptionInputs) naliModeOptions {
	return naliModeOptions{
		ipv4Only:         input.ipv4Only,
		ipv6Only:         input.ipv6Only,
		tcp:              input.tcp,
		udp:              input.udp,
		mtu:              input.mtu,
		mtr:              input.mtrModes.mtr,
		raw:              input.raw,
		table:            input.table,
		classic:          input.classic,
		json:             input.json,
		output:           strings.TrimSpace(input.outputPath) != "",
		outputDefault:    input.outputDefault,
		routePath:        input.routePath,
		from:             strings.TrimSpace(input.from) != "",
		deploy:           input.deploy,
		listen:           strings.TrimSpace(input.listen) != "",
		fastTrace:        input.fastTrace,
		file:             strings.TrimSpace(input.file) != "",
		disableMPLS:      input.disableMPLS,
		noRDNS:           input.noRDNS,
		alwaysRDNS:       input.alwaysRDNS,
		init:             input.init,
		icmpMode:         parsedFlag(input.parser, "icmp-mode"),
		port:             parsedFlag(input.parser, "port"),
		queries:          parsedFlag(input.parser, "queries"),
		maxAttempts:      parsedFlag(input.parser, "max-attempts"),
		parallelRequests: parsedFlag(input.parser, "parallel-requests"),
		maxHops:          parsedFlag(input.parser, "max-hops"),
		beginHop:         parsedFlag(input.parser, "first"),
		packetInterval:   parsedFlag(input.parser, "send-time"),
		ttlInterval:      parsedFlag(input.parser, "ttl-time"),
		packetSize:       parsedFlag(input.parser, "psize"),
		tos:              parsedFlag(input.parser, "tos"),
		source:           strings.TrimSpace(input.srcAddr) != "",
		sourcePort:       parsedFlag(input.parser, "source-port"),
		sourceDevice:     strings.TrimSpace(input.srcDev) != "",
	}
}

func parsedFlag(parser *argparse.Parser, lname string) bool {
	for _, arg := range parser.GetArgs() {
		if arg.GetLname() == lname {
			return arg.GetParsed()
		}
	}
	return false
}

func runNaliMode(ctx context.Context, opts naliRunOptions) error {
	configureGeoDNS(opts.dot)
	restoreFastIPOutput := setFastIPOutputSuppression(true)
	defer restoreFastIPOutput()

	disableMaptrace := false
	applyDN42Mode(opts.dn42, &opts.data, &disableMaptrace)
	leoWs := initLeoWebsocket(ctx, &opts.data, &opts.pow, false)
	defer closeLeoWebsocket(leoWs)

	family := nali.FamilyAll
	if opts.ipv4Only {
		family = nali.Family4
	} else if opts.ipv6Only {
		family = nali.Family6
	}
	return nali.Run(ctx, nali.Config{
		Source:  ipgeo.GetSourceWithGeoDNS(opts.data, opts.dot),
		Timeout: time.Duration(opts.timeoutMs) * time.Millisecond,
		Lang:    opts.lang,
		Family:  family,
	}, opts.stdin, opts.stdout, opts.target)
}
