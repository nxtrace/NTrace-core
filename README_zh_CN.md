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
  <a href="https://github.com/nxtrace/NTrace-dev/actions/workflows/golangci-lint.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/nxtrace/NTrace-dev/golangci-lint.yml?branch=main&style=flat-square&label=golangci-lint" alt="golangci-lint">
  </a>
  <a href="https://github.com/nxtrace/NTrace-dev/releases">
    <img src="https://img.shields.io/github/release/nxtrace/NTrace-dev/all.svg?style=flat-square">
  </a>
</p>

## 默认模式迁移公告：最早于 2027 年

> **下游开发者请注意：** NextTrace 将最早于 2027 年将 `nexttrace` 和 `nexttrace-tiny` 的默认运行与显示模式切换为 MTR，单独使用 `--raw` 时也将切换为 MTR RAW。传统 traceroute 及其 RAW 输出将继续通过 `-k/--traceroute` 提供。依赖当前行为的程序应提前使用 `--traceroute` 或 `--traceroute --raw`。**本次版本尚未切换默认模式，具体切换版本将另行公告。** `ntr` 继续作为 MTR 专用版。

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

- Debian / Ubuntu
  - 推荐：通过官方 `nexttrace-debs` APT 源安装
    - 当前支持：`amd64`、`i386`、`arm64`、`armel`、`armhf`、`loong64`、`mipsel`、`mips64el`、`ppc64el`、`riscv64`、`s390x`
    - 添加源并安装默认包：
      ```shell
      sudo install -d -m 0755 /etc/apt/keyrings
      curl -fsSL -o /tmp/nexttrace-archive-keyring.gpg https://github.com/nxtrace/nexttrace-debs/releases/latest/download/nexttrace-archive-keyring.gpg
      sudo install -m 0644 /tmp/nexttrace-archive-keyring.gpg /etc/apt/keyrings/nexttrace.gpg
      rm -f /tmp/nexttrace-archive-keyring.gpg
      printf '%s\n' 'Types: deb' 'URIs: https://github.com/nxtrace/nexttrace-debs/releases/latest/download/' 'Suites: ./' 'Signed-By: /etc/apt/keyrings/nexttrace.gpg' | sudo tee /etc/apt/sources.list.d/nexttrace.sources >/dev/null
      sudo apt update
      sudo apt install nexttrace
      ```
    - 可选：安装额外的 flavor：
      ```shell
      sudo apt install nexttrace-tiny
      sudo apt install ntr
      ```
    - 三个包可以共存安装，对应命令分别是：`nexttrace`、`nexttrace-tiny`、`ntr`

- Linux / macOS / BSD
  - 一键安装脚本（完整版，默认）
    ```shell
    curl -sL https://nxtrace.org/nt | bash
    ```
  - 一键安装脚本（Tiny）
    ```shell
    curl -sL https://nxtrace.org/nt | bash -s -- --flavor tiny
    ```
  - 一键安装脚本（NTR）
    ```shell
    curl -sL https://nxtrace.org/nt | bash -s -- --flavor ntr
    ```
  - 安装后的命令名：Full `nexttrace`，Tiny `nexttrace-tiny`，NTR `ntr`

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

    同 macOS Homebrew 安装方法；homebrew-core formula 提供 Full flavor（`nexttrace`），`nxtrace/nexttrace` tap 提供三种 flavor。

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
    - `nxtrace/nexttrace` tap 版（按 NTrace-core 最新 release 定期同步）
      ```shell
      brew tap nxtrace/nexttrace
      brew install nxtrace/nexttrace/nexttrace
      brew install nxtrace/nexttrace/nexttrace-tiny
      brew install nxtrace/nexttrace/ntr
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

请注意：

- `nexttrace-debs` APT 源由 nxtrace 和 wcbing 维护。
- 其它安装方式中的软件源大多由开源爱好者自行维护，不保证可用性和及时更新；如遇到问题请联系对应维护者，或使用本项目官方编译提供的二进制包。

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
| 环境自检（`--doctor`） | ✅ | ✅ | ✅ |
| 独立 MTU（`--mtu`）     |          ✅           |        ✅        |     —      |
| DNS 客户端（`-l` / `--dns`） |       ✅           |        —         |     —      |
| CDN 测速（`--speed`）   |          ✅           |        —         |     —      |
| IP 文本标注（`--nali`） |          ✅           |        —         |     —      |
| MTR TUI                 |          ✅           |        ✅         | ✅（默认） |
| MTR 报告（`-r`）        |          ✅           |        ✅         |     ✅     |
| MTR 宽报告（`-w`）      |          ✅           |        ✅         |     ✅     |
| MTR 原始输出（`--raw`） |          ✅           |        ✅         |     ✅     |
| MTR JSON / NDJSON | ✅ | ✅ | ✅ |
| MTR 自定义列（`--mtr-columns`） | ✅ | ✅ | ✅ |
| MTR 录制 / 回放（`--mtr-record` / `--mtr-replay`） | ✅ | ✅ | ✅ |
| Linux 策略路由（`--fwmark`，仅本地探测） | ✅ | ✅ | ✅ |
| Globalping（`--from`）  |          ✅           |        —         |     —      |
| WebUI（`--deploy`）/ MCP（`--deploy --mcp`） |          ✅           |        —         |     —      |
| 快速跟踪（`-F`）        |          ✅           |        ✅        |     —      |
| 默认运行模式            |      traceroute       |    traceroute    |  MTR TUI   |
| 二进制名                |      `nexttrace`      | `nexttrace-tiny` |   `ntr`    |

> **注意：** `APT (nexttrace-debs)` 与 `Homebrew tap (nxtrace/nexttrace)` 目前提供 **Full**（`nexttrace`）、**Tiny**（`nexttrace-tiny`）和 **NTR**（`ntr`）三种包；`homebrew-core`、AUR、Scoop 等其它包管理器目前仍仅提供 **完整版**（`nexttrace`）。

### 探测环境自检

```sh
nexttrace --doctor example.com
nexttrace --doctor --tcp --port 443 example.com
nexttrace-tiny --doctor -6 ::1
ntr --doctor --dev eth0 example.com
```

`--doctor` 输出纯文本诊断报告后退出，分别列出请求配置、目标解析与源地址选择、系统路由预测及实际后端初始化结果。自检不发送探测包、不读取捕获流量、不访问 GeoIP/API 服务，也不验证目标可达性；DNS/DoT 解析可能访问网络。

三个版本均支持。可用参数限于 `--ipv4`/`--ipv6`、`--tcp`/`--udp`、`--port`、`--source`、`--source-port`、`--dev`、`--tos`、`--dot-server`、`--timeout`、`--language`、`--no-color`、Windows 的 `--icmp-mode` 及对应短参数，详见 `--doctor --help`。目标仅接受域名或 IP，不接受 URL 和带区域后缀的 IPv6 地址。其他运行模式、JSON/RAW/文件输出及无关探测参数不能与自检组合。`--fwmark` 在自检中不受支持，不能用 Doctor 验证带标记的选路或 `SO_MARK` 权限；Doctor 的 `--source-port` 仅接受 `0..65535`，不接受探测模式的随机值 `-1`。

默认使用 ICMP、中文文本，每项网络检查超时为 5000ms。多个解析候选全部列出，自动选择首个符合地址族的地址，不进行交互选择。目标 DoT 失败不回退系统 DNS。单项失败后继续独立检查；报告写入 stdout，参数与报告写入错误写入 stderr。报告包含目标、源地址和接口信息，不包含 token 或代理凭据。

路由查询分别使用 Linux netlink、macOS 路由 socket 和 Windows `GetBestRoute2`。Linux IPv4 UDP 的内核协议 255 不支持 netlink 查询，因此通过 raw socket 的 `connect` 选源，全程不发送报文；该接口只返回源地址，`--doctor` 将出口接口及网关保留为未知。无法确认的信息显示“未知”。macOS/Windows 的预测不能覆盖全部协议、端口、源策略或 TOS 条件，报告会列明限制。socket 或过滤器初始化成功仅代表该步骤成功，不代表实际出口、收发能力或 TOS 生效。网络等待设有超时；原生同步初始化调用不承诺可被强制取消。

Windows 的 `--dev` 仅用于源地址选择，不能表述为设备绑定已验证。所有 WinDivert 自检使用 `NO_INSTALL`，不解压或安装驱动，不执行 `--init`。驱动尚未安装时标记为未验证，因为普通探测可能自动安装；Socket 备选结果单独报告。后端名称以实际构建架构为准，目前 WinDivert 探测路径编译于 Windows amd64。

退出码：**0** 必要检查完成且无失败；**1** 存在明确失败；**2** 参数错误；**3** 必要检查未能确认；**130/143** 收到 SIGINT/SIGTERM。目标可达性未验证本身不会导致退出码 3。

### 功能对比

- **`nexttrace`** — 完整版。包含 traceroute、独立 DNS 客户端与 MTU 模式、CDN 测速、IP 文本标注、MTR、Globalping、Fast Trace、WebUI（`--deploy`）与 MCP（`--deploy --mcp`）。
- **`nexttrace-tiny`** — 精简版。保留常规 traceroute、独立 MTU 和 Fast Trace；包含可选 MTR TUI、报告和 RAW；不含 DNS 客户端 / CDN 测速 / IP 文本标注 / Globalping / WebUI / MCP。适合嵌入式或极简环境。
- **`ntr`** — MTR 专用版。默认启动 MTR TUI。不含常规 traceroute、独立 DNS 客户端或 `--mtu`、CDN 测速、IP 文本标注、Globalping、Fast Trace、WebUI 或 MCP。

### 手动编译

需要 Go 1.27.1+ 环境。与正式发布一致的构建使用
`GOEXPERIMENT=nojsonv2`；CI 仍覆盖 Go 1.27 默认 JSON 运行时，但其 WebSocket
编码开销未达到发布性能门槛。详见 [JSON 运行时 ADR](docs/adr/0001-go127-json-runtime.md)。

```bash
export GOTOOLCHAIN=go1.27.1
export GOEXPERIMENT=nojsonv2

# 完整版（所有功能）
go build -trimpath -o dist/nexttrace -ldflags "-w -s" .

# 精简版（含可选 MTR，无 DNS 客户端、无 Globalping、无 WebUI）
go build -tags flavor_tiny -trimpath -o dist/nexttrace-tiny -ldflags "-w -s" .

# MTR 专用版
go build -tags flavor_ntr -trimpath -o dist/ntr -ldflags "-w -s" .
```

macOS 源码编译需要 macOS 13.0 或更高版本、Xcode Command Line Tools 和 cgo，因为 TCP/UDP 抓包会链接系统 `libpcap`。原生编译时，`go env CGO_ENABLED` 必须输出 `1`；若输出 `0`，请用 `go env -u CGO_ENABLED` 清除持久化覆盖，确认 `xcode-select -p` 成功后，再以 `CGO_ENABLED=1` 编译。

交叉编译示例：

```bash
# Linux arm64 精简版
GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=nojsonv2 \
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags flavor_tiny -trimpath -o dist/nexttrace-tiny_linux_arm64 -ldflags "-w -s" .
```

benchmark 工具已由 `go.mod` 固定版本；可用 `go tool benchstat before.txt after.txt` 比较保存的结果。

`tiny` 和 `ntr` 版本通过 **编译期 build tags** 裁剪模块——不是运行时开关。可通过 `go version -m <binary>` 验证 `nexttrace-tiny` 和 `ntr` 中不包含 `gin`、`globalping-cli` 与 `github.com/natesales/q`。

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

# 机器可读 RAW 输出：竖线分隔的数据行
nexttrace --traceroute --raw 1.0.0.1

# JSON 输出：stdout 只包含一个 JSON 文档
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

普通 traceroute 会报告停止原因：到达目标、收到终止性的 unreachable 响应（含 marker），或达到配置的最大跳数。`--json` 保持既有顶层结果结构，并新增可选 `StopReason`；其嵌套字段固定为小写 `hop`、`reason`、`responses`、`markers`，其中 `responses` 是人类可读描述，`markers` 是机器可读代码。classic/raw/JSON 不追加人类可读 footer；`--output` 会把同一停止原因以无 ANSI 的纯文本写入日志。

普通 traceroute 同时指定多个输出模式时，优先级为 `--json` > `--table` > `--classic` > `--raw` > `--output` > 实时输出。高优先级模式覆盖显式 `--output` 或 `--output-default` 时，NextTrace 会在 stderr 说明该选择，且不会创建被忽略的日志文件。

#### 下游迁移与显式传统模式

`-k/--traceroute` 在完整版和 tiny 中选择传统 traceroute；它不是 `--classic` 排版的别名。可与现有传统输出格式、Fast Trace 和文件目标组合；完整版还可与 Globalping 组合。与 `--mtr/-t`、`--report/-r`、`--wide/-w` 或独立 DNS、MTU、测速、IP 标注、deploy 模式同用会报错。

```bash
nexttrace --traceroute example.com
nexttrace -k example.com
nexttrace --traceroute --raw example.com
nexttrace --mtr --raw -q 10 example.com
```

| 调用 | 当前 | 默认切换后 |
|---|---|---|
| 仅目标地址 | 传统 traceroute | MTR |
| `--raw` | 传统 RAW | MTR RAW |
| `--traceroute` | 传统 traceroute | 传统 traceroute |
| `--traceroute --raw` | 传统 RAW | 传统 RAW |
| `--json/--table/--classic/--output/--output-default/--route-path` | 传统模式 | 自动选择传统模式 |

独立功能入口以及 Fast Trace、文件目标和 Globalping（仅完整版）保持各自工作流；独立模式中的 `--json` 仍属于该模式。显式 MTR 与传统专用格式的冲突规则不变。

未指定 `-q` 时，MTR TUI 和 `--mtr --raw` 持续运行；`--report/-r`、`--wide/-w` 默认每跳 10 次，与 `--raw` 组合时也一样；自动化调用应设置明确的 `-q`。MTR 中 `-i` 是每跳探测间隔（默认 1000ms），`-z` 被忽略；传统 traceroute 中 `-q` 默认每跳 3 次，`-i` 是 TTL 组间隔（默认 300ms），`-z` 是包间隔（默认 50ms）。

两种 RAW 不能只按成功行的列数区分：传统 RAW 成功行 12 列、超时行保留历史 8 列；MTR RAW 的数据行固定 12 列，并持续逐事件输出。解析器还需遵守各模式现有的信息头和 stderr 规则。`NEXTTRACE_UNINTERRUPTED` 配合 `--traceroute --raw` 时继续保留传统循环行为。

需兼容旧版本的 Bash 包装脚本可在调用前检测能力：

```bash
trace_mode=()
nexttrace_help=$(nexttrace --help)
if [[ "$nexttrace_help" == *"--traceroute"* ]]; then
  trace_mode=(--traceroute)
fi
nexttrace "${trace_mode[@]}" --raw example.com
```

旧版本不添加新参数；不要在探测失败后去掉参数自动重试，以免重复发起探测。


PS: 路由可视化的绘制模块为独立模块，具体代码可在 [nxtrace/traceMap](https://github.com/nxtrace/traceMap) 查看  
路由可视化功能需要每个 Hop 的地理位置坐标，目前支持搭配 NextTrace API、IPInfo 和 IP-API.com 使用。

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
  在 `Windows amd64` 上，ICMPv4/ICMPv6 未传 `--tos` 或显式 `--tos 0` 时继续走原生 Socket 发送路径；非零 ICMP `--tos` 使用 `WinDivert` 保留完整字段并要求管理员权限，`--icmp-mode 1` 选择 Socket 接收时也适用。原生 Windows ICMPv4 实测将所有测试的非零 TOS 发成了零。
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

### TOS / IPv6 Traffic Class（`-Q`、`--tos`）

`--tos` 设置完整的 8 位 IPv4 TOS / IPv6 Traffic Class 字段，范围 `0..255`，默认 `0`。计算方式为 `DSCP × 4 + ECN`：DSCP 46、ECN 0 使用 `--tos 184`；`--tos 46` 仍表示原始字段值 46（DSCP 11、ECN 2）。它修改探测报文头，实际选路与优先级由网络策略决定。

Linux、macOS 的发送路径使用原生 socket 选项或完整 IP 报文头。Linux 上非零 TOS 的自动源地址按该 TOS 的路由选择，支持与 `--fwmark` 组合；显式 `--source`、`--dev` 保留约束，各会话独立选源。路由查询按 raw socket 的内核条件匹配，不传用户态报文中的 TCP/UDP 端口。选源或 TOS 设置失败会终止探测，不转换为 MTR 丢包，也不退回默认字段值。

TOS 适用于现有 traceroute、MTR、Fast Trace、文件目标和 Web/API/MCP 探测入口；不影响 DNS/RDNS、GeoIP、API 等辅助请求。独立 MTU 和 Globalping 不支持该参数。JSON 中的 `tos` 记录请求配置，不代表抓包确认值。

BSD、Android 提供相关原生接口，但未完成本次原生抓包验收；实际运行仍受探测所需权限与系统限制影响。Windows 的 WinDivert 发送路径目前仅编译于 amd64，不能将其支持结论套用于 Windows arm64。

### Linux 策略路由（`--fwmark`）

`--fwmark 256` 或 `--fwmark 0x100` 为本地 traceroute、MTR 探测 socket 设置 32 位标记，覆盖各适用构建的 ICMP/TCP/UDP 和 IPv4/IPv6。对应的 `ip rule` 和路由表由用户配置，NextTrace 不修改系统规则；没有匹配的分流规则时，路径可以保持不变。

自动源地址按带标记的路由选择，显式 `--source`、`--dev` 保留约束，标记不会覆盖它们。带标记会话不使用旧的进程级源地址缓存。路由查询包含内核发送协议和 TOS（IPv4 UDP 的 IP_HDRINCL 路径使用协议 255），但不传入 TCP/UDP 端口，以匹配原生 raw socket 的内核选路：用户态构造的传输层头部不会作为该查询的端口条件。后续策略变化及 ECMP 仍可能影响选路。

范围为 `0..4294967295`，支持十进制及 `0x` 十六进制，不支持掩码。未传参数时不设置标记；显式 `0` 仍执行设置并检查权限。Linux 需要 `CAP_NET_ADMIN`，或 Linux 5.17 起的 `CAP_NET_RAW`。初始化失败会终止探测，不退回无标记方式。

DNS/RDNS、GeoIP、API 请求不继承探测标记。终端和 RAW 格式保持不变；MTR JSON/NDJSON 在 `effective_parameters.fwmark` 中记录显式指定的数值。macOS、Windows、BSD、Android，以及 doctor/MTU/DNS/speed/deploy 等独立模式、Globalping、Fast Trace 和文件目标明确拒绝该参数。

在 macOS 和 Linux 上，`--dev` 会绑定到指定源网卡。
在 Windows 上，`--dev` 会从指定网卡解析 source IP，并用该 source address 发起 ICMP/TCP/UDP 探测；它不会把 WinDivert 或 socket 绑定到真实出接口，实际出口仍可能由 Windows 路由表决定。独立 `--mtu` 模式也遵循相同的 source-address 语义，并额外使用网卡名查询本地 MTU。

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

# TCP/UDP Trace 可以自行指定源端口，默认使用随机一个固定的端口；`--source-port -1` 或环境变量 `NEXTTRACE_RANDOMPORT` 启用逐包随机源端口，适用于传统 traceroute 及 MTR 文本、JSON、录制
nexttrace --tcp --source-port 14514 www.bing.com
```

#### `NextTrace` 也支持独立 DNS 客户端模式

完整版 `nexttrace` 通过 NextTrace 自有 adapter 组合 [natesales/q v0.19.12](https://github.com/natesales/q/releases/tag/v0.19.12) 的公开子包，提供兼容 q 使用方式的 DNS 客户端。`-l` / `--dns` 必须作为第一个参数；其后的参数使用 q 风格的 flags 与位置参数。

```bash
# 通过指定的普通 DNS 服务器查询 MX 记录
nexttrace -l example.com MX @1.1.1.1

# 通过 DNS-over-TLS 查询 A 记录
nexttrace --dns example.com A @tls://one.one.one.one

# 通过 DNS-over-HTTPS 查询并输出 JSON
nexttrace --dns example.com A @https://cloudflare-dns.com/dns-query --format=json

# 查看 DNS 客户端专属帮助
nexttrace --dns --help
```

- 传输协议：UDP/TCP、DoT、DoH、DoQ、ODoH 与 DNSCrypt；plain DNS、DoT、DoH 与 DNSCrypt 支持 DNS Stamp 服务器格式。
- 输出格式：`pretty`、`column`、`raw`、`json` 与 `yaml`。
- 查询能力包括多服务器、多 RR 类型、反向解析、DNSSEC/EDNS、NSID、为 A/AAAA 应答补查 PTR 与递归 AXFR。
- 配置沿用 q 的 `~/.qrc`、`Q_DEFAULT_SERVER`、`NO_COLOR` 与 `SSLKEYLOGFILE` 约定。
- 该模式仅存在于完整版 `nexttrace`；`nexttrace-tiny` 与 `ntr` 不包含也不注册此功能。
- 这是独立的 CLI 工作流，不会替换 traceroute、GeoIP/RDNS、WebUI、MCP 或其它 service 路径使用的 DNS resolver。

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

#### `NextTrace` 也支持独立的 CDN 测速模式

```bash
# 默认使用 Apple CDN 后端
nexttrace --speed

# 改用 Cloudflare 后端
nexttrace --speed --speed-provider cloudflare

# 查看测速模式专属帮助
nexttrace --speed --help

# 机器可读输出
nexttrace --speed --json --non-interactive --no-metadata

# 指定测速节点 IP，或绑定 source address / 网卡
nexttrace --speed --endpoint 1.2.3.4
nexttrace --speed --source 192.0.2.10
nexttrace --speed --dev eth0
```

- `--speed` 仅在完整版 `nexttrace` 中提供，`nexttrace-tiny` 与 `ntr` 不注册该参数。
- 主 `nexttrace --help` 只展示顶层 `--speed` 入口；测速模式的详细参数放在 `nexttrace --speed --help`。
- 支持的后端为 `apple`（默认）和 `cloudflare`。
- 复用的公共参数：`--json`、`--language`、`--no-color`、`--dot-server`、`--timeout`、`--source`、`--dev`。
- 测速模式专属参数：`--speed-provider`、`--max`、`--threads`、`--latency-count`、`--non-interactive`、`--endpoint`、`--no-metadata`。
- 默认终端输出会展示候选节点、最终选中节点、客户端/服务端信息、空载延迟、下载/上传单线程与多线程轮次、负载延迟、总流量、warnings 和 degraded 状态。
- `--json` 时，stdout 只输出一个 JSON 文档。
- 退出码：`0` 表示成功，`2` 表示降级完成，`1` 表示失败，`130` 表示被中断。

#### `NextTrace` 可以对文本流中的 IP 字面量做归属标注

```bash
# 标注单行文本
nexttrace --nali 1.1.1.1

# 标注管道输出
dig example.com +short | nexttrace --nali --data-provider IPInfo --language en
```

- `--nali` 仅在完整版 `nexttrace` 中提供，`nexttrace-tiny` 与 `ntr` 不注册该参数。
- 仅标注 IPv4/IPv6 字面量，并复用 NextTrace 现有 GeoIP provider；不内置 CDN/CNAME 匹配、离线数据库、更新逻辑或 nali 专属路径。
- 复用的公共参数：`--data-provider`、`--language`、`--dot-server`、`--timeout`、`--dn42`、`-4`、`-6`。
- 该文本标注模式参考 [zu1k/nali](https://github.com/zu1k/nali) 的使用体验；nali 使用 [MIT License](https://github.com/zu1k/nali/blob/master/LICENSE)。

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

# 设置 DSCP 46、ECN 0（完整 TOS / traffic class 值为 184）
nexttrace -Q 184 example.com

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
| `--queries` | 常规 traceroute 的每跳采样数；MTR 中显式指定每跳探测次数 | traceroute: `3`；MTR report/wide（含 RAW）: 未指定时 `10`；其他 MTR TUI/raw: 未指定时无限 | 链路抖动大时可升到 `5-10` |
| `--max-attempts` | 每跳最大发包上限 | 默认按 `--queries` 自动推导 | 丢包严重、回包慢时增大 |
| `--parallel-requests` | 跨 TTL 的总并发 in-flight 探测数 | `18` | 多路径/负载均衡链路用 `1`；稳定链路一般 `6-18` |
| `--send-time` | 同一 TTL 组内相邻探测包间隔 | `50ms` | 设备限速时升到 `100-200ms`；MTR 下忽略 |
| `--ttl-time` | 常规 traceroute 的 TTL 组间隔；MTR 的每跳探测间隔 | traceroute: `300ms`；MTR: 未指定时 `1000ms` | 想加速就调低；远程/限速链路调高 |
| `--timeout` | 单个探测包超时 | `1000ms` | 跨洲或高丢包链路升到 `2000-3000ms` |
| `--psize` | 探测包大小 | 按协议/IP 族自动取最小合法值 | 含 IP + 探测协议头；负值表示每个 probe 在 `abs(value)` 内随机；超过出接口/路径 MTU 时，链路上可能看到分片 |
| `-Q`, `--tos` | IP TOS / traffic class | `0` | 设置完整 8 位字段（DSCP*4+ECN）；Windows amd64 非零 ICMP TOS 依赖 `WinDivert` |

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

三个版本均支持 MTR JSON：

```bash
nexttrace --mtr --json 1.1.1.1        # 持续输出 NDJSON 事件
nexttrace --mtr --json -q 10 1.1.1.1  # 限次输出 NDJSON 事件
nexttrace -r --json -q 10 1.1.1.1     # 结束后输出一个汇总 JSON
nexttrace -w --json -q 10 1.1.1.1     # 与 -r --json 完全等价
ntr --json 1.1.1.1                    # 持续输出 NDJSON 事件
```

JSON 固定输出完整可用元数据（FULL），忽略 `-y`，仍遵守 Geo/PTR、数据源及语言设置。`-r/-w --json` 共用 wide 采集规则。实时模式省略 `-q` 或传非正值表示无限运行；JSON 报告默认每跳 10 次，显式非正值报错。MTR 中 `--raw` 与 `--json` 互斥。full/tiny 单独 `--json` 保持传统 traceroute JSON，未来默认模式切换后也保持此行为。

NDJSON 输出 `start`、`probe`、`path_end`、`end` 事件，`seq` 连续递增。报告只输出一个对象，中断或失败时保留已有统计。诊断写入 stderr。退出码：完成 `0`，初始化/执行错误 `1`，参数错误 `2`，SIGINT `130`，SIGTERM `143`。完成不表示目的地可达，可达性读取 `path_end`。详见 [MTR JSON v1 契约及示例](docs/mtr-json.md)。

三个版本均支持保存和离线重开 MTR 会话：

```sh
nexttrace --mtr --mtr-record session.jsonl 1.1.1.1
nexttrace --mtr-replay session.jsonl
nexttrace --mtr-replay session.jsonl -r --json
```

`--mtr-record` 在当前 TUI、报告、RAW 或 JSON 输出之外新建私有记录文件，不自动开启 MTR：full/tiny 需配合 `-t/-r/-w`，ntr 使用默认模式。已有文件不会被覆盖。记录写入失败立即停止探测并报错，保留已写部分。首个会话错误及阶段不会被后续录制启动或结束错误覆盖。

回放只读取已保存的探测和元数据，不探测、不查询 DNS/Geo/PTR。终端默认显示最终统计并暂停；空格原速播放，末尾按空格从头播放；`p` 暂停播放，`r` 返回开头，`j/J` 输入相对会话开始时间 `HH:MM:SS[.mmm]` 定位。Host、统计列和历史视图操作继续可用。非 TTY 或 `-r/-w` 一次输出报告；`--json` 使用独立离线报告，包含记录完整性和回放位置。末尾截断可恢复完整部分，明确标记不完整并返回非零。历史窗口仍为三分钟，随回放位置移动；记录文件保留整个会话。详见[会话格式与恢复规则](docs/mtr-session.md)。

选择并排列 MTR 文字统计列：

```sh
nexttrace -t --mtr-columns loss,received,avg 1.1.1.1
nexttrace -w --mtr-columns received,snt,last 1.1.1.1
ntr --mtr-columns received 1.1.1.1
```

`--mtr-columns` 支持 `loss,snt,received,last,avg,best,wrst,stdev,dropped,gmean,jitter,javg,jmax,jint,space` 的任意非空子集及顺序，忽略大小写与列名两端空格；未知列、重复指标列、空项报错。`space` 增加一格显示间距，可重复，但至少需要一个指标列。`received` 显示为 `Rcv`。默认仍为 `Loss%、Snt、Last、Avg、Best、Wrst、StDev`。

参数适用于 TUI、非 TTY 表格和 report/wide，也支持离线回放的文字输出；不自动开启 MTR：full/tiny 需配合 `-t/-r/-w`，ntr 使用默认 MTR 模式。RAW、JSON、传统 traceroute 和其他独立模式会在初始化前拒绝该参数。自定义 TUI 保留完整数字及至少 8 格 Host；空间不足时显示提示，加宽终端或减少列后恢复。

按 `o/O` 编辑当前字段码：`L=Loss D=Drop R=Received S=Snt N=Last B=Best A=Avg W=Wrst V=StDev G=Gmean J=Jttr M=Javg X=Jmax I=Jint`。输入不区分大小写；每个空格增加一格实际显示间距，保留首尾及连续空格。Enter 校验并应用，Esc 取消，Backspace 删除末尾字符，Ctrl-U 清空。空串、纯空格、未知码和重复指标码保留编辑状态并显示错误。括号粘贴中的换行转为空格，不自动提交；草稿最多 256 个 ASCII 字符。

`Fields:` 页面逐行列出字段说明。计算口径、空格示例及 JSON 字段见 [MTR 列指标](docs/mtr-columns.md)。

编辑期间其他快捷键不生效，Ctrl-C 仍退出；探测、计数及暂停状态保持不变。从历史视图应用列后返回统计表，取消则保留历史视图。历史图表的固定列不变，列选择仅在当前会话有效；暂停期间仍可编辑和调整窗口大小。

在终端（TTY）中运行时，MTR 模式使用**交互式全屏 TUI**：

- **`q` / `Q`** — 退出（恢复终端，不留下输出）
- **`p`** — 暂停探测
- **空格** — 恢复探测
- **`r`** — 重置统计（计数器清零，显示模式保持不变）
- **`y`** — 循环切换主机显示模式：IP/PTR → ASN → City → Owner → Full
- **`n`** — 切换主机名显示方式：
  - 默认：PTR（无 PTR 时回退 IP）↔ 仅 IP
  - 启用 `--show-ips`：PTR (IP) ↔ 仅 IP
- **`e`** — 切换 MPLS 标签显示开/关
- **`o` / `O`** — 编辑统计列
- **`d` / `D`** — 切换可选历史显示；默认 TUI 仍是经典指标表
- **`g` / `G`** — 仅在历史显示中循环切换 History 图表：heatmap → bars → sparkline
- TUI 标题栏显示**源 → 目标**路由信息，指定 `--source`/`--dev` 时会展示对应信息。
- 使用 NextTrace API 且首选 API 元数据可用时，标题栏会显示首选 API IP 地址。
- 使用**备用屏幕缓冲区**，退出后恢复之前的终端历史记录。
- 当 stdin 非 TTY（如管道输入）时，降级为简单表格刷新模式。

历史显示会在经典表格显示期间同步收集最近 3 分钟、按探测时间戳归窗的历史样本，按 `d` 后显示 `Host`、`Last`、`Avg`、`Loss`、`History`。History 列使用固定 100ms 延迟刻度。默认使用 Unicode block/sparkline；启用 `--no-color` 时使用 ASCII，超时样本显示为 `x`。

致谢：可选 MTR 历史显示参考了 [TraceBar](https://github.com/tracebar-app/tracebar) 的连续 traceroute 历史可视化体验；TraceBar 使用 [MIT License](https://github.com/tracebar-app/tracebar/blob/main/LICENSE)。

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

如果当前数据源是 `NextTrace-API` 且首选 API 元数据可用，会先输出一行无色的 API 信息头：

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

raw stdout 契约仍严格保持 12 列。无限运行的 MTR 中，unreachable 边界是临时状态：后续 transit 证据可以重新开放更高跳数，之后新的 unreachable 边界可能再次输出 stderr 诊断。有限运行以最终结构化 `path_end` 为准。Web/API/MCP 的结构化记录通过逐 probe `response` 和最终 `path_end` 表达语义，不再按 responder IP 是否等于目标 IP 推断。

在 MTR 模式（`--mtr`、`-r`、`-w`，包括 `--raw`）下，`-i/--ttl-time` 设置的是**每个跳点的探测间隔**：同一跳点两次连续探测之间的等待时间（未显式指定时默认 1000ms）。`-z/--send-time` 在 MTR 模式下被忽略。

> 注意：`--show-ips` 仅在 MTR 模式（`--mtr`、`-r`、`-w`）生效，其他模式会忽略。
>
> 注意：`--mtr` 不可与 `--traceroute`、`--table`、`--classic`、`--output`、`--output-default`、`--route-path`、`--from`、`--fast-trace`、`--file`、`--deploy` 同时使用。

#### `NextTrace`支持用户自主选择 IP 数据库（目前支持：`NextTrace-API`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`, `IPInfoLocal`, `IPDB.One`, `CHUNZHEN`, `DN42`）

##### LeoMoeAPI 名称迁移

`LeoMoeAPI` 是本项目官方 API 已停止使用的旧名称，现统一称为 **NextTrace API**，其机器可读的 `data_provider` 规范值为 `NextTrace-API`。为兼容现有脚本，旧值 `LeoMoeAPI` 和 `LeoMoe` 仍作为大小写不敏感的静默输入别名保留，但 NextTrace 始终输出规范值。WebSocket/PoW 实现称为 **NextTrace API v3**，使用 token 鉴权的 HTTP 实现称为 **NextTrace API v4**。

```bash
# 可以自行指定 IP 数据库[此处为 IP-API.com]，不指定则默认使用 NextTrace API
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

没有可用的 NextTrace API v4 token 时，默认使用 NextTrace API v3 WebSocket/PoW。只想在当前 shell 会话启用 NextTrace API v4 HTTP GeoIP 接口时，运行设置命令并粘贴 token：

```bash
# Token 页面：
# GET https://api.nxtrace.org/v4/api-tokens

nexttrace -x
```

`nexttrace -x` 会把 token 写入临时文件：一个按父进程 PID 区分（父进程通常就是当前 shell），另一个是同用户 fallback 文件，用于 `go run` 这类 wrapper 命令。之后启动的 `nexttrace` 会按真实 `NEXTTRACE_API_V4_TOKEN`、父 PID 文件、fallback 文件的顺序读取，并把值加载到当前进程环境。该命令不会写 shell profile、永久环境变量或 `nt_config.yaml`。

设置 `NEXTTRACE_API_V4_TOKEN` 且当前数据源仍为 `NextTrace-API` 时，NextTrace 会请求 `GET https://api.nxtrace.org/v4/ipGeo?ip=<ip>`，并只通过 `X-NextTrace-Token: <token>` 请求头传 token；请求没有 JSON body。成功响应是直接映射到现有输出字段的 GeoIP JSON；配额信息只解析响应头（`X-NextTrace-Quota-Remaining`、`X-NextTrace-Quota-Expires-At`、`X-NextTrace-Quota-Cost`、`X-NextTrace-Quota-Source`），不改变默认输出格式。错误响应优先解析 `{"error":{"message":"..."}}`；已知状态包括 `400` 空/非法 IP、`401` unauthorized、`429` quota exhausted、`500` internal server error。NextTrace API v4 token 模式下的错误不会 fallback 到 NextTrace API v3。

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

启用 `NEXTTRACE_DEBUG` 时，已知 token、IPDB 凭据及代理 URL 的环境变量读取日志只显示变量名和存在状态，不输出其值。其他诊断仍可能包含目标、源地址和接口信息。

NextTrace 当前会读取下列环境变量。对于 `NEXTTRACE_*` 布尔开关，只识别 `1` 和 `0`，其他值会回退到内置默认值。为了避免混淆，修改后建议重启 NextTrace。

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
| `NEXTTRACE_HOSTPORT` | `api.nxtrace.org` | 覆盖 NextTrace API v3、tracemap、FastIP 等使用的后端地址，支持 `host` 或 `host:port`。 |
| `NEXTTRACE_TOKEN` | 未设置 | 预置 NextTrace API v3 Bearer Token；设置后将跳过 PoW 取 token 流程。 |
| `NEXTTRACE_API_V4_TOKEN` | 未设置 | NextTrace API v4 HTTP GeoIP token。未设置时，NextTrace 还会检查 `nexttrace -x` 写入的临时 token 文件；两者都不存在时，继续使用 NextTrace API v3 WebSocket/PoW。 |
| `NEXTTRACE_POWPROVIDER` | `api.nxtrace.org` | 指定 NextTrace API v3 的 PoW 服务提供方；当前内置的非默认别名为 `sakura`。 |
| `NEXTTRACE_DEPLOY_ADDR` | 未设置 | `--deploy` 模式下，当未传 `--listen` 时使用的默认监听地址。 |
| `NEXTTRACE_DEPLOY_TOKEN` | 未设置 | `--deploy` WebUI/API/WebSocket/MCP 访问 token。CLI `--deploy-token` 优先级更高。 |
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

#### 独立 DNS 客户端（仅完整版）

下列 q 兼容环境变量仅对独立 DNS 客户端模式生效。

| 变量名 | 默认值 | 说明 |
| --- | --- | --- |
| `Q_DEFAULT_SERVER` | 未设置 | 当命令行和 `~/.qrc` 均未指定服务器时使用的默认 DNS 服务器。 |
| `NO_COLOR` | 未设置 | 设置为任意非空值时禁用 DNS 客户端输出颜色。 |
| `SSLKEYLOGFILE` | 未设置 | 未传入 `--tls-key-log-file` 时，将 TLS 会话密钥写入该文件；文件包含敏感密钥材料。 |

#### 配置文件搜索

| 变量名 | 默认值 | 说明 |
| --- | --- | --- |
| `XDG_CONFIG_HOME` | 取决于系统 / Shell | 如果设置了该变量，NextTrace 也会从 `$XDG_CONFIG_HOME/nexttrace` 搜索 `nt_config.yaml`。 |

### 全部用法详见 Usage 菜单

以下为 macOS 完整版的帮助输出；Windows 还提供 `--init` 和 `--icmp-mode`。

```shell
usage: nexttrace [-h|--help] [-4|--ipv4] [-6|--ipv6] [-T|--tcp] [-U|--udp]
                 [--mtu] [-F|--fast-trace] [-p|--port <integer>] [-q|--queries
                 <integer>] [--max-attempts <integer>] [--parallel-requests
                 <integer>] [-m|--max-hops <integer>] [-d|--data-provider
                 (IP.SB|ip.sb|IPInfo|ipinfo|IPInsight|ipinsight|IPAPI.com|ip-api.com|IPInfoLocal|ipinfolocal|chunzhen|NextTrace-API|ipdb.one|disable-geoip|DN42|dn42)]
                 [--pow-provider (api.nxtrace.org|sakura)] [-n|--no-rdns]
                 [-a|--always-rdns] [-k|--traceroute] [-P|--route-path]
                 [-o|--output "<value>"] [-O|--output-default] [--table]
                 [-j|--json] [-c|--classic] [--dn42] [--raw] [-f|--first
                 <integer>] [-M|--map] [-e|--disable-mpls] [-V|--version]
                 [-x|--setup-api-v4-token] [-l|--dns] [--speed] [--nali]
                 [-s|--source "<value>"] [--fwmark "<value>"] [--source-port <integer>] [-D|--dev
                 "<value>"] [--listen "<value>"] [--deploy-token "<value>"]
                 [--mcp] [--deploy] [-z|--send-time <integer>] [-i|--ttl-time
                 <integer>] [--timeout <integer>] [--psize <integer>] [-Q|--tos
                 <integer>] [--dot-server
                 (dnssb|aliyun|dnspod|google|cloudflare)] [-g|--language
                 (en|cn)] [-C|--no-color] [--from "<value>"] [-t|--mtr]
                 [-r|--report] [-w|--wide] [--show-ips] [--mtr-columns <string>] [-y|--ipinfo <integer>]
                 [--mtr-record "<value>"] [--file "<value>"] [TARGET "<value>"]

                 An open source visual route tracking CLI tool

Arguments:

  -h  --help                         Print help information
  -4  --ipv4                         Use IPv4 only
  -6  --ipv6                         Use IPv6 only
  -T  --tcp                          Use TCP SYN for tracerouting (default
                                     dest-port is 80)
  -U  --udp                          Use UDP SYN for tracerouting (default
                                     dest-port is 33494)
      --mtu                          Run standalone UDP path-MTU discovery mode
                                     with streaming output and GeoIP/RDNS
  -F  --fast-trace                   One-Key Fast Trace to China ISPs
  -p  --port                         Set the destination port to use. With
                                     default of 80 for "tcp", 33494 for "udp"
  -q  --queries                      Traceroute: latency samples per hop
                                     (default 3). MTR: max probes per hop. 0 =
                                     unlimited in TUI/raw/JSON streams. JSON
                                     reports require a positive count. When
                                     omitted: 10 with --report/--wide
                                     (including --raw), otherwise unlimited
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
  -d  --data-provider                Choose IP Geographic Data Provider
                                     [NextTrace-API, IP.SB, IPInfo, IPInsight,
                                     IP-API.com, IPInfoLocal, ipdb.one,
                                     chunzhen, disable-geoip, DN42]. Default:
                                     NextTrace-API
      --pow-provider                 Choose PoW Provider for NextTrace API v3
                                     [api.nxtrace.org, sakura] For China
                                     mainland users, please use sakura.
                                     Default: api.nxtrace.org
  -n  --no-rdns                      Do not resolve IP addresses to their
                                     domain names
  -a  --always-rdns                  Always resolve IP addresses to their
                                     domain names
  -k  --traceroute                   Use traditional traceroute mode; with
                                     --raw, preserve traditional raw output
  -P  --route-path                   Print traceroute hop path by ASN and
                                     location
  -o  --output                       Write realtime trace output and final stop
                                     reason to FILE
  -O  --output-default               Write realtime trace output and final stop
                                     reason to the default log file
                                     (/tmp/trace.log)
      --table                        Output trace results as a final summary
                                     table (traceroute report mode)
  -j  --json                         Output JSON; MTR streams NDJSON unless
                                     --report/--wide is selected
  -c  --classic                      Classic Output trace results like
                                     BestTrace
      --dn42                         DN42 Mode
      --raw                          Machine-friendly output; use --traceroute
                                     --raw to preserve traditional raw output.
                                     With MTR (--mtr/-r/-w), enables streaming
                                     raw event mode
  -f  --first                        Start from the first_ttl hop (instead of
                                     1). Default: 1
  -M  --map                          Disable Print Trace Map
  -e  --disable-mpls                 Disable MPLS
  -V  --version                      Print version info and exit
  -x  --setup-api-v4-token           Store a session-only NextTrace API v4
                                     token in a temporary file
  -l  --dns                          Run DNS client mode. See `nexttrace --dns
                                     --help` for details
      --speed                        Run CDN speed test mode. See `nexttrace
                                     --speed --help` for details
      --nali                         Annotate IP literals in text using
                                     NextTrace GeoIP data
  -s  --source                       Use source address src_addr for outgoing
                                     packets
      --fwmark                       Linux probe socket mark (decimal or 0x
                                     hex); traceroute/MTR only
      --source-port                  Use source port src_port for outgoing
                                     packets
  -D  --dev                          Use the specified network device for
                                     explicit source selection. On Windows,
                                     this selects the device source address;
                                     routing may still choose the egress
                                     interface
      --listen                       Set listen address for web console (e.g.
                                     127.0.0.1:30080)
      --deploy-token                 Set bearer token for --deploy
                                     WebUI/API/WebSocket/MCP access
      --mcp                          Enable MCP endpoint under --deploy at /mcp
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
                                     minimum legal size for the chosen protocol
                                     and IP family; raise for MTU or
                                     large-packet testing. Negative values
                                     randomize each probe up to abs(value)
  -Q  --tos                          Set the full 8-bit IP type-of-service /
                                     traffic class [0-255]: DSCP*4+ECN (DSCP
                                     46, ECN 0 = 184). Default: 0
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
      --mtr-columns                  MTR text columns in order: loss, snt,
                                     received, last, avg, best, wrst, stdev,
                                     dropped, gmean, jitter, javg, jmax, jint,
                                     space (does not enable MTR)
      --show-ips                     MTR only: display both PTR hostnames and
                                     numeric IPs (PTR first, IP in parentheses)
  -y  --ipinfo                       Set initial MTR TUI host info mode (0-4).
                                     TUI only; ignored in --report/--raw/--json.
                                     0:IP/PTR 1:ASN 2:City 3:Owner 4:Full.
                                     Default: 0
      --mtr-record                   Save this MTR session to a new file for
                                     offline replay
      --file                         Read IP Address or domain name from file
      TARGET                         Trace target: IPv4 address (e.g. 8.8.8.8),
                                     IPv6 address (e.g. 2001:db8::1), domain
                                     name (e.g. example.com), or URL (e.g.
                                     https://example.com)

  --doctor  Check local probe prerequisites without sending probes; see --doctor --help
  --mtr-replay FILE  Open a saved MTR session offline; see --mtr-replay --help
```

## 项目截图

![image](https://user-images.githubusercontent.com/59512455/218505939-287727ce-7207-43c4-8e31-fcda7df0b872.png)

![image](https://user-images.githubusercontent.com/59512455/218504874-06b9fa4b-48e0-420a-a195-08a1200d65a7.png)

## 第三方 IP 数据库 API 开发接口

NextTrace 所有的的 IP 地理位置 `API DEMO` 可以参考[这里](https://github.com/nxtrace/NTrace-core/blob/main/ipgeo/)

你可以在这里添加自己的 API 接口。为了让 NextTrace 正确显示返回内容，请参考 `ipgeo/ipgeo.go` 中的 `IPGeoData` 字段。

✨NextTrace API 的后端 Demo

[GitHub - sjlleo/nexttrace-backend: NextTrace BackEnd](https://github.com/sjlleo/nexttrace-backend)

NextTrace API v3 使用 Proof of Work（PoW）机制防止滥用，NextTrace 客户端使用 powclient 库。PoW 客户端与服务端均已开源，相关问题请提交到对应仓库。

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

## GlobalTrace

`GlobalTrace` 是一个 `Globalping x NextTrace` 开源 Web 路由追踪项目。它借助 Globalping 遍布全球的 Probe 从不同地区发起 `MTR` measurement，并结合 NextTrace / NTrace 骨干网 IP 数据库补充 hop 的 GeoIP、ASN 与网络归属信息。

网站：[https://lg.nxtrace.org](https://lg.nxtrace.org)

项目地址：[nxtrace/GlobalTrace](https://github.com/nxtrace/GlobalTrace)

## NextTrace Web

`NextTrace Web`是一个`MTR`风格的`NextTrace`网页版服务端实现，提供了包括`Docker`在内多种部署方式。

[https://github.com/nxtrace/nexttraceweb](https://github.com/nxtrace/nexttraceweb)

## Deploy WebUI 与 MCP

完整版 `nexttrace` 可以启动本机 WebUI/API/WebSocket 服务：

```bash
nexttrace --deploy
```

MCP 是 deploy 的子模式，复用同一套网络服务栈并暴露在 `/mcp`：

```bash
nexttrace --deploy --mcp
nexttrace --deploy --mcp --listen 0.0.0.0:1080 --deploy-token "$TOKEN"
```

监听 loopback 地址（`127.0.0.1`、`::1`、`localhost`）时默认免 token。监听外网地址时必须启用 token；如果没有通过 `--deploy-token` 或 `NEXTTRACE_DEPLOY_TOKEN` 设置，NextTrace 会启动时随机生成 token 并输出到 stdout。若 stdout 会被日志系统、CI 控制台或平台采集，建议通过 `--deploy-token` 或 `NEXTTRACE_DEPLOY_TOKEN` 显式提供 token，避免泄漏。API、WebSocket 与 MCP 客户端可使用 `Authorization: Bearer <token>` 或 `X-NextTrace-Token`；浏览器 WebUI 用户可访问 `/auth/login` 登录。

### 在 Agent 客户端注册 MCP

先启动 NextTrace。MCP endpoint 是 Streamable HTTP，不是 stdio：

```text
http://127.0.0.1:1080/mcp
```

监听外网地址或手动配置 token 时，把 token 放在 HTTP header 中。不要把 deploy token 放进 URL query。

通用 MCP 客户端配置示例：

```json
{
  "mcp": {
    "servers": {
      "nexttrace": {
        "url": "http://127.0.0.1:1080/mcp",
        "transport": "streamable-http",
        "headers": {
          "Authorization": "Bearer <token>"
        }
      }
    }
  }
}
```

也可以把 `headers` 改为使用 `X-NextTrace-Token: <token>`，与 `Authorization: Bearer <token>` 等价。

OpenClaw 可通过 [`openclaw mcp set`](https://docs.openclaw.ai/zh-CN/cli/mcp) 保存同样的 server 定义：

```bash
openclaw mcp set nexttrace '{
  "url": "http://127.0.0.1:1080/mcp",
  "transport": "streamable-http",
  "headers": {
    "Authorization": "Bearer <token>"
  }
}'
```

`openclaw mcp set` 只保存 MCP server 定义；它不会启动 NextTrace，也不会验证 endpoint 是否可达，所以需要先运行 `nexttrace --deploy --mcp`。

下面列出的项为 MCP 协议中 `tools[].name` 的工具名/ID，客户端在 MCP Tool Calling 中引用这些 ID，而不是把它们当作 HTTP path。Agent 接入后可先调用：

- `nexttrace_capabilities`
- `nexttrace_traceroute`
- `nexttrace_globalping_trace`
- `nexttrace_globalping_limits`

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

NextTrace 有多个数据源可以选择，目前默认使用由本项目维护的 NextTrace API。

该项目由 OwO Network 的 [Missuo](https://github.com/missuo) && [Leo](https://github.com/sjlleo) 发起，由 [Zhshch](https://github.com/zhshch2002/) 完成最早期架构的编写和指导，后由 Leo 完成了大部分开发工作，现主要交由 [tsosunchia](https://github.com/tsosunchia) 完成后续的二开和维护工作。

NextTrace API 最初由 [Leo](https://github.com/sjlleo) 为 Leo Network 开发整套后端 API；该接口未经允许不可用于任何第三方用途。

NextTrace API 早期数据主要来自 IPInsight、IPInfo。随着项目发展，越来越多的志愿者参与进来；目前近一半数据由社区提供，另一半主要来自 IPInfo、IPData、BigDataCloud、IPGeoLocation 等第三方数据。

NextTrace API 的骨干网数据有近 70% 来自社区反馈或项目组成员校准，这为本项目的路由跟踪基础功能提供了一定保障。但全球骨干网体量庞大，我们没有 IPIP 等商业公司拥有的海量监测节点，因此数据精准度无法与 BestTrace（IPIP）相提并论。

NextTrace API 已尽力校准常见骨干网路由，但封闭型 ISP 的路由仍可能定位错误；IPInsight、IPInfo 对此类数据也可能无法正确定位。如对此类数据的精确性要求很高，请优先使用 BestTrace。

我们不保证我们的数据一定会及时更新，也不保证数据的精确性，我们希望您在发现数据错误的时候可以前往 issue 页面提交错误报告，谢谢。

使用 NextTrace API 即表示您已了解其数据精确性限制；引用其中数据所引发的问题由使用者自行承担。

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

[![Star History Chart](https://star-history.dera.page/svg?repos=nxtrace/NTrace-core&type=Date)](https://star-history.dera.page/#nxtrace/NTrace-core&type=Date)
