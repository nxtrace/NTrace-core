package ipgeo

import (
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/wshandle"
	"github.com/tidwall/gjson"
)

/***
 * 原理介绍 By Leo
 * WebSocket 一共开启了一个发送和一个接收协程，在 New 了一个连接的实例对象后，不给予关闭，持续化连接
 * 当有新的IP请求时，一直在等待IP数据的发送协程接收到从 leo.go 的 sendIPRequest 函数发来的IP数据，向服务端发送数据
 * 由于实际使用时有大量并发，但是 ws 在同一时刻每次有且只能处理一次发送一条数据，所以必须给 ws 连接上互斥锁，保证每次只有一个协程访问
 * 运作模型可以理解为一个 Node 一直在等待数据，当获得一个新的任务后，转交给下一个协程，不再关注这个 Node 的下一步处理过程，并且回到空闲状态继续等待新的任务
***/

// IPPool IP 查询池 map - ip - ip channel
type IPPool struct {
	pool    map[string]chan IPGeoData
	poolMux sync.Mutex
}

var IPPools = IPPool{
	pool: make(map[string]chan IPGeoData),
}

func sendIPRequest(ip string) {
	wsConn := wshandle.GetWsConn()
	wsConn.MsgSendCh <- ip
}

func receiveParse() {
	// 获得连接实例
	wsConn := wshandle.GetWsConn()
	// 防止多协程抢夺一个ws连接，导致死锁，当一个协程获得ws的控制权后上锁
	wsConn.ConnMux.Lock()
	// 函数退出时解锁，给其他协程使用
	defer wsConn.ConnMux.Unlock()
	for {
		// 接收到了一条IP信息
		data := <-wsConn.MsgReceiveCh

		// json解析 -> data
		res := gjson.Parse(data)
		// 根据返回的IP信息，发送给对应等待回复的IP通道上
		var domain = res.Get("domain").String()

		if res.Get("domain").String() == "" {
			domain = res.Get("owner").String()
		}

		m := make(map[string][]string)
		err := json.Unmarshal([]byte(res.Get("router").String()), &m)
		if err != nil {
			// 此处是正常的，因为有些IP没有路由信息
		}

		lat, _ := strconv.ParseFloat(res.Get("lat").String(), 32)
		lng, _ := strconv.ParseFloat(res.Get("lng").String(), 32)

		IPPools.pool[gjson.Parse(data).Get("ip").String()] <- IPGeoData{
			Asnumber:  res.Get("asnumber").String(),
			Country:   res.Get("country").String(),
			CountryEn: res.Get("country_en").String(),
			Prov:      res.Get("prov").String(),
			ProvEn:    res.Get("prov_en").String(),
			City:      res.Get("city").String(),
			CityEn:    res.Get("city_en").String(),
			District:  res.Get("district").String(),
			Owner:     domain,
			Lat:       lat,
			Lng:       lng,
			Isp:       res.Get("isp").String(),
			Whois:     res.Get("whois").String(),
			Prefix:    res.Get("prefix").String(),
			Router:    m,
		}
	}
}

func LeoIP(ip string, timeout time.Duration, lang string, maptrace bool) (*IPGeoData, error) {
	// TODO: 根据lang的值请求中文/英文API
	// TODO: 根据maptrace的值决定是否请求经纬度信息
	if timeout < 5*time.Second {
		timeout = 5 * time.Second
	}

	// 缓存中没有找到IP信息，需要请求API获取
	IPPools.poolMux.Lock()
	// 如果之前已经被别的协程初始化过了就不用初始化了
	if IPPools.pool[ip] == nil {
		IPPools.pool[ip] = make(chan IPGeoData)
	}
	IPPools.poolMux.Unlock()
	// 发送请求
	sendIPRequest(ip)
	// 同步开启监听
	go receiveParse()

	// 拥塞，等待数据返回
	select {
	case res := <-IPPools.pool[ip]:
		return &res, nil
	// 5秒后依旧没有接收到返回的IP数据，不再等待，超时异常处理
	case <-time.After(timeout):
		// 这里不可以返回一个 nil，否则在访问对象内部的键值的时候会报空指针的 Fatal Error
		return &IPGeoData{}, errors.New("TimeOut")
	}
}
