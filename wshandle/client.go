package wshandle

import (
	"crypto/tls"
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

var wsconn *WsConn
var host, port, fastIp string
var envToken = util.EnvToken
var cacheToken string
var cacheTokenFailedTimes int

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
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.Connected || c.Connecting {
		return false
	}
	c.Connecting = true
	return true
}

func (c *WsConn) keepAlive() {
	go func() {
		// 开启一个定时器
		for {
			<-time.After(time.Second * 54)
			if c.IsConnected() {
				conn := c.getConn()
				if conn == nil {
					c.setConnected(false)
					continue
				}
				err := conn.WriteMessage(websocket.TextMessage, []byte("ping"))
				if err != nil {
					log.Println(err)
					c.setConnected(false)
					return
				}
			}
		}
	}()
	for {
		if c.startReconnecting() {
			c.recreateWsConn()
		}
		// 降低检测频率，优化 CPU 占用情况
		<-time.After(200 * time.Millisecond)
	}
}

func (c *WsConn) messageReceiveHandler() {
	defer func() {
		select {
		case <-c.Done:
		default:
			close(c.Done)
		}
	}()
	for {
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
				c.MsgReceiveCh <- string(msg)
			}
		} else {
			// 降低断线时期的 CPU 占用
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (c *WsConn) messageSendHandler() {
	for {
		// 循环监听发送
		select {
		case <-c.Done:
			return
		case t := <-c.MsgSendCh:
			// log.Println(t)
			if !c.IsConnected() {
				c.MsgReceiveCh <- `{"ip":"` + t + `", "asnumber":"API Server Error"}`
				continue
			}
			conn := c.getConn()
			if conn == nil {
				c.MsgReceiveCh <- `{"ip":"` + t + `", "asnumber":"API Server Error"}`
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(t)); err != nil {
				log.Println("write:", err)
				c.setConnected(false)
				c.MsgReceiveCh <- `{"ip":"` + t + `", "asnumber":"API Server Error"}`
				continue
			}
		// 来自终端的中断运行请求
		case <-c.Interrupt:
			// 向 websocket 发起关闭连接任务
			conn := c.getConn()
			if conn == nil {
				return
			}
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				if util.EnvDevMode {
					panic(err)
				}
				log.Printf("write close: %v", err)
			}
			select {
			// 等到了结果，直接退出
			case <-c.Done:
			// 如果等待 1s 还是拿不到结果，不再等待，超时退出
			case <-time.After(1 * time.Second):
			}
			return
		}
	}
}

func (c *WsConn) recreateWsConn() {
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
	c.setConn(ws)
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

	c.Done = make(chan struct{})
	go c.messageReceiveHandler()
	go c.messageSendHandler()
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
			wsconn = &WsConn{
				MsgSendCh:    make(chan string, 10),
				MsgReceiveCh: make(chan string, 10),
				Done:         make(chan struct{}),
				Interrupt:    interrupt,
			}
			wsconn.setConnectionState(false, false)
			go wsconn.keepAlive()
			go wsconn.messageSendHandler()
			return wsconn
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

	wsconn = &WsConn{
		Conn:         c,
		MsgSendCh:    make(chan string, 10),
		MsgReceiveCh: make(chan string, 10),
		Interrupt:    interrupt,
	}
	wsconn.setConnectionState(err == nil, false)

	if err != nil {
		log.Println("dial:", err)
		// <-time.After(time.Second * 1)
		cacheTokenFailedTimes++
		wsconn.Done = make(chan struct{})
		go wsconn.keepAlive()
		go wsconn.messageSendHandler()
		return wsconn
	}
	// defer c.Close()
	// 将连接写入WsConn，方便随时可取
	wsconn.Done = make(chan struct{})
	go wsconn.keepAlive()
	go wsconn.messageReceiveHandler()
	go wsconn.messageSendHandler()
	return wsconn
}

func New() *WsConn {
	return createWsConn()
}

func GetWsConn() *WsConn {
	return wsconn
}
