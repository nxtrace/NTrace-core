<div align="center">

<img src="asset/logo.png" height="200px"/>

</div>

# NextTrace

一款开源的可视化路由跟踪工具，使用 Golang 开发。

## How To Use

### Install

```bash
curl -Ls https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh -O && sudo bash nt_install.sh
```

### Get Started

`NextTrace`默认使用`icmp`协议发起`TraceRoute`请求，该协议同时支持`IPv4`和`IPv6`

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
# 无并发，每次只发送一个探测包
nexttrace -r 1 www.hkix.net

# 打开IP反向解析功能，在IPv6的骨干网定位辅助有较大帮助
nexttrace -rdns www.bbix.net

# 联合使用
nexttrace -r 1 -q 1 -report www.time.com.my
```

### For Beginner

如果你实在不想去了解这些参数如何使用，或目的只是为了快速测试服务器的到中国内地的线路;<br>
那么建议你使用本仓库的 `quicklytest.sh` <br>
使用此脚本钱请先按照 [Install](#Install) 中的提示进行安装，才可使用;<br>
执行：
`sudo bash -c "$(curl -sL https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/quicklytest.sh)"`
，并按照提示进行操作即可。<br>

### IP 数据库

目前使用的 IP 数据库默认为我们自己搭建的 API 服务，如果后期遇到滥用，我们可能会选择关闭。

我们也会在后期开放服务端源代码，您也可以根据该项目的源码自行搭建属于您的 API 服务器。

NextTrace 所有的的 IP 地理位置`API DEMO`可以参考[这里](https://github.com/xgadget-lab/nexttrace/blob/main/ipgeo/)

### 全部用法详见 Usage 菜单

```shell
Usage of nexttrace:
  -T    Use TCP SYN for tracerouting (default port is 80)
  -U    Use UDP Package for tracerouting (default port is 53 in UDP)
  -V    Check Version
  -d string
        Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight] (default "LeoMoeAPI")
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
  -realtime
        Output trace results in runtime
  -report
        Route Path
  -table
        Output trace results as table
```

## 项目截图

![](asset/screenshot.png)

## FAQ 常见问题

如果你在安装或者使用的时候遇到了问题，我们建议你不要把新建一个 `issue` 作为首选项

或许可以在这里找到答案 -> [前往 Github Wiki](https://github.com/xgadget-lab/nexttrace/wiki/FAQ---%E5%B8%B8%E8%A7%81%E9%97%AE%E9%A2%98%E8%A7%A3%E7%AD%94)

## Thanks

[Vincent Young](https://github.com/missuo) (i@yyt.moe)

[Sam Sam](https://github.com/samleong123) (samsam123@samsam123.name.my)

[waiting4new](https://github.com/waiting4new)、[FFEE_CO](https://github.com/fkx4-p)、[nsnnns](https://github.com/tsosunchia)

## IP Database Copyright

### IPv4 Database

#### China MainLand

- 项目组自行维护 ~ 御三家骨干网数据 ~ 5%

- 埃文科技 Paid Database ~ 95%

**这里有朋友就要问了，为什么不全部使用埃文的付费库？**

埃文的库一直都不是最优选择，IPIP.NET 才是，但是因为他们不对私，所以我们只能选择价格更便宜的埃文库。

埃文家的数据库，在骨干网这块，准度可以说是非常糟糕，作为一款可视化的路由跟踪工具，骨干网的数据库准度非常重要。

所以我们选择了尝试自行去校准一部分骨干网数据，但是由于我们缺乏检测节点以及志愿者，所以这项工作可能会进展的尤其缓慢。

#### WorldWide

- 埃文科技 Paid Database ~ 15%

- IpInfo Free ~ 15%

- IPInSight Free ~ 70%

### IPv6 Database

This product includes IP2Location LITE data available from <a href="https://lite.ip2location.com">https://lite.ip2location.com</a>.

### Others

其他第三方 API 尽管集成在本项目内，但是具体的 TOS 以及 AUP，请详见第三方 API 官网。如遇到 IP 数据错误，也请直接联系他们纠错。
