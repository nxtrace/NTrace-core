<div align="center">

<img src="asset/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

<h1 align="center">
  <br>NextTrace<br>
</h1>


<h4 align="center">一款追求轻量化的开源可视化路由跟踪工具。</h4>

<p align="center">
  <a href="https://github.com/sjlleo/nexttrace/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/sjlleo/nexttrace/build.yml?branch=main&style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/sjlleo/nexttrace">
    <img src="https://goreportcard.com/badge/github.com/sjlleo/nexttrace?style=flat-square">
  </a>
  <a href="https://github.com/sjlleo/nexttrace/releases">
    <img src="https://img.shields.io/github/release/sjlleo/nexttrace/all.svg?style=flat-square">
  </a>
</p>


## How To Use

### Before Using

使用 NextTrace 之前，我们建议您先阅读 [#IP 数据以及精准度说明](https://github.com/sjlleo/nexttrace/blob/main/README_zh_CN.md#ip-%E6%95%B0%E6%8D%AE%E4%BB%A5%E5%8F%8A%E7%B2%BE%E5%87%86%E5%BA%A6%E8%AF%B4%E6%98%8E)，在了解您自己的对数据精准度需求以后再进行抉择。

### Automated Install

```bash
# Linux 一键安装脚本
bash <(curl -Ls https://raw.githubusercontent.com/sjlleo/nexttrace/main/nt_install.sh)

# GHPROXY 镜像（国内使用）
bash <(curl -Ls https://ghproxy.com/https://raw.githubusercontent.com/sjlleo/nexttrace/main/nt_install.sh)

# macOS brew 安装命令
brew tap xgadget-lab/nexttrace && brew install nexttrace
```

Windows 用户请直接前往 [Release](https://github.com/sjlleo/nexttrace/releases/latest) 下载编译后的二进制 exe 文件。

- `Release`里面为很多系统以及不同架构提供了编译好的二进制可执行文件，如果没有可以自行编译。
- 一些本项目的必要依赖在`Windows`上`Golang`底层实现不完全，所以目前`NextTrace`在`Windows`平台出于实验性支持阶段。

### Get Started

`NextTrace` 默认使用`ICMP`协议发起`TraceRoute`请求，该协议同时支持`IPv4`和`IPv6`

```bash
# IPv4 ICMP Trace
nexttrace 1.0.0.1

# 表格打印，使用 --table / -t 参数，将实时显示结果
nexttrace --table 1.0.0.1

# IPv6 ICMP Trace
nexttrace 2606:4700:4700::1111

# 禁用路径可视化 使用 --map / -M 参数
nexttrace koreacentral.blob.core.windows.net
# MapTrace URL: https://api.leo.moe/tracemap/html/c14e439e-3250-5310-8965-42a1e3545266.html
```

PS: 路由可视化的绘制模块由 [@tsosunchia](https://github.com/tsosunchia) 同学编写，具体代码可在 [tsosunchia/traceMap](https://github.com/tsosunchia/traceMap) 查看

需要注意的是，在 LeoMoeAPI 2.0 中，由于新增了了地理位置数据，**我们已经弃用 traceMap 插件中 OpenStreetMap API 的在线查询的部分，并且使用自己数据库内的位置信息**。

路由可视化功能因为需要每个 Hop 的地理位置坐标，而第三方 API 通常不提供此类信息，所以此功能目前只支持搭配 LeoMoeAPI 使用。

`NextTrace` 现已经支持快速测试，有一次性测试回程路由需求的朋友可以使用

```bash
# 北上广（电信+联通+移动+教育网）IPv4 / IPv6 ICMP 快速测试
nexttrace --fast-trace

# 也可以使用 TCP SYN 而非 ICMP 进行测试（不支持 IPv6）
nexttrace --fast-trace --tcp
```

`NextTrace` 已支持指定网卡进行路由跟踪

```bash
# 请注意 Lite 版本此参数不能和快速测试联用，如有需要请使用 enhanced 版本
# 使用 eth0 网卡
nexttrace --dev eth0 2606:4700:4700::1111

# 使用 eth0 网卡IP
# 网卡 IP 可以使用 ip a 或者 ifconfig 获取
# 使用网卡IP进行路由跟踪时需要注意跟踪的IP类型应该和网卡IP类型一致（如都为 IPv4）
nexttrace --source 204.98.134.56 9.9.9.9
```

`NextTrace` 也可以使用`TCP`和`UDP`协议发起`Traceroute`请求，不过目前只支持`IPv4`

```bash
# TCP SYN Trace
nexttrace --tcp www.bing.com

# 可以自行指定端口[此处为443]，默认80端口
nexttrace --tcp --port 443 1.0.0.1

# UDP Trace
nexttrace --udp 1.0.0.1

# 可以自行指定端口[此处为5353]，默认53端口
nexttrace --udp --port 5353 1.0.0.1
```

`NextTrace`也同样支持一些进阶功能，如 TTL 控制、并发数控制、模式切换等

```bash
# 每一跳发送2个探测包
nexttrace --queries 2 www.hkix.net

# 无并发，每次只发送一个探测包
nexttrace --parallel-requests 1 www.hkix.net

# 从TTL为5开始发送探测包，直到TTL为10结束
nexttrace --first 5 --max-hops 10 www.decix.net

# 关闭IP反向解析功能
nexttrace --no-rdns www.bbix.net

# 特色功能：打印Route-Path图
# Route-Path图示例：
# AS6453 塔塔通信「Singapore『Singapore』」
#  ╭╯
#  ╰AS9299 Philippine Long Distance Telephone Co.「Philippines『Metro Manila』」
#  ╭╯
#  ╰AS36776 Five9 Inc.「Philippines『Metro Manila』」
#  ╭╯
#  ╰AS37963 阿里云「ALIDNS.COM『ALIDNS.COM』」
nexttrace --route-path www.time.com.my
```

`NextTrace`支持用户自主选择 IP 数据库（目前支持：`LeoMoeAPI`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`）

```bash
# 可以自行指定IP数据库[此处为IP.SB]，不指定则默认为LeoMoeAPI
nexttrace --data-provider IP.SB
## 特别的：其中 ipinfo API 需要从 ipinfo 自行购买服务，如有需要可以 clone 本项目添加其提供的 token 自行编译
##        TOKEN填写路径：ipgeo/tokens.go

## 另外：由于IP.SB被滥用比较严重，会经常出现无法查询的问题，请知悉。
##      IPAPI.com限制调用较为严格，如有查询不到的情况，请几分钟后再试。
```

`NextTrace`支持使用混合参数和简略参数

```bash
Example:
nexttrace --data-provider IPAPI.com --max-hops 20 --tcp --port 443 --queries 5 --no-rdns 1.1.1.1
nexttrace -tcp --queries 2 --parallel-requests 1 --table --route-path 2001:4860:4860::8888

Equivalent to:
nexttrace -d IPAPI.com -m 20 -T -p 443 -q 5 -n 1.1.1.1
nexttrace -T -q 2 --parallel-requests 1 -t -R 2001:4860:4860::8888
```

### 全部用法详见 Usage 菜单

```shell
Usage: nexttrace [-h|--help] [-T|--tcp] [-U|--udp] [-F|--fast-trace] [-p|--port
                 <integer>] [-q|--queries <integer>] [--parallel-requests
                 <integer>] [-m|--max-hops <integer>] [-d|--data-provider
                 (IP.SB|IPInfo|IPInsight|IPAPI.com)] [-n|--no-rdns]
                 [-r|--route-path] [-o|--output] [-t|--table] [-c|--classic]
                 [-f|--first <integer>] [-M|--map] [-v|--version] [-s|--source
                 "<value>"] [-D|--dev "<value>"] [-R|--route] [-z|--send-time
                 <integer>] [-i|--ttl-time <integer>]
                 [IP Address or Domain name]
Arguments:

  -h  --help                         Print help information
  -T  --tcp                          Use TCP SYN for tracerouting (default port
                                     is 80)
  -U  --udp                          Use UDP SYN for tracerouting (default port
                                     is 53)
  -F  --fast-trace                   One-Key Fast Trace to China ISPs
  -p  --port                         Set the destination port to use. It is
                                     either initial udp port value for
                                     "default"method (incremented by each
                                     probe, default is 33434), or initial seq
                                     for "icmp" (incremented as well, default
                                     from 1), or some constantdestination port
                                     for other methods (with default of 80 for
                                     "tcp", 53 for "udp", etc.)
  -q  --queries                      Set the number of probes per each hop.
                                     Default: 3
      --parallel-requests            Set ParallelRequests number. It should be
                                     1 when there is a multi-routing. Default:
                                     18
  -m  --max-hops                     Set the max number of hops (max TTL to be
                                     reached). Default: 30
  -d  --data-provider                Choose IP Geograph Data Provider
                                     [LeoMoeAPI,IP.SB, IPInfo, IPInsight,
                                     IPAPI.com]. Default: LeoMoeAPI
  -n  --no-rdns                       Do not resolve IP addresses to their
                                     domain names
  -r  --route-path                   Print traceroute hop path by ASN and
                                     location
  -o  --output                       Write trace result to file
                                     (RealTimePrinter ONLY)
  -t  --table                        Output trace results as table
  -c  --classic                      Classic Output trace results like
                                     BestTrace
  -f  --first                        Start from the first_ttl hop (instead from
                                     1). Default: 1
  -M  --map                          Disable Print Trace Map Function
  -v  --version                      Print version info and exit
  -s  --source                       Use source src_addr for outgoing packets
  -D  --dev                          Use the following Network Devices as the
                                     source address in outgoing packets
  -R  --route                        Show Routing Table [Provided By BGP.Tools]
  -z  --send-time                    Set the time interval for sending every
                                     packet. Useful when some routers use
                                     rate-limit for ICMP messages.. Default: 0
  -i  --ttl-time                     Set the time interval for sending packets
                                     groups by TTL. Useful when some routers
                                     use rate-limit for ICMP messages..
                                     Default: 500
  -g  --language                     Choose the language for displaying [en,
                                     cn]. Default: cn
```

## 项目截图

![image](https://user-images.githubusercontent.com/13616352/208289553-7f633f9c-7356-40d1-bbc4-cc2687419cca.png)

![image](https://user-images.githubusercontent.com/13616352/208289568-2a135c2d-ae4a-4a3e-8a43-f5a9a87ade4a.png)

## 第三方 IP 数据库 API 开发接口

NextTrace 所有的的 IP 地理位置 `API DEMO` 可以参考[这里](https://github.com/sjlleo/nexttrace/blob/main/ipgeo/)

你可以在这里添加你自己的 API 接口，为了 NextTrace 能够正确显示你接口中的内容，请参考 `leo.go` 中所需要的信息

✨NextTrace `LeoMoeAPI` 的后端 Demo

[GitHub - sjlleo/nexttrace-backend: NextTrace BackEnd](https://github.com/sjlleo/nexttrace-backend)

## NextTrace Enhanced

https://github.com/OwO-Network/nexttrace-enhanced

## Credits

BGP.TOOLS 提供了本项目的一些数据支持，在此表示由衷地感谢。

[Vincent Young](https://github.com/missuo) (i@yyt.moe)

[Sam Sam](https://github.com/samleong123) (samsam123@samsam123.name.my)

[tsosunchia](https://github.com/tsosunchia)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p)

## Others

其他第三方 API 尽管集成在本项目内，但是具体的 TOS 以及 AUP，请详见第三方 API 官网。如遇到 IP 数据错误，也请直接联系他们纠错。

## IP 数据以及精准度说明

NextTrace 有多个数据源可以选择，目前默认使用的 LeoMoeAPI 为我们项目维护的数据源。

LeoMoeAPI 早期数据主要来自 IPInsight、IPInfo，随着项目发展，越来越多的志愿者参与进了这个项目。目前 LeoMoeAPI 有近一半的数据是社区提供的，而另外一半主要来自于包含 IPInfo、IPData、BigDataCloud、IPGeoLocation 在内的多个第三方数据。

LeoMoeAPI 的骨干网数据有近 70% 是社区自发反馈又或者是项目组成员校准的，这给本项目的路由跟踪基础功能带来了一定的保证，但是全球骨干网的体量庞大，我们并无能力如 IPIP 等商业公司拥有海量监测节点，这使得 LeoMoeAPI 的数据精准度无法和形如 BestTrace（IPIP）相提并论。

LeoMoeAPI 已经尽力校准了比较常见的骨干网路由，这部分在测试的时候经常会命中，但是如果遇到封闭型 ISP 的路由，大概率可以遇到错误，此类数据不仅是我们，哪怕 IPInsight、IPInfo 也无法正确定位，目前只有 IPIP 能够标记正确，如对此类数据的精确性有着非常高的要求，请务必使用 BestTrace 作为首选。

我们不保证我们的数据一定会及时更新，也不保证数据的精确性，我们希望您在发现数据错误的时候可以前往 issue 页面提交错误报告，谢谢。

当您使用 LeoMoeAPI 即视为您已经完全了解 NextTrace LeoMoeAPI 的数据精确性，并且同意如果您引用 LeoMoeAPI 其中的数据从而引发的一切问题，均由您自己承担。
