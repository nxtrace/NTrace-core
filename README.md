<div align="center">

<img src="asset/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

# NextTrace

一款开源的可视化路由跟踪工具，使用 Golang 开发。

NextTrace 只是一个实验性项目，不推荐用于生产环境下。对于路由跟踪工具，我们依旧更加推荐 WorstTrace & Besttrace。

## How To Use

### Automated Install

```bash
# Note: This Script Only Supports Linux/macOS, Other Unix-Like Systems are UNAVAILABLE
curl -Ls https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh -O && sudo bash nt_install.sh
```

```bash
# GHPROXY镜像(在连接raw.githubusercontent.com网络不畅时使用)
curl -Ls https://ghproxy.com/https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh -O && sudo bash nt_install.sh
```

- `Release`里面为很多系统以及不同架构提供了编译好的二进制可执行文件，如果没有可以自行编译。
- 一些本项目的必要依赖在`Windows`上`Golang`底层实现不完全，所以目前`NextTrace`在`Windows`平台不可用。

### Fast Test

此脚本旨在快速测试服务器的到中国内地的线路，建议新手或者没有自定义`NextTrace`功能需求的用户使用。

```bash
curl -Ls https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/quicklytest.sh -O && sudo bash quicklytest.sh
```

```bash
# GHPROXY镜像(在连接raw.githubusercontent.com网络不畅时使用)
curl -Ls https://ghproxy.com/https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/quicklytest.sh -O && sudo bash quicklytest.sh
```

### Get Started

`NextTrace`默认使用`ICMP`协议发起`TraceRoute`请求，该协议同时支持`IPv4`和`IPv6`

```bash
# IPv4 ICMP Trace
nexttrace 1.0.0.1

# 表格打印（一次性输出全部跳数，需等待20-40秒）
nexttrace -table 1.0.0.1

# IPv6 ICMP Trace
nexttrace 2606:4700:4700::1111
```

`NextTrace`也可以使用`TCP`和`UDP`协议发起`Traceroute`请求，不过目前只支持`IPv4`

```bash
# TCP SYN Trace
nexttrace -T www.bing.com

# 可以自行指定端口[此处为443]，默认80端口
nexttrace -T -p 443 1.0.0.1

# UDP Trace
nexttrace -U 1.0.0.1

nexttrace -U -p 53 1.0.0.1
```

`NextTrace`也同样支持一些进阶功能，如 IP 反向解析、并发数控制、模式切换等

```bash
# 每一跳发送2个探测包
nexttrace -q 2 www.hkix.net

# 无并发，每次只发送一个探测包
nexttrace -r 1 www.hkix.net

# 打开IP反向解析功能，在IPv6的骨干网定位辅助有较大帮助
nexttrace -rdns www.bbix.net

# 特色功能：打印Route-Path图
# Route-Path图示例：
# AS6453 塔塔通信「Singapore『Singapore』」
#  ╭╯
#  ╰AS9299 Philippine Long Distance Telephone Co.「Philippines『Metro Manila』」
#  ╭╯
#  ╰AS36776 Five9 Inc.「Philippines『Metro Manila』」
#  ╭╯
#  ╰AS37963 阿里云「ALIDNS.COM『ALIDNS.COM』」
nexttrace -report www.time.com.my
```

`NextTrace`支持用户自主选择 IP 数据库（目前支持：`LeoMoeAPI`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`）

```bash
# 可以自行指定IP数据库[此处为IP.SB]，不指定则默认为LeoMoeAPI
nexttrace -d IP.SB
## 特别的：其中 ipinfo API 需要从ipinfo自行购买服务，如有需要可以clone本项目添加其提供的token自行编译
##        TOKEN填写路径：ipgeo/tokens.go
## 另外：由于IP.SB被滥用比较严重，会经常出现无法查询的问题，请知悉。
##      IPAPI.com限制调用较为严格，如有查询不到的情况，请几分钟后再试。
```

`NextTrace`支持参数混合使用

```bash
Example:
nexttrace -d IPInsight -m 20 -p 443 -q 5 -r 20 -rdns 1.1.1.1
nexttrace -T -q 2 -r 1 -rdns -table -report 2001:4860:4860::8888
```

### IP 数据库

目前使用的 IP 数据库默认为我们自己搭建的 API 服务，如果后期遇到滥用，我们可能会选择关闭。

我们也会在后期开放服务端源代码，您也可以根据该项目的源码自行搭建属于您的 API 服务器。

NextTrace 所有的的 IP 地理位置`API DEMO`可以参考[这里](https://github.com/xgadget-lab/nexttrace/blob/main/ipgeo/)

### 全部用法详见 Usage 菜单

```shell
Usage of nexttrace:
      'nexttrace [options] <hostname>' or 'nexttrace <hostname> [option...]'
Options:
  -T    Use TCP SYN for tracerouting (default port is 80)
  -U    Use UDP Package for tracerouting (default port is 53 in UDP)
  -V    Check Version
  -d string
        Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight, IPAPI.com] (default "LeoMoeAPI")
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
  -table
        Output trace results as table
  -report
        Route Path

```

## 项目截图

<img src=asset/screenshot.png alt="NextTrace Screenshot" width="683" height="688" />

## FAQ 常见问题

如果你在安装或者使用的时候遇到了问题，我们建议你不要把新建一个 `issue` 作为首选项

以下是我们推荐的排错流程：

1. 查看是否为常见问题 -> [前往 Github Wiki](https://github.com/xgadget-lab/nexttrace/wiki/FAQ---%E5%B8%B8%E8%A7%81%E9%97%AE%E9%A2%98%E8%A7%A3%E7%AD%94)
2. 不会软件的参数使用 -> [前往 Github Discussions](https://github.com/xgadget-lab/nexttrace/discussions)
3. 疑似 BUG、或者功能建议 -> [前往 Github Issues](https://github.com/xgadget-lab/nexttrace/issues)

## Thanks

[Vincent Young](https://github.com/missuo) (i@yyt.moe)

[Sam Sam](https://github.com/samleong123) (samsam123@samsam123.name.my)

[tsosunchia](https://github.com/tsosunchia)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p)

## IP Database Copyright

### IPv4 Database

#### China

|      ISP       |  类型  |  数据源   | 占比 |
| :------------: | :----: | :-------: | :--: |
| 电信/联通/移动 | 骨干网 | NextTrace | 10%  |
| 电信/联通/移动 | 城域网 | 埃文科技  | 90%  |

#### WorldWide

|   ISP   |  类型  |  数据源   | 占比 |
| :-----: | :----: | :-------: | :--: |
| Tier-01 | 骨干网 |  IPInfo   |  2%  |
| Tier-01 | 骨干网 | 埃文科技  |  3%  |
| Tier-01 | 骨干网 | IPInSight |  5%  |
| Tier-01 | 城域网 | IPInSight | 90%  |

|  ISP   |  类型  |  数据源   | 占比 |
| :----: | :----: | :-------: | :--: |
| Others | 骨干网 | IPInSight |  5%  |
| Others | 城域网 | IPInSight | 95%  |

### IPv6 Database

| ISP | 类型 |      数据源      | 占比 |
| :-: | :--: | :--------------: | :--: |
| All | 全部 | IP2Location Lite | 100% |

This product includes IP2Location LITE data available from <a href="https://lite.ip2location.com">https://lite.ip2location.com</a>.

### Others

其他第三方 API 尽管集成在本项目内，但是具体的 TOS 以及 AUP，请详见第三方 API 官网。如遇到 IP 数据错误，也请直接联系他们纠错。
