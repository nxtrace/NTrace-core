package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracemap"
	"github.com/nxtrace/NTrace-core/util"
)

var traceUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type wsEnvelope struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
	Status int         `json:"status,omitempty"`
}

type wsTraceSession struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	closed  atomic.Bool
	lang    string
	seen    map[int]int
}

func (s *wsTraceSession) send(msg wsEnvelope) error {
	if s.closed.Load() {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.conn.WriteJSON(msg); err != nil {
		if !s.closed.Load() {
			s.closed.Store(true)
		}
		return err
	}
	return nil
}

func traceWebsocketHandler(c *gin.Context) {
	conn, err := traceUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[deploy] websocket upgrade failed: %v", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Printf("[deploy] websocket read failed: %v", err)
		return
	}

	var req traceRequest
	if err := json.Unmarshal(message, &req); err != nil {
		_ = conn.WriteJSON(wsEnvelope{Type: "error", Error: "invalid request payload", Status: 400})
		return
	}

	setup, statusCode, err := prepareTrace(req)
	if err != nil {
		if statusCode == 0 {
			statusCode = 500
		}
		log.Printf("[deploy] websocket prepare trace failed target=%s error=%v", req.Target, err)
		_ = conn.WriteJSON(wsEnvelope{Type: "error", Error: err.Error(), Status: statusCode})
		return
	}

	session := &wsTraceSession{
		conn: conn,
		lang: setup.Config.Lang,
		seen: make(map[int]int),
	}

	startPayload := gin.H{
		"target":        setup.Target,
		"resolved_ip":   setup.IP.String(),
		"protocol":      setup.Protocol,
		"data_provider": setup.DataProvider,
		"language":      setup.Config.Lang,
	}
	if err := session.send(wsEnvelope{Type: "start", Data: startPayload}); err != nil {
		log.Printf("[deploy] websocket send start failed: %v", err)
	}

	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				session.closed.Store(true)
				return
			}
		}
	}()

	log.Printf("[deploy] (ws) trace request target=%s proto=%s provider=%s lang=%s ipv4_only=%t ipv6_only=%t", setup.Target, setup.Protocol, setup.DataProvider, setup.Config.Lang, setup.Req.IPv4Only, setup.Req.IPv6Only)
	log.Printf("[deploy] (ws) target resolved target=%s ip=%s via dot=%s", setup.Target, setup.IP, strings.ToLower(setup.Req.DotServer))

	mode := setup.Req.Mode
	if mode == "" {
		mode = "single"
	}

	switch mode {
	case "mtr", "continuous":
		runMTRTrace(session, setup)
	default:
		runSingleTrace(session, setup)
	}
}

func runSingleTrace(session *wsTraceSession, setup *traceExecution) {
	session.seen = make(map[int]int)

	res, duration, err := executeTrace(session, setup, func(cfg *trace.Config) {
		cfg.RealtimePrinter = nil
		cfg.AsyncPrinter = func(result *trace.Result) {
			for idx, attempts := range result.Hops {
				if len(attempts) == 0 {
					continue
				}
				if session.seen[idx] == len(attempts) {
					continue
				}
				session.seen[idx] = len(attempts)

				hop := buildHopResponse(attempts, idx, session.lang)
				if len(hop.Attempts) == 0 {
					continue
				}
				if err := session.send(wsEnvelope{Type: "hop", Data: hop}); err != nil {
					log.Printf("[deploy] websocket hop send failed ttl=%d err=%v", hop.TTL, err)
					return
				}
			}
		}
	})

	if err != nil {
		log.Printf("[deploy] websocket trace failed target=%s error=%v", setup.Target, err)
		_ = session.send(wsEnvelope{Type: "error", Error: err.Error(), Status: 500})
		return
	}

	if session.closed.Load() {
		return
	}

	traceMapURL := ""
	if setup.Config.Maptrace && shouldGenerateMap(setup.DataProvider) {
		if payload, err := json.Marshal(res); err == nil {
			if url, err := tracemap.GetMapUrl(string(payload)); err == nil {
				traceMapURL = url
				log.Printf("[deploy] (ws) trace map generated target=%s url=%s", setup.Target, traceMapURL)
			}
		}
	}

	final := traceResponse{
		Target:       setup.Target,
		ResolvedIP:   setup.IP.String(),
		Protocol:     setup.Protocol,
		DataProvider: setup.DataProvider,
		TraceMapURL:  traceMapURL,
		Language:     setup.Config.Lang,
		Hops:         convertHops(res, setup.Config.Lang),
		DurationMs:   duration.Milliseconds(),
	}

	if err := session.send(wsEnvelope{Type: "complete", Data: final}); err != nil {
		log.Printf("[deploy] websocket send complete failed: %v", err)
	}
	log.Printf("[deploy] (ws) trace completed target=%s hops=%d duration=%s", setup.Target, len(final.Hops), duration)
}

func runMTRTrace(session *wsTraceSession, setup *traceExecution) {
	interval := time.Duration(setup.Req.IntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = 2 * time.Second
	}
	maxRounds := setup.Req.MaxRounds

	aggregator := newMTRAggregator()
	iteration := 0
	queries := setup.Config.NumMeasurements
	if queries <= 0 {
		queries = 1
	}

	for {
		if session.closed.Load() {
			break
		}

		res, _, err := executeTrace(session, setup, func(cfg *trace.Config) {
			cfg.RealtimePrinter = nil
			cfg.AsyncPrinter = nil
		})

		if err != nil {
			log.Printf("[deploy] websocket MTR trace failed target=%s error=%v", setup.Target, err)
			_ = session.send(wsEnvelope{Type: "error", Error: err.Error(), Status: 500})
			break
		}

		iteration++
		stats := aggregator.Update(res, queries)
		snapshot := mtrSnapshot{Iteration: iteration, Stats: stats}
		if err := session.send(wsEnvelope{Type: "mtr", Data: snapshot}); err != nil {
			session.closed.Store(true)
			break
		}

		if maxRounds > 0 && iteration >= maxRounds {
			break
		}

		if session.closed.Load() {
			break
		}

		time.Sleep(interval)
	}

	finalStats := aggregator.Snapshot()
	if !session.closed.Load() {
		_ = session.send(wsEnvelope{Type: "complete", Data: mtrSnapshot{Iteration: iteration, Stats: finalStats}})
	}
}

func executeTrace(session *wsTraceSession, setup *traceExecution, configure func(*trace.Config)) (*trace.Result, time.Duration, error) {
	traceMu.Lock()
	defer traceMu.Unlock()

	prevSrcPort := util.SrcPort
	prevDstIP := util.DstIP
	prevSrcDev := util.SrcDev
	prevDisableMPLS := util.DisableMPLS
	prevPowProvider := util.PowProviderParam
	defer func() {
		util.SrcPort = prevSrcPort
		util.DstIP = prevDstIP
		util.SrcDev = prevSrcDev
		util.DisableMPLS = prevDisableMPLS
		util.PowProviderParam = prevPowProvider
	}()

	if setup.NeedsLeoWS {
		if setup.PowProvider != "" {
			log.Printf("[deploy] (ws) LeoMoeAPI using custom PoW provider=%s", setup.PowProvider)
		} else {
			log.Printf("[deploy] (ws) LeoMoeAPI using default PoW provider")
		}
		util.PowProviderParam = setup.PowProvider
		ensureLeoMoeConnection()
	} else if setup.PowProvider != "" {
		log.Printf("[deploy] (ws) overriding PoW provider=%s", setup.PowProvider)
		util.PowProviderParam = setup.PowProvider
	} else {
		util.PowProviderParam = ""
	}

	util.SrcPort = setup.Req.SourcePort
	util.DstIP = setup.IP.String()
	if setup.Req.SourceDevice != "" {
		util.SrcDev = setup.Req.SourceDevice
	} else {
		util.SrcDev = ""
	}
	util.DisableMPLS = setup.Req.DisableMPLS

	config := setup.Config
	if configure != nil {
		configure(&config)
	}

	if session.closed.Load() {
		return nil, 0, nil
	}

	log.Printf("[deploy] (ws) starting trace target=%s resolved=%s method=%s lang=%s queries=%d maxHops=%d", setup.Target, setup.IP.String(), string(setup.Method), config.Lang, config.NumMeasurements, config.MaxHops)
	start := time.Now()
	res, err := trace.Traceroute(setup.Method, config)
	duration := time.Since(start)
	return res, duration, err
}
