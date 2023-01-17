<div align="center">

<img src="asset/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

## NextTrace

An open source visual routing tool that pursues light weight, developed using Golang.

2022/12/18: Due to time and effort, it is becoming more and more difficult to maintain 2 branches at the same time, so I will be phasing out support for the NextTrace Enhanced version in the near future. I will resume updating the `Enhanced` version when I have more time.

## 公告

我今天看到了一些非常难过的事情，一些用户在 BestTrace 和 WorstTrace 下面宣传 NextTrace 的完全可替代性。

这么做是不正确的，NextTrace 从来都不是一个从零开始的软件，NextTrace 之所以能够拥有某些功能特性，是因为吸取了 BestTrace 、WorstTrace 的一些想法。

我们希望您在使用的时候知晓这一点，**我们是站在巨人的肩膀上，而尊重其他软件作者，向他们或者是我们提交 Bug 或贡献代码，才是推动整个 traceroute 工具的软件多样化发展的最好方式**。

NextTrace 并不追求成为一个替代者，同类软件越多样化，才能满足更多人的需求，这才是我们希望看到的，而去诋毁其他软件，这违背了我们对于开发 NextTrace 的初衷。

我们希望看到这条公告的朋友应该主动删除自己过激的言论，如果您有任何问题或建议，请随时在我们的社区中发表。

## LeoMoeAPI Credit

NextTrace 重点在于研究 Go 语言 Traceroute 的实现，其 LeoMoeAPI 的地理位置信息并没有原始数据的支撑，故也不可能有商用版本。

LeoMoeAPI 存在部分社区贡献者校准的数据，也包含了部分其他第三方数据库的数据，这些数据的所有权归校准者、第三方数据库所有，**仅供路由跟踪地理位置的展示参考使用**，我们不对数据提供准度做任何保证，请尊重他们的成果，如用于其他用途后果自负，特此告知。

1. 对于辛勤提供马来西亚地区节点的 samleong123、全球节点的 TOHUNET Looking Glass 以及来自 Misaka 的 Ping.sx 表示感谢，目前 80% 以上的可靠校准数据出自这些节点的 ping / mtr 报告。

2. 同时感谢 isyekong 在基于 rDNS 校准上思路以及数据上做出的贡献，LeoMoeAPI 正在加快对 rDNS 的解析功能研发，目前已经做到部分骨干网的地理位置自动化解析，但存在一定误判。
我们希望 NextTrace 在未来能成为对 One-Man ISP 友好的 Traceroute 工具，我们也在尽可能完善对这些 ASN 的微型骨干网的校准。

3. 在开发上，我要由衷感谢 missuo 以及 zhshch 在 Go 交叉编译、设计理念以及 TCP/UDP Traceroute 重构上的帮助、tsosunchia 在 TraceMap 上的倾力支持。

4. 我还要感谢 FFEE_CO、TheresaQWQ、stydxm 和其他朋友的帮助。LeoMoeAPI自首次发布以来得到了很多各方面的支持，所以我想把他们都归功于此。

我们希望您能够在使用时尽可能多多反馈 IP 地理位置错误（详见 issue），这样它就能够在第一时间得到校准，他人也会因此而受益。

NextTrace focuses on Golang Traceroute implementations, and its LeoMoeAPI geolocation information is not supported by raw data, so a commercial version is not possible.

The LeoMoeAPI data is subject to copyright restrictions from multiple data sources, and is only used for the purpose of displaying the geolocation of route tracing.

1. We would like to credit samleong123 for providing nodes in Malaysia, TOHUNET Looking Glass for global nodes, and Ping.sx from Misaka, where more than 80% of reliable calibration data comes from ping/mtr reports.

2. At the same time, we would like to credit isyekong for their contribution on rDNS-based calibration ideas and data. LeoMoeAPI is accelerating the development of rDNS resolution function, and has already achieved automated geolocation resolution for some backbone networks, but there are some misjudgments. We hope that NextTrace will become a One-Man ISP-friendly traceroute tool in the future, and we are working on improving the calibration of these ASN micro-backbones as much as possible.

3. In terms of development, I would like to credit missuo and zhshch for their help with Go cross-compilation, design concepts and TCP/UDP Traceroute refactoring, and tsosunchia for their support on TraceMap.

4. I would also like to credit FFEE_CO, TheresaQWQ, stydxm and others for their help. leoMoeAPI has received a lot of support since its first release, so I would like to credit them all!

We hope you can give us as much feedback as possible on IP geolocation errors (see issue) so that it can be calibrated in the first place and others can benefit from it.

## How To Use

Document Language: English | [简体中文](README_zh_CN.md)

### Automated Installation

```bash
# Linux one-click install script
bash <(curl -Ls https://raw.githubusercontent.com/sjlleo/nexttrace/main/nt_install.sh)

# macOS brew install command
brew tap xgadget-lab/nexttrace && brew install nexttrace
```

Windows users please go to [Release Page](https://github.com/sjlleo/nexttrace/releases/latest) directly and download exe file.

- `Release` provides compiled executables for many systems and architectures, if not, you can compile it yourself.
- Some of the necessary dependencies of this project are not fully implemented in `Golang` on `Windows`, so currently `NextTrace` is experimental on `Windows` platform.

### Get Started

`NextTrace` uses the `ICMP` protocol to perform TraceRoute requests by default, which supports both `IPv4` and `IPv6`

```bash
# IPv4 ICMP Trace
nexttrace 1.0.0.1

# Form printing (output all hops at one time, wait 20-40 seconds)
nexttrace --table 1.0.0.1

# IPv6 ICMP Trace
nexttrace 2606:4700:4700::1111

# Path Visualization With the -M parameter, a map URL is returned
nexttrace --map koreacentral.blob.core.windows.net
# MapTrace URL: https://api.leo.moe/tracemap/html/c14e439e-3250-5310-8965-42a1e3545266.html
```

PS: The routing visualization drawing module was written by [@tsosunchia](https://github.com/tsosunchia), and the specific code can be viewed at [tsosunchia/traceMap](https://github.com/tsosunchia/traceMap).

Note that in LeoMoeAPI 2.0, due to the addition of geographical location data, **we have deprecated the online query part of the OpenStreetMap API in the traceMap plugin and are using location information from our own database**.

The routing visualization function requires the geographical coordinates of each Hop, but third-party APIs generally do not provide this information, so this function is currently only supported when used with LeoMoeAPI.

`NextTrace` now supports quick testing, and friends who have a one-time backhaul routing test requirement can use it

```bash
# IPv4 ICMP Fast Test (Beijing + Shanghai + Guangzhou + Hangzhou) in China Telecom / Unicom / Mobile / Education Network
nexttrace -F

# You can also use TCP SYN for testing
nexttrace -F -T
```

`NextTrace` already supports route tracing for specified Network Devices

```bash
# Use eth0 network interface
nexttrace --dev eth0 2606:4700:4700::1111

# Use eth0 network interface's IP
# When using the network interface's IP for route tracing, note that the IP type to be traced should be the same as network interface's IP type (e.g. both IPv4)
nexttrace --source 204.98.134.56 9.9.9.9
```

`NextTrace` can also use `TCP` and `UDP` protocols to perform `Traceroute` requests, but these protocols only supports `IPv4` now

```bash
# TCP SYN Trace
nexttrace -T www.bing.com

# You can specify the port by yourself [here is 443], the default port is 80
nexttrace -T -p 443 1.0.0.1

# UDP Trace
nexttrace -U 1.0.0.1

nexttrace -U -p 53 1.0.0.1
```

`NextTrace` also supports some advanced functions, such as ttl control, concurrent probe packet count control, mode switching, etc.

```bash
# Send 2 probe packets per hop
nexttrace -q 2 www.hkix.net

# No concurrent probe packets, only one probe packet is sent at a time
nexttrace -r 1 www.hkix.net

# Start Trace with TTL of 5, end at TTL of 10
nexttrace -f 5 -m 10 www.decix.net

# Turn off the IP reverse parsing function
nexttrace -n www.bbix.net

# Feature: print Route-Path diagram
# Route-Path diagram example:
# AS6453 Tata Communication「Singapore『Singapore』」
#  ╭╯
#  ╰AS9299 Philippine Long Distance Telephone Co.「Philippines『Metro Manila』」
#  ╭╯
#  ╰AS36776 Five9 Inc.「Philippines『Metro Manila』」
#  ╭╯
#  ╰AS37963 Aliyun「ALIDNS.COM『ALIDNS.COM』」
nexttrace --route-path www.time.com.my
```

`NextTrace` supports users to select their own IP API (currently supports: `LeoMoeAPI`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`)

```bash
# You can specify the IP database by yourself [IP.SB here], if not specified, LeoMoeAPI will be used
nexttrace -d IP.SB
## Note that the ipinfo API needs users to purchase services from ipinfo. If necessary, you can clone this project, add the token provided by ipinfo and compile it yourself
## Fill the token to: ipgeo/tokens.go
## Please be aware: Due to the serious abuse of IP.SB, you will often be not able to query IP data from this source
## IPAPI.com has a stricter restiction on API calls, if you can't query IP data from this source, please try again in a few minutes.
```

`NextTrace` supports mixed parameters

```bash
Example:
nexttrace --data-provider LeoMoeAPI -m 20 -p 443 -q 5 -n 1.1.1.1
nexttrace -T -q 2 -r 1 --table --route-path 2001:4860:4860::8888
```

### IP Database

#### We use [bgp.tools](https://bgp.tools) as a data provider for routing tables.

NextTrace BackEnd is now open-source.

https://github.com/sjlleo/nexttrace-backend

All NextTrace IP geolocation `API DEMO` can refer to [here](https://github.com/xgadget-lab/nexttrace/blob/main/ipgeo/)

### For full usage list, please refer to the usage menu

```shell
Usage of nexttrace:
      'nexttrace [options] <hostname>' or 'nexttrace <hostname> [option...]'
Options:
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

## Project screenshot

![image](https://user-images.githubusercontent.com/13616352/208289553-7f633f9c-7356-40d1-bbc4-cc2687419cca.png)

![image](https://user-images.githubusercontent.com/13616352/208289568-2a135c2d-ae4a-4a3e-8a43-f5a9a87ade4a.png)


## NextTrace Enhanced

`NextTrace Enhanced` is an enhanced version for enthusiasts, `Enhanced` provides trace route calls in the form of Web API and a simple Looking Glass webpage with built-in visualization.

The `Enhanced` version supports many functions that the `lite` version does not have, such as the ability to customize the timeout period, and the ability to specify TTL as the starting point for route tracking, etc. For ordinary users, the `lite` version is usually enough.

https://github.com/OwO-Network/nexttrace-enhanced

# 

## FAQ Frequently Asked Questions

If you encounter problems while installing or using it, we do not recommend you to choose creating an `issue` as a preference

Here is our recommended troubleshooting process:

1. Check if it is already in FAQ -> [Go to Github Wiki](https://github.com/xgadget-lab/nexttrace/wiki/FAQ---%E5%B8%B8%E8%A7%81%E9%97%AE%E9%A2%98%E8%A7%A3%E7%AD%94)
2. Suspected bug or feature suggestion -> [Go to Github Issues](https://github.com/xgadget-lab/nexttrace/issues)

## JetBrain Support

#### This Project uses [JetBrain Open-Source Project License](https://jb.gg/OpenSourceSupport). We Proudly Develop By Goland.

<img src="https://resources.jetbrains.com/storage/products/company/brand/logos/GoLand.png" title="" alt="GoLand logo" width="331">

## Credits

BGP.TOOLS provided some data support for this project and we would like to express our sincere gratitude.

[Vincent Young](https://github.com/missuo) (i@yyt.moe)

[Sam Sam](https://github.com/samleong123) (samsam123@samsam123.name.my)

[tsosunchia](https://github.com/tsosunchia)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p) 

### Others

Although other third-party APIs are integrated in this project, please refer to the official website of the third-party APIs for specific TOS and AUP. If you encounter IP data errors, please contact them directly to correct them.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=xgadget-lab/nexttrace&type=Date)](https://star-history.com/#xgadget-lab/nexttrace&Date)
