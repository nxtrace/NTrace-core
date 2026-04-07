<div align="center">

<img src="assets/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

<h1 align="center">
  <br>NextTrace<br>
</h1>

<h4 align="center">一款追求轻量化的开源可视化路由跟踪工具。</h4>

---

<h6 align="center">主页：www.nxtrace.org</h6>

<p align="center">
  <a href="https://github.com/nxtrace/NTrace-dev/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/nxtrace/NTrace-dev/build.yml?branch=main&style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/nxtrace/NTrace-dev">
    <img src="https://goreportcard.com/badge/github.com/nxtrace/NTrace-dev?style=flat-square">
  </a>
  <a href="https://github.com/nxtrace/NTrace-dev/releases">
    <img src="https://img.shields.io/github/release/nxtrace/NTrace-dev/all.svg?style=flat-square">
  </a>
</p>

## IAAS Sponsor

<div style="text-align: center;">
    <a href="https://dmit.io">
        <img src="https://assets.nxtrace.org/dmit.svg" width="170.7" height="62.9">
    </a>
    &nbsp;&nbsp;&nbsp;&nbsp;
    <a href="https://misaka.io" >
        <img src="https://assets.nxtrace.org/misaka.svg" width="170.7" height="62.9">
    </a>
    &nbsp;&nbsp;&nbsp;&nbsp;
    <a href="https://portal.saltyfish.io" >
        <img src="https://assets.nxtrace.org/snapstack.svg" width="170.7" height="62.9">
    </a>
</div>

我们非常感谢 [DMIT](https://dmit.io)、 [Misaka](https://misaka.io) 和 [SnapStack](https://portal.saltyfish.io) 提供了支持本项目所需的网络基础设施。

## How To Use

Document Language: [English](README.md) | 简体中文

⚠️ 请注意：我们欢迎来自社区的PR提交，但是请将您的PR提交至 [NTrace-dev](https://github.com/nxtrace/NTrace-dev) 仓库，而不是 [NTrace-core](https://github.com/nxtrace/NTrace-core) 仓库。<br>
关于NTrace-dev和NTrace-core两个仓库的说明：<br>
二者将大体上保持一致。所有的开发工作均在NTrace-dev仓库中进行。NTrace-dev仓库首先发布新版本，在稳定运行一段时间后（时长不定），我们会把版本同步至NTrace-core。这意味着NTrace-dev仓库充当了一个“测试版”的角色。<br>
请注意，版本同步也存在例外。如果NTrace-dev的某个版本出现了严重的bug，NTrace-core会跳过这一有缺陷的版本，直接同步到下一个修复了该问题的版本。

### Before Using

使用 NextTrace 之前，我们建议您先阅读 [#IP 数据以及精准度说明](https://github.com/nxtrace/NTrace-core/blob/main/README_zh_CN.md#ip-%E6%95%B0%E6%8D%AE%E4%BB%A5%E5%8F%8A%E7%B2%BE%E5%87%86%E5%BA%A6%E8%AF%B4%E6%98%8E)，在了解您自己的对数据精准度需求以后再进行抉择。

### Automated Install

- Linux
  - 一键安装脚本
    ```shell
    curl -sL https://nxtrace.org/nt | bash
    ```
  - 从 nxtrace的APT源安装
    - 支持 AMD64/ARM64 架构
      ```shell
      curl -fsSL https://github.com/nxtrace/nexttrace-debs/releases/latest/download/nexttrace-archive-keyring.gpg | sudo tee /etc/apt/keyrings/nexttrace.gpg >/dev/null
      echo "Types: deb
      URIs: https://github.com/nxtrace/nexttrace-debs/releases/latest/download/
      Suites: ./
      Signed-By: /etc/apt/keyrings/nexttrace.gpg" | sudo tee /etc/apt/sources.list.d/nexttrace.sources >/dev/null
      sudo apt update
      sudo apt install nexttrace
      ```
    - APT源由 wcbing, nxtrace 维护

  - Arch Linux AUR 安装命令
    - 直接下载bin包(仅支持amd64)
      ```shell
      yay -S nexttrace-bin
      ```
    - 从源码构建(仅支持amd64)
      ```shell
      yay -S nexttrace
      ```
    - AUR 的构建分别由 ouuan, huyz 维护

  - Linuxbrew 安装命令

    同macOS Homebrew安装方法(homebrew-core版仅支持amd64)

  - deepin 安装命令

    ```shell
    apt install nexttrace
    ```

  - [x-cmd](https://cn.x-cmd.com/pkg/nexttrace) 安装命令

    ```shell
    x env use nexttrace
    ```

  - Termux 安装命令
    ```shell
    pkg install root-repo
    pkg install nexttrace
    ```
  - ImmortalWrt 安装命令
    ```shell
    opkg install nexttrace
    ```

- macOS
  - macOS Homebrew 安装命令
    - homebrew-core版
      ```shell
      brew install nexttrace
      ```
    - 本仓库ACTIONS自动构建版(更新更快)
      ```shell
      brew tap nxtrace/nexttrace && brew install nxtrace/nexttrace/nexttrace
      ```
    - homebrew-core 构建由 chenrui333 维护，请注意该版本更新可能会落后仓库Action自动构建版本

- Windows
  - Windows WinGet 安装命令
    - WinGet 版
      ```powershell
      winget install nexttrace
      ```
    - WinGet 构建由 Dragon1573 维护

  - Windows Scoop 安装命令
    - scoop-extras 版
    ```powershell
    scoop bucket add extras && scoop install extras/nexttrace
    ```

    - scoop-extra 由 soenggam 维护

请注意，以上多种安装方式的仓库均由开源爱好者自行维护，不保证可用性和及时更新，如遇到问题请联系仓库维护者解决，或使用本项目官方编译提供的二进制包。

### Manual Install

- 下载预编译的可执行程序

  对于以上方法没有涵盖的用户，请直接前往 [Release](https://www.nxtrace.org/downloads) 下载编译好的二进制可执行文件。
  - `Release`里面为很多系统以及不同架构提供了编译好的二进制可执行文件，如果没有可以自行编译。
  - 一些本项目的必要依赖在`Windows`上`Golang`底层实现不完全，所以目前`NextTrace`在`Windows`平台出于实验性支持阶段。

### 版本说明

从本版本开始，NextTrace 在同一 release tag 下发布 **三种构建版本**，按需选用：

| 功能                    | `nexttrace`（完整版） | `nexttrace-tiny` |   `ntr`    |
| ----------------------- | :-------------------: | :--------------: | :--------: |
| 常规 traceroute         |          ✅           |        ✅        |     —      |
| 独立 MTU（`--mtu`）     |          ✅           |        ✅        |     —      |
| MTR TUI                 |          ✅           |        —         | ✅（默认） |
| MTR 报告（`-r`）        |          ✅           |        —         |     ✅     |
| MTR 宽报告（`-w`）      |          ✅           |        —         |     ✅     |
| MTR 原始输出（`--raw`） |          ✅           |        —         |     ✅     |
| Globalping（`--from`）  |          ✅           |        —         |     —      |
| WebUI（`--deploy`）     |          ✅           |        —         |     —      |
| 快速跟踪（`-F`）        |          ✅           |        ✅        |     —      |
| 默认运行模式            |      traceroute       |    traceroute    |  MTR TUI   |
| 二进制名                |      `nexttrace`      | `nexttrace-tiny` |   `ntr`    |

> **注意：** 包管理器（Homebrew、AUR、Scoop 等）目前仅安装 **完整版**（`nexttrace`）。

### 功能对比

- **`nexttrace`** — 完整版。包含所有功能：traceroute、MTR、Globalping 与 WebUI。
- **`nexttrace-tiny`** — 精简版。仅保留常规 traceroute，不含 MTR / Globalping / WebUI。适合嵌入式或极简环境。
- **`ntr`** — MTR 专用版。默认启动 MTR TUI。无 Globalping / WebUI，无常规 traceroute 模式，也不带独立 `--mtu` 模式。

### 手动编译

需要 Go 1.22+ 环境：

```bash
# 完整版（所有功能）
go build -trimpath -o dist/nexttrace -ldflags "-w -s" .

# 精简版（无 MTR、无 Globalping、无 WebUI）
go build -tags flavor_tiny -trimpath -o dist/nexttrace-tiny -ldflags "-w -s" .

# MTR 专用版
go build -tags flavor_ntr -trimpath -o dist/ntr -ldflags "-w -s" .
```

交叉编译示例：

```bash
# Linux arm64 精简版
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags flavor_tiny -trimpath -o dist/nexttrace-tiny_linux_arm64 -ldflags "-w -s" .
```

`tiny` 和 `ntr` 版本通过 **编译期 build tags** 裁剪模块——不是运行时开关。可通过 `go version -m <binary>` 验证 `nexttrace-tiny` 和 `ntr` 中不包含 `gin` 与 `globalping-cli`。

`.cross_compile.sh` 脚本支持按版本构建：

```bash
./.cross_compile.sh all     # 构建全部三个版本（所有平台）
./.cross_compile.sh full    # 仅构建 nexttrace（完整版）
./.cross_compile.sh tiny    # 仅构建 nexttrace-tiny
./.cross_compile.sh ntr     # 仅构建 ntr
```

### 发行资产命名规则

Release 二进制文件命名格式：

```text
{二进制名}_{操作系统}_{架构}[v{arm版本}][.exe][_softfloat]
```

示例：

- `nexttrace_linux_amd64`、`nexttrace-tiny_linux_amd64`、`ntr_linux_amd64`
- `nexttrace_darwin_universal`、`nexttrace-tiny_darwin_universal`、`ntr_darwin_universal`
- `nexttrace_windows_amd64.exe`、`ntr_windows_amd64.exe`

### Get Started

`NextTrace` 默认使用`ICMP`协议发起`TraceRoute`请求，该协议同时支持`IPv4`和`IPv6`

```bash
# IPv4 ICMP Trace
nexttrace 1.0.0.1
# URL
nexttrace http://example.com:8080/index.html?q=1

# 表格输出（报告模式）：运行一次探测后打印最终汇总表格
nexttrace --table 1.0.0.1

# 机器可读输出：stdout 只包含一个 JSON 文档
nexttrace --raw 1.0.0.1
nexttrace --json 1.0.0.1

# 将实时 traceroute 输出写入自定义文件
nexttrace --output ./trace.log 1.0.0.1

# 将实时 traceroute 输出写入默认日志文件
nexttrace --output-default 1.0.0.1

# 只进行IPv4/IPv6解析，且当多个IP时自动选择第一个IP
nexttrace --ipv4 g.co
nexttrace --ipv6 g.co

# IPv6 ICMP Trace
nexttrace 2606:4700:4700::1111

# 普通 traceroute 模式下设置 TTL 分组间隔（默认 300ms）
nexttrace -i 300 1.1.1.1

# 禁用路径可视化 使用 --map / -M 参数
nexttrace koreacentral.blob.core.windows.net
# MapTrace URL: https://api.nxtrace.org/tracemap/html/c14e439e-3250-5310-8965-42a1e3545266.html

# 禁用MPLS显示 使用 --disable-mpls / -e 参数 或 NEXTTRACE_DISABLEMPLS 环境变量
nexttrace --disable-mpls example.com
export NEXTTRACE_DISABLEMPLS=1
```

PS: 路由可视化的绘制模块为独立模块，具体代码可在 [nxtrace/traceMap](https://github.com/nxtrace/traceMap) 查看  
路由可视化功能因为需要每个 Hop 的地理位置坐标，而第三方 API 通常不提供此类信息，所以此功能目前只支持搭配 LeoMoeAPI 使用。

#### `Windows` 用户必须完成的配置步骤

- 对于普通用户模式：  
  只能使用 **ICMP mode**，且需防火墙配置允许`ICMP/ICMPv6`。
  ```powershell
  netsh advfirewall firewall add rule name="All ICMP v4" dir=in action=allow protocol=icmpv4:any,any
  netsh advfirewall firewall add rule name="All ICMP v6" dir=in action=allow protocol=icmpv6:any,any
  ```
- 对于管理员模式：  
  **TCP/UDP mode** 依赖 `WinDivert`。  
  **ICMP mode** 支持 `1=Socket` 与 `2=WinDivert`（`0=Auto`）。使用 Socket 模式时，需防火墙配置允许`ICMP/ICMPv6`。  
  在 `Windows` 上，`ICMPv6` 未传 `--tos` 或显式 `--tos 0` 时继续走原生 Socket 发送路径；只有非零 `ICMPv6 --tos` 才额外依赖 `WinDivert` 发送能力，并要求管理员权限。  
  `WinDivert` 可使用 `--init` 参数自动配置环境；该命令会将运行时解压到可执行文件目录。

#### `NextTrace` 现已经支持快速测试，有一次性测试回程路由需求的朋友可以使用

```bash
# 北上广（电信+联通+移动+教育网）IPv4 / IPv6 ICMP 快速测试
nexttrace --fast-trace

# 也可以使用 TCP SYN 而非 ICMP 进行测试
nexttrace --fast-trace --tcp

# 也可以通过自定义的IP/DOMAIN列表文件进行快速测试
nexttrace --file /path/to/your/iplist.txt
# 自定义的IP/DOMAIN列表文件格式
## 一行一个IP/DOMAIN + 空格 + 描述信息（可选）
## 例如：
## 106.37.67.1 北京电信
## 240e:928:101:31a::1 北京电信
## bj.10086.cn 北京移动
## 2409:8080:0:1::1
## 223.5.5.5
```

#### `NextTrace` 已支持指定网卡进行路由跟踪

在 macOS 和 Linux 上，`--dev` 会绑定到指定源网卡。
在 Windows 上，`--dev` 只用于选择 source address，不保证真实出接口。

```bash
# 请注意 Lite 版本此参数不能和快速测试联用，如有需要请使用 enhanced 版本
# 使用 eth0 网卡
nexttrace --dev eth0 2606:4700:4700::1111

# 使用 eth0 网卡IP
# 网卡 IP 可以使用 ip a 或者 ifconfig 获取
# 使用网卡IP进行路由跟踪时需要注意跟踪的IP类型应该和网卡IP类型一致（如都为 IPv4）
nexttrace --source 204.98.134.56 9.9.9.9
```

#### `NextTrace` 也可以使用`TCP`和`UDP`协议发起`Traceroute`请求

```bash
# TCP SYN Trace
nexttrace --tcp www.bing.com

# 可以自行指定目标端口[此处为443]，默认80端口
nexttrace --tcp --port 443 2001:4860:4860::8888

# UDP Trace
nexttrace --udp 1.0.0.1

# 可以自行指定目标端口[此处为5353]，默认33494端口
nexttrace --udp --port 5353 1.0.0.1

# TCP/UDP Trace 可以自行指定源端口，默认使用随机一个固定的端口(如需每次发包随机使用不同的源端口，请设置`ENV` `NEXTTRACE_RANDOMPORT`)
nexttrace --tcp --source-port 14514 www.bing.com
```

#### `NextTrace` 也支持独立的路径 MTU 探测模式

```bash
# 类 tracepath 的 UDP PMTU 探测，运行中实时刷行
nexttrace --mtu 1.1.1.1

# mtu 模式同样复用常规的 GeoIP / RDNS 参数
nexttrace --mtu --data-provider IPInfo --language en 1.1.1.1

# JSON 输出沿用独立 mtu schema，并包含 hop.geo
nexttrace --mtu --json 1.1.1.1
```

- `--mtu` 是独立的 UDP-only 模式，不复用普通 traceroute 引擎。
- TTY 下会原地更新当前 hop，并为 hop 状态 / PMTU 高亮加色；重定向/管道输出会退化成“定稿一跳输出一行”的无 ANSI 流式文本。
- `--mtu --json` 在 stdout 上只输出独立的 MTU JSON 文档。
- GeoIP、RDNS、`--data-provider`、`--language`、`--no-rdns`、`--always-rdns`、`--dot-server` 都对该模式生效。

#### `NextTrace`也同样支持一些进阶功能，如 TTL 控制、并发数控制、模式切换等

```bash
# 每一跳发送2个探测包
nexttrace --queries 2 www.hkix.net

# 无并发，每次只发送一个探测包
nexttrace --parallel-requests 1 www.hkix.net

# 从TTL为5开始发送探测包，直到TTL为10结束
nexttrace --first 5 --max-hops 10 www.decix.net
# 此外还提供了一个ENV，可以设置是否隐匿目的IP
export NEXTTRACE_ENABLEHIDDENDSTIP=1

# 关闭IP反向解析功能
nexttrace --no-rdns www.bbix.net

# 设置探测包总大小为1024字节（含 IP + 探测协议头）
nexttrace --psize 1024 example.com

# 让每个 probe 在 1500 字节内随机大小
nexttrace --psize -1500 example.com

# 设置 TOS / traffic class 字段
nexttrace -Q 46 example.com

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
# 禁止色彩输出
nexttrace --no-color 1.1.1.1
# 或者使用环境变量
export NO_COLOR=1
```

#### 高级参数调优速查

| 参数 | 控制内容 | 默认值 / 起步建议 | 什么时候调 |
| --- | --- | --- | --- |
| `--queries` | 常规 traceroute 的每跳采样数；MTR 中显式指定每跳探测次数 | traceroute: `3`；MTR report: 未指定时 `10`；MTR TUI/raw: 未指定时无限 | 链路抖动大时可升到 `5-10` |
| `--max-attempts` | 每跳最大发包上限 | 默认按 `--queries` 自动推导 | 丢包严重、回包慢时增大 |
| `--parallel-requests` | 跨 TTL 的总并发 in-flight 探测数 | `18` | 多路径/负载均衡链路用 `1`；稳定链路一般 `6-18` |
| `--send-time` | 同一 TTL 组内相邻探测包间隔 | `50ms` | 设备限速时升到 `100-200ms`；MTR 下忽略 |
| `--ttl-time` | 常规 traceroute 的 TTL 组间隔；MTR 的每跳探测间隔 | traceroute: `300ms`；MTR: 未指定时 `1000ms` | 想加速就调低；远程/限速链路调高 |
| `--timeout` | 单个探测包超时 | `1000ms` | 跨洲或高丢包链路升到 `2000-3000ms` |
| `--psize` | 探测包大小 | 按协议/IP 族自动取最小合法值 | 含 IP + 探测协议头；负值表示每个 probe 在 `abs(value)` 内随机；超过出接口/路径 MTU 时，链路上可能看到分片 |
| `-Q`, `--tos` | IP TOS / traffic class | `0` | 设置 IP 头里的 TOS / traffic class；在 Windows 上仅 `ICMPv6` 且值非零时额外依赖 `WinDivert` |

这些探测参数目前仍是 CLI 级配置，`nt_config.yaml` 还不能直接保存它们。若要复用一组调优参数，建议写成 shell alias 或小脚本。

```bash
# 适合多路径 / ECMP 的保守配置
nexttrace --parallel-requests 1 --send-time 100 --ttl-time 500 --timeout 2000 example.com

# 适合稳定单路径链路的快速配置
nexttrace --parallel-requests 18 --send-time 20 --ttl-time 150 example.com

# 适合高丢包长途链路的配置
nexttrace --queries 5 --max-attempts 10 --timeout 2500 example.com
```

#### `NextTrace` 支持 MTR（My Traceroute）连续探测模式

```bash
# MTR 模式：使用 ICMP（默认）连续探测，实时刷新表格
nexttrace -t 1.1.1.1
# 等价写法：
nexttrace --mtr 1.1.1.1

# MTR 模式使用 TCP SYN 探测
nexttrace -t --tcp --port 443 www.bing.com

# MTR 模式使用 UDP 探测
nexttrace -t --udp 1.0.0.1

# 设置每个跳点的探测间隔（MTR 模式下默认 1000ms；-z/--send-time 在 MTR 模式下无效）
nexttrace -t -i 500 1.1.1.1

# 限制每个跳点的最大探测次数（TUI 默认无限，报告模式默认 10）
nexttrace -t -q 20 1.1.1.1

# 报告模式：对每个跳点探测 N 次后一次性输出统计摘要（类似 mtr -r）
nexttrace -r 1.1.1.1       # = --mtr --report，默认每跳点 10 次
nexttrace -r -q 5 1.1.1.1  # 每跳点 5 次

# 宽报告模式：主机列不截断（类似 mtr -rw）
nexttrace -w 1.1.1.1       # = --mtr --report --wide

# 在 MTR 输出中同时显示 PTR 和 IP（PTR 在前，IP 括号）
nexttrace --mtr --show-ips 1.1.1.1
nexttrace -r --show-ips 1.1.1.1
nexttrace -w --show-ips 1.1.1.1

# MTR 原始流式模式（面向程序解析，逐事件输出）
nexttrace --mtr --raw 1.1.1.1
nexttrace -r --raw 1.1.1.1

# 与其他选项组合使用
nexttrace -t --tcp --max-hops 20 --first 3 --no-rdns 8.8.8.8
```

在终端（TTY）中运行时，MTR 模式使用**交互式全屏 TUI**：

- **`q` / `Q`** — 退出（恢复终端，不留下输出）
- **`p`** — 暂停探测
- **空格** — 恢复探测
- **`r`** — 重置统计（计数器清零，显示模式保持不变）
- **`y`** — 循环切换主机显示模式：ASN → City → Owner → Full
- **`n`** — 切换主机名显示方式：
  - 默认：PTR（无 PTR 时回退 IP）↔ 仅 IP
  - 启用 `--show-ips`：PTR (IP) ↔ 仅 IP
- **`e`** — 切换 MPLS 标签显示开/关
- TUI 标题栏显示**源 → 目标**路由信息，指定 `--source`/`--dev` 时会展示对应信息。
- 使用 LeoMoeAPI 时，标题栏会显示首选 API IP 地址。
- 使用**备用屏幕缓冲区**，退出后恢复之前的终端历史记录。
- 当 stdin 非 TTY（如管道输入）时，降级为简单表格刷新模式。

**报告模式**（`-r`/`--report`）在所有探测完成后一次性输出统计，适合脚本使用：

```text
Start: 2025-07-14T09:12:00+08:00
HOST: myhost                    Loss%   Snt   Last    Avg   Best   Wrst  StDev
  1. one.one.one.one            0.0%    10    1.23   1.45   0.98   2.10   0.32
  2. 10.0.0.2                 100.0%    10    0.00   0.00   0.00   0.00   0.00
```

显示为 `(waiting for reply)` 的行仍然保留同样的表格列布局，只是该行的指标单元格会留空。

非 wide 报告模式会刻意保持 Host 列精简：

- 只显示 `PTR/IP`
- 不发起 Geo API 查询
- 不显示 ASN / 运营商 / 地理位置字段
- 不显示 MPLS 标签

wide 报告模式（`-w` / `--wide`）继续保留当前完整信息行为，包括 Geo 衍生字段和 MPLS 输出。

当 `--raw` 与 MTR（`--mtr`、`-r`、`-w`）一起使用时，会进入 **MTR 原始流式模式**。

如果当前数据源是 `LeoMoeAPI`，会先输出一行无色的 API 信息头：

```text
[NextTrace API] preferred API IP - [2403:18c0:1001:462:dd:38ff:fe48:e0c5] - 21.33ms - DMIT.NRT
```

之后再逐行输出 `|` 分隔的事件流：

```
4|84.17.33.106|po66-3518.cr01.nrt04.jp.misaka.io|0.27|60068|日本|东京都|东京||cdn77.com|35.6804|139.7690
```

字段顺序：

`ttl|ip|ptr|rtt|asn|一级行政区|二级行政区|三级行政区|四级行政区|owner|纬度|经度`

超时行保持固定 12 列：

`ttl|*||||||||||`

在 MTR 模式（`--mtr`、`-r`、`-w`，包括 `--raw`）下，`-i/--ttl-time` 设置的是**每个跳点的探测间隔**：同一跳点两次连续探测之间的等待时间（未显式指定时默认 1000ms）。`-z/--send-time` 在 MTR 模式下被忽略。

> 注意：`--show-ips` 仅在 MTR 模式（`--mtr`、`-r`、`-w`）生效，其他模式会忽略。
>
> 注意：`--mtr` 不可与 `--table`、`--classic`、`--json`、`--output`、`--output-default`、`--route-path`、`--from`、`--fast-trace`、`--file`、`--deploy` 同时使用。

#### `NextTrace`支持用户自主选择 IP 数据库（目前支持：`LeoMoeAPI`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`, `IPInfoLocal`, `CHUNZHEN`)

```bash
# 可以自行指定IP数据库[此处为IP-API.com]，不指定则默认为LeoMoeAPI
nexttrace --data-provider ip-api.com
## 特别的: 其中 ipinfo 和 IPInsight API 对于免费版查询有频率限制，可从这些服务商自行购买服务以解除限制，如有需要可以 clone 本项目添加其提供的 token 自行编译
##        TOKEN填写路径：ipgeo/tokens.go

## 特别的: 对于离线库 IPInfoLocal，请自行下载并命名为 ipinfoLocal.mmdb
##        (可以从这里下载：https://ipinfo.io/signup?ref=free-database-downloads)，
##        默认搜索用户当前路径、程序所在路径、和 FHS 路径（Unix-like）
##        如果需要自定义路径，请设置环境变量
export NEXTTRACE_IPINFOLOCALPATH=/xxx/yyy.mmdb
## 另外：由于IP.SB被滥用比较严重，会经常出现无法查询的问题，请知悉。
##      IP-API.com限制调用较为严格，如有查询不到的情况，请几分钟后再试。
# 纯真IP数据库默认使用 http://127.0.0.1:2060 作为查询接口，如需自定义请使用环境变量
export NEXTTRACE_CHUNZHENURL=http://127.0.0.1:2060
## 可使用 https://github.com/freshcn/qqwry 自行搭建纯真IP数据库服务

# 也可以通过设置环境变量来指定默认IP数据库
export NEXTTRACE_DATAPROVIDER=ipinfo
```

#### `NextTrace`支持使用混合参数和简略参数

```bash
Example:
nexttrace --data-provider ip-api.com --max-hops 20 --tcp --port 443 --queries 5 --no-rdns 1.1.1.1
nexttrace -tcp --queries 2 --parallel-requests 1 --table --route-path 2001:4860:4860::8888

Equivalent to:
nexttrace -d ip-api.com -m 20 -T -p 443 -q 5 -n 1.1.1.1
nexttrace -T -q 2 --parallel-requests 1 --table -P 2001:4860:4860::8888
```

### Globalping

[Globalping](https://globalping.io/) 提供了对成千上万由社区托管的探针的访问能力，可用于运行网络测试和测量。

通过 `--from` 参数可以选择使用指定位置的探针来执行 traceroute。位置字段支持洲、国家、地区、城市、ASN、ISP 或云厂商区域等多种类型。

```bash
nexttrace google.com --from Germany
nexttrace google.com --from comcast+california
```

匿名用户默认每小时限额为 250 次测试。将 `GLOBALPING_TOKEN` 环境变量设置为你的令牌后，可将限额提升至每小时 500 次。

```bash
export GLOBALPING_TOKEN=your_token_here
```

### 环境变量总览

NextTrace 当前会读取下列环境变量。对于布尔开关，只识别 `1` 和 `0`，其他值会回退到内置默认值。为了避免混淆，修改后建议重启 NextTrace。

#### 核心运行 / 网络

| 变量名 | 默认值 | 说明 |
| --- | --- | --- |
| `NEXTTRACE_DEVMODE` | `0` | 开发调试模式：致命错误改为 panic，并打印堆栈。 |
| `NEXTTRACE_DEBUG` | 未设置 | 在 `GetEnv*` 解析环境变量时打印检测到的值。 |
| `NEXTTRACE_DISABLEMPLS` | `0` | 全局禁用 MPLS 显示，效果类似 `--disable-mpls`。 |
| `NEXTTRACE_ENABLEHIDDENDSTIP` | `0` | 隐匿目的 IP，并省略其主机名显示。 |
| `NEXTTRACE_RANDOMPORT` | `0` | TCP/UDP 每个探测包使用不同的随机源端口。 |
| `NEXTTRACE_MAXATTEMPTS` | 自动计算 | 当未显式传入 `--max-attempts` 时，提供默认最大重试次数。 |
| `NEXTTRACE_ICMPMODE` | `0` | 当未显式传入 `--icmp-mode` 时提供默认值（`0=自动`、`1=Socket`、`2=WinDivert`）。 |
| `NEXTTRACE_UNINTERRUPTED` | `0` | 与 `--raw` 一起使用时，会在一次探测结束后继续循环执行，而不是退出。 |
| `NEXTTRACE_PROXY` | 未设置 | 为 PoW、Geo API、tracemap 等出站 HTTP / WebSocket 请求设置代理 URL。 |
| `NEXTTRACE_DATAPROVIDER` | 未设置 | 覆盖默认 IP 地理信息源，例如 `ipinfo`。 |

#### 服务 / Web / 后端

| 变量名 | 默认值 | 说明 |
| --- | --- | --- |
| `NEXTTRACE_HOSTPORT` | `api.nxtrace.org` | 覆盖 LeoMoeAPI、tracemap、FastIP 等使用的后端地址，支持 `host` 或 `host:port`。 |
| `NEXTTRACE_TOKEN` | 未设置 | 预置 LeoMoeAPI Bearer Token；设置后将跳过 PoW 取 token 流程。 |
| `NEXTTRACE_POWPROVIDER` | `api.nxtrace.org` | 指定 PoW 服务提供方；当前内置的非默认别名为 `sakura`。 |
| `NEXTTRACE_DEPLOY_ADDR` | 未设置 | `--deploy` 模式下，当未传 `--listen` 时使用的默认监听地址。 |
| `NEXTTRACE_ALLOW_CROSS_ORIGIN` | `0` | 仅对 `--deploy` 生效：是否允许跨站浏览器访问 Web UI / API。默认关闭以保证安全。 |

#### IP 数据库 / 第三方服务

| 变量名 | 默认值 | 说明 |
| --- | --- | --- |
| `NEXTTRACE_IPINFOLOCALPATH` | 自动搜索 | `IPInfoLocal` 离线库 `ipinfoLocal.mmdb` 的完整路径。 |
| `NEXTTRACE_CHUNZHENURL` | `http://127.0.0.1:2060` | 纯真 IP 查询服务的基础 URL。 |
| `NEXTTRACE_IPINFO_TOKEN` | 未设置 | `IPInfo` 数据源使用的 token。 |
| `NEXTTRACE_IPINSIGHT_TOKEN` | 未设置 | `IPInsight` 数据源使用的 token。 |
| `NEXTTRACE_IPAPI_BASE` | 各 provider 内置地址 | 覆盖当前实现里兼容 HTTP 接口的数据源基础地址（`IPInfo`、`IPInsight`、`ip-api.com`）。 |
| `IPDBONE_BASE_URL` | `https://api.ipdb.one` | 覆盖 IPDB.One API 基础地址。 |
| `IPDBONE_API_ID` | 未设置 | IPDB.One API ID。 |
| `IPDBONE_API_KEY` | 未设置 | IPDB.One API Key。 |
| `GLOBALPING_TOKEN` | 未设置 | Globalping 鉴权 token；设置后可提升匿名用户的每小时测试额度。 |

#### 配置文件搜索

| 变量名 | 默认值 | 说明 |
| --- | --- | --- |
| `XDG_CONFIG_HOME` | 取决于系统 / Shell | 如果设置了该变量，NextTrace 也会从 `$XDG_CONFIG_HOME/nexttrace` 搜索 `nt_config.yaml`。 |

### 全部用法详见 Usage 菜单

```shell
Usage: nexttrace [-h|--help] [--init] [-4|--ipv4] [-6|--ipv6] [-T|--tcp]
                 [-U|--udp] [-F|--fast-trace] [-p|--port <integer>]
                 [--icmp-mode <integer>] [-q|--queries <integer>]
                 [--max-attempts <integer>] [--parallel-requests <integer>]
                 [-m|--max-hops <integer>] [-d|--data-provider
                 (IP.SB|ip.sb|IPInfo|ipinfo|IPInsight|ipinsight|IPAPI.com|ip-api.com|IPInfoLocal|ipinfolocal|chunzhen|LeoMoeAPI|leomoeapi|ipdb.one|disable-geoip)]
                 [--pow-provider (api.nxtrace.org|sakura)] [-n|--no-rdns]
                 [-a|--always-rdns] [-P|--route-path] [--dn42] [-o|--output
                 "<value>"] [-O|--output-default] [--table] [--raw]
                 [-j|--json] [-c|--classic] [-f|--first <integer>] [-M|--map]
                 [-e|--disable-mpls] [-V|--version]
                 [-s|--source "<value>"] [--source-port <integer>] [-D|--dev
                 "<value>"] [--listen "<value>"] [--deploy] [-z|--send-time
                 <integer>] [-i|--ttl-time <integer>] [--timeout <integer>]
                 [--psize <integer>] [--dot-server
                 (dnssb|aliyun|dnspod|google|cloudflare)] [-g|--language
                 (en|cn)] [-C|--no-color] [--from "<value>"] [-t|--mtr]
                 [-r|--report] [-w|--wide] [--show-ips] [-y|--ipinfo <integer>]
                 [--file "<value>"] [TARGET "<value>"]

Arguments:

  -h  --help                         Print help information
      --init                         Windows ONLY: Extract WinDivert runtime to
                                     executable directory
  -4  --ipv4                         Use IPv4 only
  -6  --ipv6                         Use IPv6 only
  -T  --tcp                          Use TCP SYN for tracerouting (default
                                     dest-port is 80)
  -U  --udp                          Use UDP SYN for tracerouting (default
                                     dest-port is 33494)
  -F  --fast-trace                   One-Key Fast Trace to China ISPs
  -p  --port                         Set the destination port to use. With
                                     default of 80 for "tcp", 33494 for "udp"
      --icmp-mode                    Windows ONLY: Choose the method to listen
                                     for ICMP packets (1=Socket, 2=WinDivert;
                                     0=Auto)
  -q  --queries                      Latency samples per hop. Increase to 5-10
                                     on unstable paths for a steadier view.
                                     Default: 3
      --max-attempts                 Advanced: hard cap on probe packets per
                                     hop. Leave unset for auto sizing; raise on
                                     lossy links if --queries is not enough
      --parallel-requests            Advanced: total concurrent in-flight
                                     probes across TTLs. Use 1 on
                                     multipath/load-balanced paths; 6-18 is a
                                     good starting range on stable links.
                                     Default: 18
  -m  --max-hops                     Set the max number of hops (max TTL to be
                                     reached). Default: 30
  -d  --data-provider                Choose IP Geograph Data Provider [IP.SB,
                                     IPInfo, IPInsight, IP-API.com,
                                     IPInfoLocal, CHUNZHEN, disable-geoip].
                                     Default: LeoMoeAPI
      --pow-provider                 Choose PoW Provider [api.nxtrace.org,
                                     sakura] For China mainland users, please
                                     use sakura. Default: api.nxtrace.org
  -n  --no-rdns                      Do not resolve IP addresses to their
                                     domain names
  -a  --always-rdns                  Always resolve IP addresses to their
                                     domain names
  -P  --route-path                   Print traceroute hop path by ASN and
                                     location
      --dn42                         DN42 Mode
  -o  --output                       Write trace result to FILE
                                     (RealtimePrinter only)
  -O  --output-default               Write trace result to the default log file
                                     (/tmp/trace.log)
      --table                        Output trace results as a final summary
                                     table (traceroute report mode)
      --raw                          Machine-friendly output. With MTR
                                     (--mtr/-r/-w), enables streaming raw event
                                     mode
  -j  --json                         Output trace results as JSON
  -c  --classic                      Classic Output trace results like
                                     BestTrace
  -f  --first                        Start from the first_ttl hop (instead of
                                     1). Default: 1
  -M  --map                          Disable Print Trace Map
  -e  --disable-mpls                 Disable MPLS
  -V  --version                      Print version info and exit
  -s  --source                       Use source address src_addr for outgoing
                                     packets
      --source-port                  Use source port src_port for outgoing
                                     packets
  -D  --dev                          Use the specified network device for
                                     explicit source selection. On Windows,
                                     this only chooses the source address and
                                     does not guarantee the egress interface
      --listen                       Set listen address for web console (e.g.
                                     127.0.0.1:30080)
      --deploy                       Start the Gin powered web console
  -z  --send-time                    Advanced: per-packet gap [ms] inside the
                                     same TTL group. Lower is faster; raise to
                                     100-200ms on rate-limited links. Ignored
                                     in MTR mode. Default: 50
  -i  --ttl-time                     Advanced: TTL-group interval [ms] in
                                     normal traceroute. In MTR mode
                                     (--mtr/-r/-w, including --raw), this
                                     becomes per-hop probe interval. 500-1000ms
                                     is a good MTR starting range
      --timeout                      Per-probe timeout [ms]. Raise to 2000-3000
                                     on slow intercontinental or high-loss
                                     paths. Default: 1000
      --psize                        Probe packet size in bytes, inclusive IP
                                     and active probe headers. Default is the
                                     minimum legal size for the chosen
                                     protocol and IP family; raise for MTU or
                                     large-packet testing. Negative values
                                     randomize each probe up to abs(value).
  -Q  --tos                          Set the IP type-of-service / traffic class
                                     value [0-255]. Default: 0
      --dot-server                   Use DoT Server for DNS Parse [dnssb,
                                     aliyun, dnspod, google, cloudflare]
  -g  --language                     Choose the language for displaying [en,
                                     cn]. Default: cn
  -C  --no-color                     Disable Colorful Output
      --from                         Run traceroute via Globalping
                                     (https://globalping.io/network) from a
                                     specified location. The location field
                                     accepts continents, countries, regions,
                                     cities, ASNs, ISPs, or cloud regions.
  -t  --mtr                          Enable MTR (My Traceroute) continuous
                                     probing mode
  -r  --report                       MTR report mode (non-interactive, implies
                                     --mtr); can trigger MTR without --mtr
  -w  --wide                         MTR wide report mode (implies --mtr
                                     --report); alone equals --mtr --report
                                     --wide
      --show-ips                     MTR only: display both PTR hostnames and
                                     numeric IPs (PTR first, IP in parentheses)
  -y  --ipinfo                       Set initial MTR TUI host info mode (0-4).
                                     TUI only; ignored in --report/--raw.
                                     0:IP/PTR 1:ASN 2:City 3:Owner 4:Full.
                                     Default: 0
      --file                         Read IP Address or domain name from file
      TARGET                         Trace target: IPv4 address (e.g. 8.8.8.8),
                                     IPv6 address (e.g. 2001:db8::1), domain
                                     name (e.g. example.com), or URL (e.g.
                                     https://example.com)
```

## 项目截图

![image](https://user-images.githubusercontent.com/59512455/218505939-287727ce-7207-43c4-8e31-fcda7df0b872.png)

![image](https://user-images.githubusercontent.com/59512455/218504874-06b9fa4b-48e0-420a-a195-08a1200d65a7.png)

## 第三方 IP 数据库 API 开发接口

NextTrace 所有的的 IP 地理位置 `API DEMO` 可以参考[这里](https://github.com/nxtrace/NTrace-core/blob/main/ipgeo/)

你可以在这里添加你自己的 API 接口，为了 NextTrace 能够正确显示你接口中的内容，请参考 `leo.go` 中所需要的信息

✨NextTrace `LeoMoeAPI` 的后端 Demo

[GitHub - sjlleo/nexttrace-backend: NextTrace BackEnd](https://github.com/sjlleo/nexttrace-backend)

NextTrace `LeoMoeAPI`现已使用Proof of Work(POW)机制来防止滥用，其中NextTrace作为客户端引入了powclient库，POW CLIENT/SERVER均已开源，欢迎大家使用。(POW模块相关问题请发到对应的仓库)

- [GitHub - tsosunchia/powclient: Proof of Work CLIENT for NextTrace](https://github.com/tsosunchia/powclient)
- [GitHub - tsosunchia/powserver: Proof of Work SERVER for NextTrace](https://github.com/tsosunchia/powserver)

对于中国大陆用户，可以使用 [Nya Labs](https://natfrp.com) 提供的位于大陆的POW服务器优化访问速度

```shell
#使用方法任选其一
#1. 在环境变量中设置
export NEXTTRACE_POWPROVIDER=sakura
#2. 在命令行中设置
nexttrace --pow-provider sakura
```

## OpenTrace

`OpenTrace`是 @Archeb 开发的`NextTrace`的跨平台`GUI`版本，带来您熟悉但更强大的用户体验。  
该软件仍然处于早期开发阶段，可能存在许多缺陷和错误，需要您宝贵的使用反馈。

[https://github.com/Archeb/opentrace](https://github.com/Archeb/opentrace)

## NEXTTRACE WEB API

`NextTraceWebApi`是一个`MTR`风格的`NextTrace`网页版服务端实现，提供了包括`Docker`在内多种部署方式。

在 WebSocket 持续探测模式中，MTR 现改为逐事件推送 `type: "mtr_raw"`（不再使用周期性 `mtr` 快照消息）。

[https://github.com/nxtrace/nexttracewebapi](https://github.com/nxtrace/nexttracewebapi)

## NextTraceroute

`NextTraceroute`，一款默认使用`NextTrace API`的免`root`安卓版路由跟踪应用，由 @surfaceocean 开发。  
感谢所有测试用户的热情支持，本应用已经通过封闭测试，正式进入 Google Play 商店。

[https://github.com/nxtrace/NextTraceroute](https://github.com/nxtrace/NextTraceroute)  
<a href='https://play.google.com/store/apps/details?id=com.surfaceocean.nexttraceroute&pcampaignid=pcampaignidMKT-Other-global-all-co-prtnr-py-PartBadge-Mar2515-1'><img alt='Get it on Google Play' width="128" height="48" src='https://play.google.com/intl/en_us/badges/static/images/badges/en_badge_web_generic.png'/></a>

## Cloudflare Support

本项目受 [Alexandria 计划](http://www.cloudflare.com/oss-credits)赞助。

<img src="https://cf-assets.www.cloudflare.com/slt3lc6tev37/2I3y49Uz9Y61lBS0kIPZu6/db6df1e6f99a8659267c442b75a0dff9/image.png" alt="Cloudflare Logo" width="331">

## AIWEN TECH Support

本项目受 [埃文科技](https://www.ipplus360.com) 赞助。 很高兴使用`埃文科技城市级IP库`增强本项目 GEOIP 查询的准确性与完整性，并免费提供给公众。

<img src="https://www.ipplus360.com/img/LOGO.c86cd0e1.svg" title="" alt="埃文科技 IP 定位数据" width="331">

## JetBrain Support

本项目受 [JetBrain Open-Source Project License](https://jb.gg/OpenSourceSupport) 支持。 很高兴使用`Goland`作为我们的开发工具。

<img src="https://resources.jetbrains.com/storage/products/company/brand/logos/GoLand.png" title="" alt="GoLand logo" width="331">

## Credits

[Gubo](https://www.gubo.org) 靠谱主机推荐

[IPInfo](https://ipinfo.io) 无偿提供了本项目大部分数据支持

[BGP.TOOLS](https://bgp.tools) 无偿提供了本项目的一些数据支持

[PeeringDB](https://www.peeringdb.com) 无偿提供了本项目的一些数据支持

[Globalping](https://globalping.io) 一个开源且免费的项目，提供全球范围内运行 traceroute 等网络测试的访问服务

[sjlleo](https://github.com/sjlleo) 项目永远的领导者、创始人及核心贡献者

[tsosunchia](https://github.com/tsosunchia) 项目现任管理、基础设施运维及核心贡献者

[Yunlq](https://github.com/Yunlq) 活跃的社区贡献者

[Vincent Young](https://github.com/missuo)

[zhshch2002](https://github.com/zhshch2002)

[Sam Sam](https://github.com/samleong123)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p)

[bobo liu](https://github.com/fakeboboliu)

[YekongTAT](https://github.com/isyekong)

## Others

- 其他第三方 API 尽管集成在本项目内，但是具体的 TOS 以及 AUP，请详见第三方 API 官网。如遇到 IP 数据错误，也请直接联系他们纠错。

- 如何获取最新commit的新鲜出炉的二进制可执行文件？

  > 请前往GitHub Actions中最新一次 [Build & Release](https://github.com/nxtrace/NTrace-dev/actions/workflows/build.yml) workflow.

- 常见疑问
  - Windows 平台下，ICMP 模式须手动放行ICMP/ICMPv6防火墙
  - macOS 平台下，仅 ICMP 模式不需要提权运行
  - 在一些情况下，同时运行多个 NextTrace 实例可能会导致互相干扰结果(目前仅在 TCP 模式下有观察到)

## IP 数据以及精准度说明

对于IP相关信息的纠错反馈，我们目前开放了两个渠道：

> - 本项目的GITHUB ISSUES区中的[IP 错误报告汇总帖](https://github.com/orgs/nxtrace/discussions/222)
> - 本项目的纠错专用邮箱: `correct#nxtrace.org` （请注意此邮箱仅供IP相关信息纠错专用，其他反馈请发送ISSUE）

NextTrace 有多个数据源可以选择，目前默认使用的 LeoMoeAPI 为我们项目维护的数据源。

该项目由 OwO Network 的 [Missuo](https://github.com/missuo) && [Leo](https://github.com/sjlleo) 发起，由 [Zhshch](https://github.com/zhshch2002/) 完成最早期架构的编写和指导，后由 Leo 完成了大部分开发工作，现主要交由 [tsosunchia](https://github.com/tsosunchia) 完成后续的二开和维护工作。

LeoMoeAPI 是 [Leo](https://github.com/sjlleo) 的作品，归属于 Leo Network，由 [Leo](https://github.com/sjlleo) 完成整套后端 API 编写，该接口未经允许不可用于任何第三方用途。

LeoMoeAPI 早期数据主要来自 IPInsight、IPInfo，随着项目发展，越来越多的志愿者参与进了这个项目。目前 LeoMoeAPI 有近一半的数据是社区提供的，而另外一半主要来自于包含 IPInfo、IPData、BigDataCloud、IPGeoLocation 在内的多个第三方数据。

LeoMoeAPI 的骨干网数据有近 70% 是社区自发反馈又或者是项目组成员校准的，这给本项目的路由跟踪基础功能带来了一定的保证，但是全球骨干网的体量庞大，我们并无能力如 IPIP 等商业公司拥有海量监测节点，这使得 LeoMoeAPI 的数据精准度无法和形如 BestTrace（IPIP）相提并论。

LeoMoeAPI 已经尽力校准了比较常见的骨干网路由，这部分在测试的时候经常会命中，但是如果遇到封闭型 ISP 的路由，大概率可以遇到错误，此类数据不仅是我们，哪怕 IPInsight、IPInfo 也无法正确定位，目前只有 IPIP 能够标记正确，如对此类数据的精确性有着非常高的要求，请务必使用 BestTrace 作为首选。

我们不保证我们的数据一定会及时更新，也不保证数据的精确性，我们希望您在发现数据错误的时候可以前往 issue 页面提交错误报告，谢谢。

当您使用 LeoMoeAPI 即视为您已经完全了解 NextTrace LeoMoeAPI 的数据精确性，并且同意如果您引用 LeoMoeAPI 其中的数据从而引发的一切问题，均由您自己承担。

## DN42 模式使用说明

使用这个模式需要您配置 2 个文件，分别是 geofeed.csv 以及 ptr.csv

当您初次运行 DN42 模式，NT 会为您生成 nt_config.yaml 文件，您可以自定义 2 个文件的存放位置，默认应该存放在 NT 的运行目录下

### GeoFeed

对于 geofeed.csv 来说，格式如下：

```
IP_CDIR,LtdCode,ISO3166-2,CityName,ASN,IPWhois
```

比如，您可以这么写：

```
58.215.96.0/20,CN,CN-JS,Wuxi,23650,CHINANET-JS
```

如果您有一个大段作为骨干网使用，您也可以不写地理位置信息，如下：

```
202.97.0.0/16,,,4134,CHINANET-BACKBONE
```

### PTR

对于 ptr.csv 来说，格式如下：

```
IATA_CODE,LtdCode,RegionName,CityName
```

比如对于美国洛杉矶，您可以这么写

```
LAX,US,California,Los Anegles
```

需要注意的是，NextTrace 支持自动匹配 CSV 中的城市名，如果您的 PTR 记录中有 `losangeles`，您可以只添加上面一条记录就可以正常识别并读取。

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=nxtrace/NTrace-core&type=Date)](https://star-history.com/#nxtrace/NTrace-core&Date)
