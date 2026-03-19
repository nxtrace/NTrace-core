package wshandle

import (
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/nxtrace/NTrace-core/pow"
	"github.com/nxtrace/NTrace-core/util"
)

func formatHostPort(addr, port string) string {
	clean := strings.TrimSpace(addr)
	clean = strings.Trim(clean, "[]")
	if strings.Contains(clean, ":") {
		return "[" + clean + "]:" + port
	}
	return clean + ":" + port
}

type wsWriteJob struct {
	msgType int
	data    []byte
}

const (
	wsClientWriteQueueSize = 1024
	wsClientWriteTimeout   = 5 * time.Second
)

type WsConn struct {
	Connecting   bool
	Connected    bool            // 连接状态
	MsgSendCh    chan string     // 消息发送通道
	MsgReceiveCh chan string     // 消息接收通道
	Done         chan struct{}   // 发送结束通道
	Exit         chan bool       // 程序退出信号
	Interrupt    chan os.Signal  // 终端中止信号
	Conn         *websocket.Conn // 主连接
	ConnMux      sync.Mutex      // 连接互斥锁
	stateMu      sync.RWMutex
	lifecycleMu  sync.Mutex
	loopWG       sync.WaitGroup
	closeOnce    sync.Once
	writeCh      chan wsWriteJob // serialized write queue
	writeStop    chan struct{}   // signals writeLoop to exit
	closeCh      chan struct{}   // signals background loops to exit
	closed       bool
}

func (c *WsConn) getConn() *websocket.Conn {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.Conn
}

func (c *WsConn) setConn(conn *websocket.Conn) {
	c.stateMu.Lock()
	c.Conn = conn
	c.stateMu.Unlock()
}

func (c *WsConn) getDoneChan() chan struct{} {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.Done
}

func (c *WsConn) setDoneChan(done chan struct{}) {
	c.stateMu.Lock()
	c.Done = done
	c.stateMu.Unlock()
}

// initWriteLoop creates the write channel and starts the single writer goroutine.
// Must be called once when the WsConn is created.
func (c *WsConn) initWriteLoop() {
	c.writeCh = make(chan wsWriteJob, wsClientWriteQueueSize)
	c.writeStop = make(chan struct{})
	c.startLoop(c.writeLoop)
}

// writeLoop is the sole goroutine allowed to call conn.WriteMessage.
func (c *WsConn) writeLoop() {
	for {
		select {
		case <-c.writeStop:
			return
		case job, ok := <-c.writeCh:
			if !ok {
				return
			}
			conn := c.getConn()
			if conn == nil {
				c.setConnected(false)
				continue
			}
			_ = conn.SetWriteDeadline(time.Now().Add(wsClientWriteTimeout))
			if err := conn.WriteMessage(job.msgType, job.data); err != nil {
				log.Printf("wshandle writeLoop: %v", err)
				c.setConnected(false)
			}
		}
	}
}

// enqueueWrite sends a write job to the writeLoop. Returns an error if the
// queue is full or writeLoop has stopped.
func (c *WsConn) enqueueWrite(job wsWriteJob) error {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		return errWriteLoopStopped
	}
	select {
	case c.writeCh <- job:
		return nil
	case <-c.writeStop:
		return errWriteLoopStopped
	default:
		return errWriteQueueFull
	}
}

var (
	errWriteQueueFull   = errors.New("wshandle: write queue full")
	errWriteLoopStopped = errors.New("wshandle: write loop stopped")
)

var wsconn *WsConn
var wsconnMu sync.RWMutex
var wsconnNewMu sync.Mutex
var host, port, fastIp string
var envToken = util.EnvToken
var cacheToken string
var cacheTokenFailedTimes int
var createWsConnFn = createWsConn

func newWsConn(conn *websocket.Conn, interrupt chan os.Signal) *WsConn {
	c := &WsConn{
		Conn:         conn,
		MsgSendCh:    make(chan string, 10),
		MsgReceiveCh: make(chan string, 10),
		Interrupt:    interrupt,
		closeCh:      make(chan struct{}),
	}
	c.initWriteLoop()
	return c
}

func (c *WsConn) startLoop(fn func()) {
	c.loopWG.Add(1)
	go func() {
		defer c.loopWG.Done()
		fn()
	}()
}

func (c *WsConn) isClosed() bool {
	if c == nil {
		return true
	}
	select {
	case <-c.closeCh:
		return true
	default:
		return false
	}
}

func closeSignalChan(ch chan struct{}) {
	if ch == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	close(ch)
}

func (c *WsConn) closeConn() {
	conn := c.getConn()
	if conn == nil {
		return
	}
	_ = conn.Close()
	c.setConn(nil)
}

func (c *WsConn) replaceConn(conn *websocket.Conn) {
	c.stateMu.Lock()
	prev := c.Conn
	c.Conn = conn
	c.stateMu.Unlock()
	if prev != nil && prev != conn {
		_ = prev.Close()
	}
}

func (c *WsConn) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		c.lifecycleMu.Lock()
		c.closed = true
		c.lifecycleMu.Unlock()

		c.setConnectionState(false, false)
		if c.closeCh != nil {
			close(c.closeCh)
		}
		closeSignalChan(c.writeStop)
		closeSignalChan(c.getDoneChan())
		if c.Interrupt != nil {
			signal.Stop(c.Interrupt)
		}
		c.closeConn()
	})
	c.loopWG.Wait()
}

func (c *WsConn) setConnectionState(connected, connecting bool) {
	c.stateMu.Lock()
	c.Connected = connected
	c.Connecting = connecting
	c.stateMu.Unlock()
}

func (c *WsConn) setConnected(v bool) {
	c.stateMu.Lock()
	c.Connected = v
	c.stateMu.Unlock()
}

func (c *WsConn) setConnecting(v bool) {
	c.stateMu.Lock()
	c.Connecting = v
	c.stateMu.Unlock()
}

func (c *WsConn) IsConnected() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.Connected
}

func (c *WsConn) IsConnecting() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.Connecting
}

func (c *WsConn) startReconnecting() bool {
	if c.isClosed() {
		return false
	}
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.Connected || c.Connecting {
		return false
	}
	c.Connecting = true
	return true
}

func (c *WsConn) keepAlive() {
	pingTicker := time.NewTicker(54 * time.Second)
	defer pingTicker.Stop()
	reconnectTicker := time.NewTicker(200 * time.Millisecond)
	defer reconnectTicker.Stop()

	for {
		select {
		case <-c.closeCh:
			return
		case <-pingTicker.C:
			if !c.IsConnected() {
				continue
			}
			if err := c.enqueueWrite(wsWriteJob{
				msgType: websocket.TextMessage,
				data:    []byte("ping"),
			}); err != nil {
				log.Println(err)
				c.setConnected(false)
			}
		case <-reconnectTicker.C:
			if c.startReconnecting() {
				c.recreateWsConn()
			}
		}
	}
}

func (c *WsConn) messageReceiveHandler() {
	done := c.getDoneChan()
	defer closeSignalChan(done)
	for {
		select {
		case <-c.closeCh:
			return
		default:
		}
		if c.IsConnected() {
			conn := c.getConn()
			if conn == nil {
				c.setConnected(false)
				continue
			}
			_, msg, err := conn.ReadMessage()
			if err != nil {
				// 读取信息出错，连接已经意外断开
				// log.Println(err)
				c.setConnected(false)
				return
			}
			if string(msg) != "pong" {
				select {
				case c.MsgReceiveCh <- string(msg):
				case <-c.closeCh:
					return
				}
			}
		} else {
			// 降低断线时期的 CPU 占用
			select {
			case <-c.closeCh:
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
}

func apiServerErrorMessage(ip string) string {
	return `{"ip":"` + ip + `", "asnumber":"API Server Error"}`
}

func (c *WsConn) waitForNextDoneChan(doneCh chan struct{}) chan struct{} {
	for {
		newDone := c.getDoneChan()
		if newDone != nil && newDone != doneCh {
			return newDone
		}
		select {
		case <-c.closeCh:
			return nil
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (c *WsConn) sendQueuedMessage(msg string) {
	if !c.IsConnected() {
		c.MsgReceiveCh <- apiServerErrorMessage(msg)
		return
	}

	if err := c.enqueueWrite(wsWriteJob{
		msgType: websocket.TextMessage,
		data:    []byte(msg),
	}); err != nil {
		log.Println("write:", err)
		c.setConnected(false)
		c.MsgReceiveCh <- apiServerErrorMessage(msg)
	}
}

func (c *WsConn) handleInterrupt(doneCh chan struct{}) {
	_ = c.enqueueWrite(wsWriteJob{
		msgType: websocket.CloseMessage,
		data:    websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	})

	select {
	case <-doneCh:
	case <-time.After(1 * time.Second):
	}
}

func (c *WsConn) messageSendHandler() {
	doneCh := c.getDoneChan()
	for {
		if current := c.getDoneChan(); current != nil && current != doneCh {
			doneCh = current
		}

		select {
		case <-c.closeCh:
			return
		case <-doneCh:
			doneCh = c.waitForNextDoneChan(doneCh)
			if doneCh == nil {
				return
			}
		case msg := <-c.MsgSendCh:
			c.sendQueuedMessage(msg)
		case <-c.Interrupt:
			c.handleInterrupt(doneCh)
			return
		}
	}
}

func (c *WsConn) recreateWsConn() {
	if c.isClosed() {
		return
	}
	c.setConnected(false)
	// 尝试重新连线
	if host != "" && net.ParseIP(host) == nil {
		// 刷新一次最优 IP，防止旧 IP 已失效
		fastIp = util.GetFastIP(host, port, true)
	}
	u := url.URL{Scheme: "wss", Host: formatHostPort(fastIp, port), Path: "/v3/ipGeoWs"}
	// log.Printf("connecting to %s", u.String())
	jwtToken, ua := envToken, []string{"Privileged Client"}
	err := error(nil)
	if envToken == "" {
		// 无环境变量 token
		if cacheToken == "" {
			// 无cacheToken, 重新获取 token
			if util.GetPowProvider() == "" {
				jwtToken, err = pow.GetToken(fastIp, host, port)
			} else {
				jwtToken, err = pow.GetToken(util.GetPowProvider(), util.GetPowProvider(), port)
			}
			if err != nil {
				if util.EnvDevMode {
					panic(err)
				}
				log.Printf("pow token fetch failed: %v", err)
				cacheToken = ""
				cacheTokenFailedTimes++
				c.setConnectionState(false, false)
				return
			}
		} else {
			// 使用 cacheToken
			jwtToken = cacheToken
		}
		ua = []string{util.UserAgent}
	}
	cacheToken = jwtToken
	requestHeader := http.Header{
		"Host":          []string{host},
		"User-Agent":    ua,
		"Authorization": []string{"Bearer " + jwtToken},
	}
	dialer := *websocket.DefaultDialer // 按值拷贝，变成独立实例
	// 现在 dialer 是一个新的 Dialer（值），内部字段与 DefaultDialer 相同
	dialer.TLSClientConfig = &tls.Config{
		ServerName: host,
	}
	proxyUrl := util.GetProxy()
	if proxyUrl != nil {
		dialer.Proxy = http.ProxyURL(proxyUrl)
	}
	ws, _, err := dialer.Dial(u.String(), requestHeader)
	if c.isClosed() {
		if ws != nil {
			_ = ws.Close()
		}
		return
	}
	c.replaceConn(ws)
	if err != nil {
		log.Println("dial:", err)
		// <-time.After(time.Second * 1)
		c.setConnectionState(false, false)
		cacheTokenFailedTimes += 1
		time.Sleep(1 * time.Second)
		//fmt.Println("重连失败", cacheTokenFailedTimes, "次")
		return
	}
	c.setConnectionState(err == nil, false)

	c.setDoneChan(make(chan struct{}))
	c.startLoop(c.messageReceiveHandler)
}

func createWsConn() *WsConn {
	proxyUrl := util.GetProxy()
	//fmt.Println("正在连接 WS")
	// 设置终端中断通道
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	host, port = util.GetHostAndPort()
	// 如果 host 是一个 IP 使用默认域名
	if valid := net.ParseIP(host); valid != nil {
		fastIp = host
		host = "api.nxtrace.org"
	} else {
		// 默认配置完成，开始寻找最优 IP
		fastIp = util.GetFastIP(host, port, true)
	}
	jwtToken, ua := envToken, []string{"Privileged Client"}
	err := error(nil)
	if envToken == "" {
		if util.GetPowProvider() == "" {
			jwtToken, err = pow.GetToken(fastIp, host, port)
		} else {
			jwtToken, err = pow.GetToken(util.GetPowProvider(), util.GetPowProvider(), port)
		}
		if err != nil {
			if util.EnvDevMode {
				panic(err)
			}
			log.Printf("pow token fetch failed: %v", err)
			ws := newWsConn(nil, interrupt)
			ws.setDoneChan(make(chan struct{}))
			ws.setConnectionState(false, false)
			ws.startLoop(ws.keepAlive)
			ws.startLoop(ws.messageSendHandler)
			return ws
		}
		ua = []string{util.UserAgent}
	}
	cacheToken = jwtToken
	cacheTokenFailedTimes = 0
	requestHeader := http.Header{
		"Host":          []string{host},
		"User-Agent":    ua,
		"Authorization": []string{"Bearer " + jwtToken},
	}
	dialer := *websocket.DefaultDialer // 按值拷贝，变成独立实例
	// 现在 dialer 是一个新的 Dialer（值），内部字段与 DefaultDialer 相同
	dialer.TLSClientConfig = &tls.Config{
		ServerName: host,
	}
	if proxyUrl != nil {
		dialer.Proxy = http.ProxyURL(proxyUrl)
	}
	u := url.URL{Scheme: "wss", Host: formatHostPort(fastIp, port), Path: "/v3/ipGeoWs"}
	// log.Printf("connecting to %s", u.String())

	c, _, err := dialer.Dial(u.String(), requestHeader)

	ws := newWsConn(c, interrupt)
	ws.setConnectionState(err == nil, false)

	if err != nil {
		log.Println("dial:", err)
		// <-time.After(time.Second * 1)
		cacheTokenFailedTimes++
		ws.setDoneChan(make(chan struct{}))
		ws.startLoop(ws.keepAlive)
		ws.startLoop(ws.messageSendHandler)
		return ws
	}
	// defer c.Close()
	// 将连接写入WsConn，方便随时可取
	ws.setDoneChan(make(chan struct{}))
	ws.startLoop(ws.keepAlive)
	ws.startLoop(ws.messageReceiveHandler)
	ws.startLoop(ws.messageSendHandler)
	return ws
}

func New() *WsConn {
	wsconnNewMu.Lock()
	defer wsconnNewMu.Unlock()

	newConn := createWsConnFn()

	wsconnMu.Lock()
	oldConn := wsconn
	wsconn = newConn
	wsconnMu.Unlock()

	if oldConn != nil && oldConn != newConn {
		oldConn.Close()
	}
	return newConn
}

func GetWsConn() *WsConn {
	wsconnMu.RLock()
	defer wsconnMu.RUnlock()
	return wsconn
}
