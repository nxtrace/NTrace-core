<div align="center">

<img src="asset/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

## NextTrace

一款追求轻量的开源可视化路由跟踪工具，使用 Golang 开发。

如果您对 NextTrace 项目本身感兴趣，可以阅读 [有关 NextTrace 的一些碎碎念](https://leo.moe/annoucement/nexttrace.html) 或许可以帮您解决疑惑。

2022/12/18: 由于时间精力的关系，同时维护2个分支变得愈发困难，近期我将逐步停止 NextTrace Enhanced 版本的支持，如有觉得 NextTrace Enhanced 一些不错的功能，可以在 issue 里面提出来。待我重新有富裕时间时重新恢复对 `Enhanced` 版本的更新。

## How To Use

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

# 表格打印（一次性输出全部跳数，需等待20-40秒）
nexttrace --table 1.0.0.1

# IPv6 ICMP Trace
nexttrace 2606:4700:4700::1111

# 路径可视化 使用 -M 参数，将返回一个地图 URL
nexttrace -M koreacentral.blob.core.windows.net
# MapTrace URL: https://api.leo.moe/tracemap/html/c14e439e-3250-5310-8965-42a1e3545266.html
```

PS: 路由可视化的绘制模块由 [@tsosunchia](https://github.com/tsosunchia) 同学编写，具体代码可在 [tsosunchia/traceMap](https://github.com/tsosunchia/traceMap) 查看

需要注意的是，在 LeoMoeAPI 2.0 中，由于新增了了地理位置数据，**我们已经弃用 traceMap 插件中 OpenStreetMap API 的在线查询的部分，并且使用自己数据库内的位置信息**。

路由可视化功能因为需要每个 Hop 的地理位置坐标，而第三方 API 通常不提供此类信息，所以此功能目前只支持搭配 LeoMoeAPI 使用。

`NextTrace` 现已经支持快速测试，有一次性测试回程路由需求的朋友可以使用

```bash
# 北上广（电信+联通+移动+教育网）IPv4 ICMP 快速测试
nexttrace -F

# 也可以使用 TCP SYN 而非 ICMP 进行测试
nexttrace -F -T
```

`NextTrace` 已支持指定网卡进行路由跟踪

```bash
# 请注意 Lite 版本此参数不能和快速测试联用，如有需要请使用 enhanced 版本
# 使用 eth0 网卡
nexttrace -D eth0 2606:4700:4700::1111

# 使用 eth0 网卡IP
# 网卡 IP 可以使用 ip a 或者 ifconfig 获取
# 使用网卡IP进行路由跟踪时需要注意跟踪的IP类型应该和网卡IP类型一致（如都为 IPv4）
nexttrace -S 204.98.134.56 9.9.9.9
```

`NextTrace` 也可以使用`TCP`和`UDP`协议发起`Traceroute`请求，不过目前只支持`IPv4`

```bash
# TCP SYN Trace
nexttrace -T www.bing.com

# 可以自行指定端口[此处为443]，默认80端口
nexttrace -T -p 443 1.0.0.1

# UDP Trace
nexttrace -U 1.0.0.1

nexttrace -U -p 53 1.0.0.1
```

`NextTrace`也同样支持一些进阶功能，如 TTL 控制、并发数控制、模式切换等

```bash
# 每一跳发送2个探测包
nexttrace -q 2 www.hkix.net

# 无并发，每次只发送一个探测包
nexttrace -r 1 www.hkix.net

# 从TTL为5开始发送探测包，直到TTL为10结束
nexttrace -b 5 -m 10 www.decix.net

# 关闭IP反向解析功能
nexttrace -n www.bbix.net

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
nexttrace -d IP.SB
## 特别的：其中 ipinfo API 需要从ipinfo自行购买服务，如有需要可以clone本项目添加其提供的token自行编译
##        TOKEN填写路径：ipgeo/tokens.go
## 另外：由于IP.SB被滥用比较严重，会经常出现无法查询的问题，请知悉。
##      IPAPI.com限制调用较为严格，如有查询不到的情况，请几分钟后再试。
```

`NextTrace`支持参数混合使用

```bash
Example:
nexttrace --data-provider LeoMoeAPI -m 20 -p 443 -q 5 --parallel-requests 20 -n 1.1.1.1
nexttrace -T -q 2 --table --route-path 2001:4860:4860::8888
```

### IP 数据库

我们使用[bgp.tools](https://bgp.tools)作为路由表功能的数据提供者。

✨NextTrace `LeoMoeAPI` 的后端也开源啦

[GitHub - sjlleo/nexttrace-backend: NextTrace BackEnd](https://github.com/sjlleo/nexttrace-backend)

NextTrace 所有的的 IP 地理位置`API DEMO`可以参考[这里](https://github.com/xgadget-lab/nexttrace/blob/main/ipgeo/)

### 全部用法详见 Usage 菜单

```shell
Usage of nexttrace:
      'nexttrace [options] <hostname>' or 'nexttrace <hostname> [option...]'
Options:
  -D string
        Use the following Network Devices as the source address in outgoing packets
  -S string
        Use the following IP address as the source address in outgoing packets
  -T    Use TCP SYN for tracerouting (default port is 80)
  -U    Use UDP Package for tracerouting (default port is 53 in UDP)
  -V    Print Version
  -b int
        Set The Begin TTL (default 1)
  -d string
        Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight, IPAPI.com] (default "LeoMoeAPI")
  -f    One-Key Fast Traceroute
  -m int
        Set the max number of hops (max TTL to be reached). (default 30)
  -n    Disable IP Reverse DNS lookup
  -p int
        Set SYN Traceroute Port (default 80)
  -q int
        Set the number of probes per each hop. (default 3)
  -r int
        Set ParallelRequests number. It should be 1 when there is a multi-routing. (default 18)
  -report
        Route Path
  -table
        Output trace results as table
```

## 项目截图

![image](https://user-images.githubusercontent.com/13616352/208289553-7f633f9c-7356-40d1-bbc4-cc2687419cca.png)

![image](https://user-images.githubusercontent.com/13616352/208289568-2a135c2d-ae4a-4a3e-8a43-f5a9a87ade4a.png)

## NextTrace Enhanced

`NextTrace Enhanced` 是面向发烧友的增强版，`Enhanced`提供Web API形式的路由跟踪调用，以及一个简单的自带可视化的Looking Glass网页。

`Enhanced` 版本支持很多`lite`版本没有的功能，如能够自定义设置超时时间，也能指定TTL作为起点进行路由跟踪等，对于普通用户来说，通常`lite`版本已经足够完成大部分需要。

**很遗憾，由于时间精力的关系，`Enhanced` 版本将在很长一段时间不会再收到新的更新补丁，我们将继续维护 `Standard` 版本。**

https://github.com/OwO-Network/nexttrace-enhanced

## Thanks

BGP.TOOLS 提供了本项目的一些数据支持，在此表示由衷地感谢。

[Vincent Young](https://github.com/missuo) (i@yyt.moe)

[Sam Sam](https://github.com/samleong123) (samsam123@samsam123.name.my)

[tsosunchia](https://github.com/tsosunchia)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p)

### Others

其他第三方 API 尽管集成在本项目内，但是具体的 TOS 以及 AUP，请详见第三方 API 官网。如遇到 IP 数据错误，也请直接联系他们纠错。
