<div align="center">

<img src="asset/logo.png" height="200px"/>

</div>

# NextTrace

一款开源的可视化路由跟踪工具，使用Golang开发。

## How To Use

### Get Started

`NextTrace`默认使用`icmp`协议发起`TraceRoute`请求，该协议同时支持`IPv4`和`IPv6`

```bash
# IPv4 ICMP Trace
./nexttrace 1.0.0.1

# IPv6 ICMP Trace
./nexttrace 2606:4700:4700::1111
```

`NextTrace`也可以使用`TCP`和`UDP`协议发起`Traceroute`请求，不过目前只支持`IPv4`
```bash
# TCP SYN Trace
./nexttrace -T www.bing.com

# 可以自行指定端口[此处为443]，默认80端口
./nexttrace -T -p 443 1.0.0.1

# UDP Trace
./nexttrace -U 1.0.0.1

./nexttrace -U -p 53 1.0.0.1
```

### IP数据库

目前使用的IP数据库默认为我们自己搭建的API服务，如果后期遇到滥用，我们可能会选择关闭。

我们也会在后期开放服务端源代码，您也可以根据该项目的源码自行搭建属于您的API服务器。

NextTrace所有的的IP地理位置`API DEMO`可以参考[这里](https://github.com/xgadget-lab/nexttrace/blob/main/ipgeo/)

### 全部用法详见Usage菜单

```shell
Usage of nexttrace:
  -T    Use TCP SYN for tracerouting (default port is 80)
  -U    Use UDP Package for tracerouting (default port is 53 in UDP)
  -d string
        Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight] (default "LeoMoeAPI")
  -displayMode string
        Choose The Display Mode [table, classic] (default "table")
  -m int
        Set the max number of hops (max TTL to be reached). (default 30)
  -p int
        Set SYN Traceroute Port (default 80)
  -q int
        Set the number of probes per each hop. (default 3)
  -r int
        Set ParallelRequests number. It should be 1 when there is a multi-routing. (default 18)
  -rdns
        Set whether rDNS will be display
  -report
        Route Path
```
## 项目截图

![](asset/screenshot.png)

## Thanks

[Sam Sam](https://github.com/samleong123) (samsam123@samsam123.name.my)
[Vincent Young](https://github.com/missuo) (i@yyt.moe)

