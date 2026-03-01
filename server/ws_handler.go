package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

const (
	wsSendQueueSize = 1024
	wsWriteTimeout  = 5 * time.Second
)

var (
	errWSSlowConsumer  = errors.New("websocket client too slow for mtr stream")
	errWSSessionClosed = errors.New("websocket session closed")
)

// sanitizeLogParam 清理用户输入中的换行和控制字符，防止日志注入。
func sanitizeLogParam(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' {
			b.WriteString("\\n")
		} else if r < 0x20 && r != '\t' {
			// 保留 tab，替换其他 C0 控制字符
			b.WriteRune('\uFFFD')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type wsEnvelope struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
	Status int         `json:"status,omitempty"`
}

type wsConn interface {
	WriteJSON(v interface{}) error
	SetWriteDeadline(t time.Time) error
	WriteControl(messageType int, data []byte, deadline time.Time) error
	Close() error
	NextReader() (messageType int, r io.Reader, err error)
}

type wsTraceSession struct {
	conn       wsConn
	sendMu     sync.Mutex
	sendCh     chan wsEnvelope
	stopCh     chan struct{}
	writerDone chan struct{}
	closeOnce  sync.Once
	finishOnce sync.Once
	closed     atomic.Bool
	lang       string
	seen       map[int]int
}

func newWSTraceSession(conn wsConn, lang string, queueSize int) *wsTraceSession {
	if queueSize <= 0 {
		queueSize = wsSendQueueSize
	}
	s := &wsTraceSession{
		conn:       conn,
		sendCh:     make(chan wsEnvelope, queueSize),
		stopCh:     make(chan struct{}),
		writerDone: make(chan struct{}),
		lang:       lang,
		seen:       make(map[int]int),
	}
	go s.writeLoop()
	return s
}

func (s *wsTraceSession) writeLoop() {
	defer close(s.writerDone)
	for {
		select {
		case <-s.stopCh:
			return
		case msg, ok := <-s.sendCh:
			if !ok {
				return
			}
			deadline := time.Now().Add(wsWriteTimeout)
			_ = s.conn.SetWriteDeadline(deadline)
			err := s.conn.WriteJSON(msg)
			if err != nil {
				s.closeWithCode(websocket.CloseInternalServerErr, "write failed")
				return
			}
		}
	}
}

func (s *wsTraceSession) send(msg wsEnvelope) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if s.closed.Load() {
		return errWSSessionClosed
	}
	select {
	case s.sendCh <- msg:
		return nil
	default:
		s.closeWithCode(websocket.CloseTryAgainLater, "client too slow for mtr stream")
		return errWSSlowConsumer
	}
}

func (s *wsTraceSession) closeWithCode(code int, reason string) {
	s.closed.Store(true)
	s.closeOnce.Do(func() {
		close(s.stopCh)
		deadline := time.Now().Add(wsWriteTimeout)
		_ = s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), deadline)
		_ = s.conn.Close()
	})
}

func (s *wsTraceSession) finish() {
	s.finishOnce.Do(func() {
		s.sendMu.Lock()
		wasClosed := s.closed.Swap(true)
		if !wasClosed {
			close(s.sendCh)
		}
		s.sendMu.Unlock()
		<-s.writerDone
		s.closeOnce.Do(func() {
			_ = s.conn.Close()
		})
	})
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
		log.Printf("[deploy] websocket prepare trace failed target=%s error=%v", sanitizeLogParam(req.Target), err)
		_ = conn.WriteJSON(wsEnvelope{Type: "error", Error: err.Error(), Status: statusCode})
		return
	}

	session := newWSTraceSession(conn, setup.Config.Lang, wsSendQueueSize)
	defer session.finish()

	startPayload := gin.H{
		"target":        setup.Target,
		"resolved_ip":   setup.IP.String(),
		"protocol":      setup.Protocol,
		"data_provider": setup.DataProvider,
		"language":      setup.Config.Lang,
	}
	if err := session.send(wsEnvelope{Type: "start", Data: startPayload}); err != nil {
		log.Printf("[deploy] websocket send start failed: %v", err)
		return
	}

	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				session.closeWithCode(websocket.CloseNormalClosure, "client disconnected")
				return
			}
		}
	}()

	log.Printf("[deploy] (ws) trace request target=%s proto=%s provider=%s lang=%s ipv4_only=%t ipv6_only=%t", sanitizeLogParam(setup.Target), sanitizeLogParam(setup.Protocol), sanitizeLogParam(setup.DataProvider), sanitizeLogParam(setup.Config.Lang), setup.Req.IPv4Only, setup.Req.IPv6Only)
	log.Printf("[deploy] (ws) target resolved target=%s ip=%s via dot=%s", sanitizeLogParam(setup.Target), setup.IP, sanitizeLogParam(strings.ToLower(setup.Req.DotServer)))

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
				snapshot := append([]trace.Hop(nil), attempts...)
				newLen := len(snapshot)
				if newLen == 0 {
					continue
				}
				if prevLen, ok := session.seen[idx]; ok && newLen <= prevLen {
					continue
				}
				session.seen[idx] = newLen

				hop := buildHopResponse(snapshot, idx, session.lang)
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
		log.Printf("[deploy] websocket trace failed target=%s error=%v", sanitizeLogParam(setup.Target), err)
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
				log.Printf("[deploy] (ws) trace map generated target=%s url=%s", sanitizeLogParam(setup.Target), traceMapURL)
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
	log.Printf("[deploy] (ws) trace completed target=%s hops=%d duration=%s", sanitizeLogParam(setup.Target), len(final.Hops), duration)
}

func runMTRTrace(session *wsTraceSession, setup *traceExecution) {
	interval := time.Duration(setup.Req.IntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = 2 * time.Second
	}
	maxRounds := setup.Req.MaxRounds

	iteration := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Client disconnect / stop should terminate continuous raw stream.
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if session.closed.Load() {
					cancel()
					return
				}
			}
		}
	}()

	err := executeMTRRaw(ctx, session, setup, trace.MTRRawOptions{
		Interval:  interval,
		MaxRounds: maxRounds,
	}, func(rec trace.MTRRawRecord) {
		if rec.Iteration > iteration {
			iteration = rec.Iteration
		}
		if err := session.send(wsEnvelope{Type: "mtr_raw", Data: rec}); err != nil {
			cancel()
		}
	})
	if err != nil && err != context.Canceled {
		log.Printf("[deploy] websocket MTR raw trace failed target=%s error=%v", sanitizeLogParam(setup.Target), err)
		_ = session.send(wsEnvelope{Type: "error", Error: err.Error(), Status: 500})
		return
	}

	if !session.closed.Load() {
		_ = session.send(wsEnvelope{Type: "complete", Data: gin.H{"iteration": iteration}})
	}
}

func executeMTRRaw(ctx context.Context, session *wsTraceSession, setup *traceExecution, opts trace.MTRRawOptions, onRecord trace.MTRRawOnRecord) error {
	config := setup.Config
	log.Printf("[deploy] (ws) starting MTR raw trace target=%s resolved=%s method=%s lang=%s maxHops=%d interval=%s maxRounds=%d",
		sanitizeLogParam(setup.Target), setup.IP.String(), string(setup.Method), sanitizeLogParam(config.Lang), config.MaxHops, opts.Interval, opts.MaxRounds)

	if session.closed.Load() {
		return nil
	}

	opts.RunRound = func(method trace.Method, cfg trace.Config) (*trace.Result, error) {
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
				log.Printf("[deploy] (ws) LeoMoeAPI using custom PoW provider=%s", sanitizeLogParam(setup.PowProvider))
			} else {
				log.Printf("[deploy] (ws) LeoMoeAPI using default PoW provider")
			}
			util.PowProviderParam = setup.PowProvider
			ensureLeoMoeConnection()
		} else if setup.PowProvider != "" {
			log.Printf("[deploy] (ws) overriding PoW provider=%s", sanitizeLogParam(setup.PowProvider))
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

		return trace.Traceroute(method, cfg)
	}

	return trace.RunMTRRaw(ctx, setup.Method, config, opts, onRecord)
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
			log.Printf("[deploy] (ws) LeoMoeAPI using custom PoW provider=%s", sanitizeLogParam(setup.PowProvider))
		} else {
			log.Printf("[deploy] (ws) LeoMoeAPI using default PoW provider")
		}
		util.PowProviderParam = setup.PowProvider
		ensureLeoMoeConnection()
	} else if setup.PowProvider != "" {
		log.Printf("[deploy] (ws) overriding PoW provider=%s", sanitizeLogParam(setup.PowProvider))
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

	log.Printf("[deploy] (ws) starting trace target=%s resolved=%s method=%s lang=%s queries=%d maxHops=%d", sanitizeLogParam(setup.Target), setup.IP.String(), string(setup.Method), sanitizeLogParam(config.Lang), config.NumMeasurements, config.MaxHops)
	start := time.Now()
	res, err := trace.Traceroute(setup.Method, config)
	duration := time.Since(start)
	return res, duration, err
}
