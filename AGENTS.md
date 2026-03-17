# NTrace-core 项目记忆文件（2026-03 快照，rev-3）

# 供 LLM 在后续会话中快速加载上下文，减少重复分析。

## 项目概览

- 名称：NextTrace (NTrace-core)
- 仓库：github.com/nxtrace/NTrace-core
- 模块：`github.com/nxtrace/NTrace-core`
- 语言：Go（`go 1.26.0`）
- 入口：`main.go -> cmd.Execute()`
- 核心能力：ICMP/TCP/UDP traceroute、GeoIP/RDNS、MTR 连续探测、Web/API、多平台构建

## 构建与测试（必须遵守）

- 常用命令：
  - 构建：`go build ./...`
  - 测试：`go test ./...`
- 交叉编译脚本：`.cross_compile.sh`
- Darwin 下 `trace/internal/icmp_darwin.go` 已不再使用 `//go:linkname`，改为
  `syscall.Socket` + `os.NewFile` + 自定义 `icmpPacketConn`（实现 `net.PacketConn` /
  `net.Conn` / `syscall.Conn` + `ReadMsgIP` 以满足 `x/net/internal/socket.ipConn`
  接口），并在 `ReadFrom` 中调用 `stripIPv4Header` 剥离 macOS DGRAM ICMP socket
  返回的外层 IP 头。

## 当前 CLI 语义（重点）

### 常规 traceroute 路径

- `--table`：现在是"最终汇总表"模式（一次探测完成后输出汇总表），不再是旧的异步 table 刷新模式。
- `--route-path`：仍由 `reporter.New(...).Print()` 负责（与 MTR report 无关）。

### 独立 `--mtu` 路径

- `--mtu`：独立 UDP path-MTU / tracepath 风格模式，不复用普通 `trace.Traceroute` / MTR / Web 路径。
- flavor 可用性：仅 `nexttrace` / `nexttrace-tiny` 包含；`ntr` 不注册该 flag。
- 输出：
  - `TTY`：当前 TTL 占位后原地更新，边探测边刷行。
  - `非 TTY`：TTL 定稿后逐行流式输出，不使用 renderer 自己的 ANSI 控制序列。
  - `--json`：输出独立 mtu schema；`hop.geo` 已存在。
- 参数语义：
  - 复用 `--data-provider`、`--language`、`--no-rdns`、`--always-rdns`、`--dot-server`。
  - `--mtu` 仍只支持 UDP；显式 `--tcp` 冲突报错。
- Geo/RDNS：
  - `trace/mtu` 自带独立 metadata helper，不依赖普通 `trace.Hop.fetchIPData`。
  - 流式事件会先输出基础 hop，再在同一 TTL 内补一条带 Geo/RDNS 的 update，最后 `ttl_final` 定稿。
  - macOS 上曾有 `Warning: macOS --mtu support is experimental.` 提示，现已删除；不要再假设 CLI 会打印这句。

### `--psize` / `--tos` 语义与平台差异

- `--psize` 现在统一对齐 `mtr -s/--psize`：
  - 用户输入语义是“含 IP + 当前探测协议头的总字节数”。
  - 内部 `trace.Config.PktSize` 仍保存 payload bytes。
  - 未显式传入时，不再固定默认 `52`，而是按协议/IP 族自动取最小合法值：
    - ICMPv4 / UDPv4 = `28`
    - TCPv4 = `44`
    - ICMPv6 = `48`
    - UDPv6 = `50`
    - TCPv6 = `64`
  - 负数 `--psize` 表示“每个 probe 独立随机”，CLI 允许 `--psize -84` 这种写法并会在解析前归一化。
- `--tos` / `-Q`：
  - 范围固定 `0..255`。
  - `--mtu` 与 Globalping 显式传 `--psize` / `--tos` 会直接报不支持。
- 平台发送路径差异（这是后续判断 bug 的关键记忆）：
  - Linux / 其他 Unix：
    - `ICMP/TCP/UDP` 的 IPv4/IPv6 都走原生 socket/raw socket 路径。
    - `--tos` 只是在现有路径上设置 `TOS/TrafficClass`，不会切换实现。
  - macOS：
    - 与 Linux 类似，`ICMP/TCP/UDP` 的 IPv4/IPv6 都走原生发送路径。
    - `--tos` 同样只是在现有路径上设置 `TOS/TrafficClass`。
  - Windows：
    - `TCP/UDP` 的 IPv4/IPv6 一直走 WinDivert raw send。
    - `ICMPv4` 一直走 socket path（`SetTOS` / `SetTTL`）。
    - `ICMPv6`：
      - 默认或 `--tos 0`：继续走原生 socket path，只设置 `HopLimit`，保持与 `v1.5.2` 一致。
      - 非零 `--tos`：切到 WinDivert raw send，直接发送完整 `IPv6 + ICMPv6` 报文，因为 Windows 的 `x/net/ipv6.PacketConn` 不能可靠设置 `TrafficClass`。
    - 因此，Windows 上只有“`ICMPv6` 且 `--tos != 0`”这个组合会额外依赖 WinDivert 发送能力；README 中英两份都已写明。

### 间隔默认值（分层体系）

- `-z/--send-time`：每包间隔，默认 `defaultPacketIntervalMs = 50` ms。
- `-i/--ttl-time`：
  - **常规 traceroute**：TTL 分组间隔，默认 `defaultTracerouteTTLIntervalMs = 300` ms。
  - **MTR 模式**：`normalizeMTRTraceConfig()` 始终覆盖为 `defaultMTRInternalTTLIntervalMs = 0` ms（各 TTL 间不间隔）。
  - MTR 每跳探测间隔由 `-i` 显式传值 或 默认 1000ms 决定（见下文 `-q/-i` 语义）。`-z/--send-time` 在 MTR 模式下被忽略。

### MTR 相关参数

- `-t/--mtr`：开启 MTR 交互模式（TTY 全屏 TUI）。
- `-r/--report`：MTR 报告模式（非交互），隐式开启 MTR。
- `-w/--wide`：宽报告模式，隐式等价 `--mtr --report --wide`。
- `--raw`：与 MTR 组合时进入 **MTR raw 流式模式**（`runMTRRaw`），不再与 MTR 冲突。
- 有效 MTR 开关：`effectiveMTR = mtr || report || wide`。
- MTR 三路分支（`chooseMTRRunMode`）：
  1. `effectiveMTRRaw` → `runMTRRaw`（流式行输出，适合管道/脚本）
  2. `effectiveReport` → `runMTRReport`（非交互报告表）
  3. 默认 → `runMTRTUI`（全屏 TUI）
- MTR 冲突参数（会直接报错退出）：`--table` `--classic` `--json` `--output` `--route-path` `--from` `--fast-trace` `--file` `--deploy`。
  - **注意**：`--raw` 不再是冲突参数。

### MTR 中 `-q/-i/-y` 的新语义

- `-q/--queries`：
  - 在 MTR report 下表示每跳探测次数，默认 10（仅当用户未显式传 `-q`）。
  - 在 MTR TUI 下表示每跳最大探测次数，未显式传时默认无限运行。
- `-i/--ttl-time`：
  - 在 MTR 下表示每跳探测间隔毫秒，默认 1000ms（仅当用户未显式传 `-i`）。
  - 各 TTL 间内部扫描间隔固定 0ms（`normalizeMTRTraceConfig` 覆盖为 `defaultMTRInternalTTLIntervalMs = 0`）。
  - `-z/--send-time` 在 MTR 模式下被忽略。
- `-y/--ipinfo <0..4>`：
  - TUI 初始 Host 显示模式，默认 0（IP/PTR only）。
  - 0=Base(IP/PTR) 1=ASN 2=City 3=Owner 4=Full
  - 仅 TUI 模式生效，report/raw 不受影响。

### MTR report wide / non-wide 区别

- **wide 模式**（`-w` 或 `--mtr --report --wide`）：
  - 查询 GeoIP，显示完整 host 信息（ASN + geo + MPLS）。
- **非 wide 模式**（`-r` 或 `--mtr --report`）：
  - `normalizeMTRReportConfig` 设 `IPGeoSource=nil`（不查 geo）、`AlwaysWaitRDNS=true`。
  - 显示 `formatCompactReportHost`：仅 IP/PTR + ASN，无 geo 列。

## MTR 运行链路（重要文件）

- 入口与调度：`cmd/mtr_mode.go`（~315 行）
  - `runMTRTUI(...)` / `runMTRReport(...)` / `runMTRRaw(...)`
  - `normalizeMTRTraceConfig(conf)` / `normalizeMTRReportConfig(conf, wide)`
  - `buildAPIInfo(...)` / `buildRawAPIInfoLine(...)`
  - MTR CLI 现在统一使用 `signal.NotifyContext(...)` 管理 Ctrl-C / SIGTERM；不再保留额外的 `sigCh` + goroutine 等待器。
- 交互控制：`cmd/mtr_ui.go`
  - alternate screen + raw mode
  - 输入状态机 `mtrInputParser`（字节流，吞掉 CSI/SS3/OSC/鼠标/焦点等序列）
  - Enter/Leave 显式关闭输入扩展模式：1000/1002/1003/1006/1015/1004/2004
  - Quit 路径会先判空 `cancel`，因此 `newMTRUI(nil, ...)` / 测试注入 nil 不会 panic。
- 核心探测循环：`trace/mtr_runner.go`
  - `RunMTR` / `mtrLoop` / `RunMTRRaw`
  - 支持暂停、重置、流式预览（`ProgressThrottle` 默认 200ms）
  - ICMP 持久引擎 + TCP/UDP fallback
- 统计聚合：`trace/mtr_stats.go`
  - `MTRAggregator` / `MTRHopStat`
  - unknown 合并策略：单路径时把 unknown 合并到唯一已知路径，避免同 TTL 分裂成 waiting + 真实 IP 两行
- 输出层：
  - TUI：`printer/mtr_tui.go`
  - table/report：`printer/mtr_table.go`
  - raw 行格式化：`printer.FormatMTRRawLine(rec)`
  - TUI 颜色：`printer/mtr_tui_color.go`

## MTR 交互行为（当前）

- `q`/`Q`/`Ctrl-C`：退出
- `p`：暂停
- `SPACE`：恢复
- `r`：重置统计
- `y`：切换 Host 显示模式（IP/PTR → ASN → City → Owner → Full → 循环）
- `n`：切换 Host 基名显示（PTR-or-IP / IP-only）
- `e`：切换 MPLS 标签显示（toggle MPLS on/off）

## MTR 显示与统计规则（当前）

- Host 显示支持 5 种模式（`-y/--ipinfo` 设初始值，`y` 键运行时循环）：
  - `HostModeBase=0`：仅 IP/PTR，无 ASN 前缀
  - `HostModeASN=1` / `HostModeCity=2` / `HostModeOwner=3` / `HostModeFull=4`
  - `HostNamePTRorIP` / `HostNameIPOnly`
- 默认语言：`cn`（`--language en` 才优先英文字段）
- waiting 判定：`loss >= 99.95 && IP=="" && Host==""`
  - 显示为 `(waiting for reply)`
  - 指标列（Loss/Snt/Last/Avg/Best/Wrst/StDev）留空
- TUI Host 对齐（重要，已从 tab 改为手动空格）：
  - `buildTUIHostParts(stat, mode, nameMode, lang, showIPs)` 生成结构化 parts
  - `computeTUIASNWidth(stats, ...)` 扫描所有 hop 确定 ASN 列最大宽度
  - `formatTUIHost(parts, asnW)` 用 `padRight(asn, asnW)` + 空格拼接（不用 `\t`）
  - ASN 为空但 IP 已知时填 `"AS???"` 占位符，保证列对齐（HostModeBase 除外，该模式不显示 ASN）
  - waiting hop 不填占位符
- compact report host（非 wide report）：
  - `formatCompactReportHost(stat, nameMode, lang)` 仅输出 hostname/IP + ASN
- TUI 其他特性：
  - 终端宽度自适应 + CJK 宽度计算（go-runewidth）
  - 窄屏右锚定指标区
  - 动态 hop 前缀宽度（覆盖 3 位/4 位 TTL）
  - MPLS 独立续行显示
  - 紧凑指标列宽度：Loss=5 Snt=3 RTT=7 RTTMin=5

## MTR 目的地检测与高 TTL 丢弃

- 当 `knownFinalTTL` 已确定后，所有 `TTL > knownFinalTTL` 的调度槽位被标记为 `disabled`。
- disabled TTL 的探测回包（包括在途探测返回的 dst-ip 回复）**一律丢弃**，不折叠、不计入任何统计。
- **MaxPerHop 上限检查**（`states[originTTL].completed + inFlightCount >= MaxPerHop`）：
  - 调度时使用 `completed + inFlightCount >= MaxPerHop` 防止超发。
  - 完成时仍检查 `completed >= MaxPerHop` 丢弃溢出结果。
- `originTTL < curFinal` 时（更低 TTL 先到 dst-ip → 降低 `knownFinalTTL`）：
  - 保存 `oldFinal`，更新 `knownFinalTTL = originTTL`，disable 所有 `originTTL+1..maxHops`。
  - 调用 `agg.ClearHop(oldFinal)`：清除旧 finalTTL 的聚合数据（避免幽灵行），**不合并**到新 finalTTL。
  - 新 finalTTL 由独立的 per-hop 调度器自行积累新鲜探测数据，不存在 Snt 膨胀问题。
- 调度状态（`inFlightCount`/`nextAt`/`consecutiveErrs`）更新在 `originTTL`。
- 统计聚合（`completed++`/`agg.Update`/`onProbe`）均使用 `originTTL`（不再有 `accountTTL` 分离）。

## MTR Per-Hop 调度器关键设计（当前）

- **多 in-flight 探测**：每 TTL 允许最多 `MaxInFlightPerHop`（默认 3）个并发探测。
  - `mtrHopState.inFlightCount` 是计数器（非 bool）。
  - 这解决了高丢包 hop 因超时阻塞导致 Snt 积累速率远低于低丢包 hop 的问题。
- **nextAt 基于发送时间**：`launchProbe` 时设 `nextAt = now + hopInterval`。
  - 不再等探测完成才设 nextAt，调度器可在超时探测还在飞行中时为同一 TTL 发射新探测。
  - 这保证了所有 TTL 的 Snt 积累速率大致相同，不受丢包率影响。
- **全局并发限制**：`inFlight`（全局计数器）< `parallelism` 仍然有效。
- **`MaxInFlightPerHop` 配置**：`mtrSchedulerConfig.MaxInFlightPerHop`，默认动态计算。
  - 动态默认 = `ceil(Timeout / HopInterval) + 1`（至少 1）。
  - 例：`Timeout=2s, HopInterval=1s` → 默认 3；`Timeout=2s, HopInterval=200ms` → 默认 11。
  - 用户显式设置 > 0 时优先使用用户值。

## MTR 引擎关键机制（易踩坑）

- 目的地提前停止：
  - `knownFinalTTL`（持久缓存）用于缩短后续探测的 TTL 上界；高 TTL 标记 disabled 后不再调度。
- seq 16 位回卷处理：
  - `seqWillWrap(...)` 触发 `rotateEngine(...)`
  - 轮换 echoID 并重建 listener，协议层隔离新旧回包。
- 额外安全网：
  - onICMP 中有 RTT 合理性检查（`<=0` 或 `>timeout` 丢弃）。
- 流式预览：
  - 仅已发送 TTL 才会参与预览；未发送 TTL 保持 nil 槽位，避免提前计入 Snt/Loss。

## Web Console / WebSocket（server/）

### WS 架构（`server/ws_handler.go`，~451 行）

- **异步写模型**：`wsTraceSession` 使用 `sendCh`（buffered channel，1024）+ `writeLoop` goroutine。
  - 调用方通过 `send(envelope)` 非阻塞投递；channel 满时返回 `errWSSlowConsumer`。
  - `writeLoop` 从 `sendCh` 取消息，`SetWriteDeadline` + `WriteJSON`。
- **关闭路径**：
  - `closeWithCode(code, reason)`：异常关闭（slow consumer / write error），关 `stopCh` + 发 close frame。
  - `finish()`：正常结束，`sendMu` 下关 `sendCh`，等 `writerDone`，再关 conn。
  - 两者均幂等（`closeOnce` / `finishOnce`）。
- **可测试性**：`wsConn` 接口 + `fakeWSConn` mock（`server/ws_handler_test.go`）。
- **常量**：`wsSendQueueSize=1024`，`wsWriteTimeout=5s`。

### Web MTR 调度模式（重要变更）

- **已从 round-based 迁移到 per-hop 调度**。
- `runMTRTrace()`：
  - 优先读 `HopIntervalMs`，fallback `IntervalMs`，再缺省 1000ms。
  - `MaxRounds` → `MaxPerHop`（0 = 无限运行直到客户端断开）。
  - 不再使用 legacy round-based 的 `Interval` / `RunRound`。
- `executeMTRRaw()` 两路分支：
  - `HopInterval > 0`：per-hop 模式，仅在 LeoMoe/FastIP 初始化阶段短暂加锁；长期探测不再依赖 `SrcDev` / `DisableMPLS` 等进程级全局。
  - fallback：legacy round-based 模式（保留兼容），`RunRound` 回调内 per-round 锁定。
  - `trace/runMTRRawRoundBased()` 也会先做 `normalizeRuntimeConfig(&cfg)`，因此 legacy raw 路径同样能继承 `SourceDevice`；`DisableMPLS` 不再从全局反向覆盖会话配置。
- `traceRequest` 新增 `HopIntervalMs` 字段（`json:"hop_interval_ms"`），与 `IntervalMs` 解耦。
- 前端 MTR 请求现在发送 `hop_interval_ms=1000`，不再把旧的 `interval_ms=2000` 当默认值。
- 前端 raw 聚合键现在按 TTL 折叠，避免同一 hop 的 timeout / success 被拆成两行。

### 前端渲染节流（`server/web/assets/app.js`）

- MTR raw 消息通过 `scheduleMTRRender()` 节流，最小间隔 100ms，优先 `requestAnimationFrame`。
- `cancelScheduledMTRRender()` 在 `clearResult`、socket close/error 路径调用，避免孤儿回调。
- `flushMTRRender()` 立即执行挂起渲染。

### 其他 server 文件

- `server/server.go`：Gin 路由注册
- `server/handlers.go`：REST 接口
- `server/mtr.go`：MTR 专用 handler 逻辑
- `server/trace_handler.go`：traceroute handler
- `server/cache_handler.go`：缓存

## DoT 与 Geo DNS

- `--dot-server` 不仅影响目标域名解析，也影响 GeoIP API / LeoMoe FastIP 的域名解析链路。
- 关键文件：`util/dns_resolver.go`
  - `SetGeoDNSResolver(dotServer)`
  - `WithGeoDNSResolver(dotServer, fn)`：为 Web/API 请求提供作用域化的 resolver 切换；不同 resolver 串行切换，相同 resolver 允许安全嵌套，避免 `GetSourceWithGeoDNS` + 外层作用域组合时死锁。
  - `geoResolverOverride` 的读写现在也走 `geoMu`，避免测试覆盖 resolver 时的数据竞争。
  - `LookupHostForGeo(ctx, host)`：IP 字面量短路 -> DoT -> 失败时按配置 fallback 系统 DNS
- `cmd/cmd.go` 在早期阶段（fast-trace / ws 初始化之前）注入 DoT 解析策略，避免早期分支绕过。
- `server/trace_handler.go` 通过 `ipgeo.GetSourceWithGeoDNS(...)` + `WithGeoDNSResolver(...)` 让 Web/API 请求也遵守 `dot_server`，包括 LeoMoe/FastIP 初始化阶段。
- Geo HTTP 请求统一走 `util.NewGeoHTTPClient(...)`（`util/http_client_geo.go`），其 Transport 现在从默认 Transport `Clone()` 而来，保留代理/HTTP2/连接池等标准行为。

## LeoMoe FastIP 与 MTR 首行

- `util/latency.go`：
  - `FastIPMetaCache` 缓存节点元数据（IP/Latency/NodeName）
  - `SuppressFastIPOutput` 可抑制彩色横幅
- `GetFastIP(...)` 的 DNS 阶段现在显式受 `timeout` 限制；`FastIPMetaCache` 也改为在 fallback/default IP 决定后再写入，避免缓存空 IP。
- MTR 模式在进入 TUI 前会设 `SuppressFastIPOutput=true`，避免污染主终端历史。
- MTR TUI/report 首行 `APIInfo` 由 `cmd/mtr_mode.go` 的 `buildAPIInfo(...)` 生成（仅 LeoMoeAPI 且有元数据时显示）。
- MTR raw 首行由 `buildRawAPIInfoLine(...)` 生成（格式略不同，包含延迟信息）。

## `--source` / `--dev` 现状

- `--dev` 在 `cmd/cmd.go` 先解析网卡并推导 `srcAddr`（已处理非 `*net.IPNet` 地址类型，避免 panic）。
- `trace.Config` 现在显式携带 `SourceDevice` / `DisableMPLS`，Darwin TCP/UDP 抓包与 MPLS 解析优先走会话级配置，不再依赖 Web 侧临时改写全局变量。
- `trace.Config` 也显式携带 `Context`；`TracerouteWithContext(...)` 通过把上游 ctx 传入各 tracer 的 `signal.NotifyContext(...)` 基底，让 TCP/UDP fallback MTR 可以响应取消。
- Windows TCP 目前仍无法把 `SourceDevice` 映射到 WinDivert 接口选择；当前策略是显式报错拒绝，而不是静默忽略该字段。
- MTR 标题显示源信息来自：
  - `--source`（最高优先）
  - `--dev` 推导
  - UDP dial fallback
- 相关函数：`cmd/mtr_mode.go -> resolveSrcIP(...)`

## CI 与工具链（当前）

- `go.mod`: `go 1.26.0`
- GitHub Actions：
  - `.github/workflows/build.yml` 使用 `setup-go@v6` + `go-version: 1.26.x`
  - `.github/workflows/test.yml` 使用 `setup-go@v6` + `go-version: 1.26.x`
  - test workflow 中 `GOTOOLCHAIN=go1.26.0+auto`
  - build matrix 已移除 `windows/arm`
- `.cross_compile.sh` 与 workflow 里的 `go build` 现在都用数组构造 `-tags` 参数，避免 shell word-splitting；脚本也会把当前 `GOARM` 传给 `compress_with_upx`，使 linux/armv7 目标能命中对应压缩分支。
- `ipgeo/ipdbone.go` 不再原地修改全局 `defaultClient.httpClient.Timeout`；超时覆盖会通过克隆 client（复用 token cache / token init，同步替换整个 HTTP client）实现，避免 dial timeout 与 client timeout 脱节。

## 关键文件导航

- CLI 主调度：`cmd/cmd.go`（~855 行）
- MTR 参数/流程：`cmd/mtr_mode.go`（~315 行）
- MTR 交互输入：`cmd/mtr_ui.go`
- MTR 引擎：`trace/mtr_runner.go`
- MTR 聚合：`trace/mtr_stats.go`
- MTR TUI：`printer/mtr_tui.go`（~691 行）
- MTR table/report：`printer/mtr_table.go`（~625 行）
- MTR TUI 颜色：`printer/mtr_tui_color.go`
- WS handler：`server/ws_handler.go`（~449 行）
- 前端：`server/web/assets/app.js`
- Geo DoT 解析：`util/dns_resolver.go`
- Geo HTTP 客户端：`util/http_client_geo.go`
- FastIP：`util/latency.go`

## 2026-03 Gocyclo 重构快照

- 第一波低风险重构已落地：
  - `ipgeo.Filter` 改为 CIDR 规则表驱动。
  - `util.DomainLookUp` 拆成 resolver / lookup / family filter / interactive select 四段。
  - `server.prepareTrace`、`normalizeTarget`、`trace.Traceroute`、`trace.Hop.fetchIPData` 已改成薄协调器。
  - `GetMTUByIPForDevice`、`GetICMPResponsePayload`、`parseIPDBOneResponse`、`GlobalpingFormatLocation` 已拆 helper。
- 第二波主流程/输出层已部分落地：
  - `cmd.Execute` 已拆成 parser 注册 helper、启动模式 helper、运行时调度 helper。
  - `fast_trace.FastTest` / `testFile` 已拆成交互选择、源地址推导、文件目标解析、单目标执行。
  - `server.mtrAggregator.Update`、`wshandle.messageSendHandler` 已改成薄入口。
  - `printer.RealtimePrinter`、`RealtimePrinterWithRouter`、`tracelog.RealtimePrinter` 现在共用 `internal/hoprender` 的 hop attempt 分组逻辑。
- 第三波协议层已开始落地：
  - `trace/internal/icmp_common.go`、`tcp_common.go`、`udp_common.go` 已改成“读包循环 + 共享解码 helper + 回调派发”结构。
  - 新增 `trace/internal/icmp_decode.go`，集中处理 ICMPv4/v6 解析、echo reply 匹配、内嵌目标 IP 校验、内嵌 ICMP seq 提取。
  - 新增 `trace/internal/icmp_decode_test.go`，覆盖 IPv4/IPv6 echo reply、error payload、目标 IP 校验、内嵌 seq 提取。
  - `trace/internal/udp_unix.go` 的 `SendUDP` 已拆成 IPv4/IPv6 独立发送 helper；`trace/udp_ipv4.go` 的 `send()` 也已拆成配额检查、构包、超时守护、发送记账四段。
  - Windows 协议层新增 `trace/internal/windivert_sniff_windows.go`，把 WinDivert sniff handle 打开、收包、ICMP/TCP 解码下沉为共享 helper；`icmp_windows.go` / `tcp_windows.go` / `udp_windows.go` 的 sniff 入口已变成薄协调器。
  - Darwin `trace/internal/icmp_darwin.go:ListenPacket` 已拆成 socket spec、接口绑定、bind sockaddr、finalize packet conn 四段；`trace/internal/tcp_darwin.go:ListenTCP` 也改成设备选择 + BPF + 共享 TCP reply 解码 helper。
  - 新增 `trace/internal/tcp_probe_decode.go` 与 `trace/internal/tcp_probe_decode_test.go`，集中处理 TCP probe reply 的 seq 还原、peer IP 提取与 IPv4/IPv6 解析，供 Darwin/Windows TCP sniff 共用。
- 当前已知剩余高复杂度主要集中在：
  - `printer/mtr_*` 渲染层
  - `cmd/mtr_ui.go` 输入状态机
  - `trace/globalping.go` 的主流程函数
  - `trace/mtr_runner.go` 中仍未拆薄的 ICMP round handler（`probeRound` / `onICMP`）
  - 少量收尾函数：`fast_trace ipv6.go`、`reporter/reporter.go`、`trace/mtr_raw.go`

## 2026-03 Gocyclo 重构快照（追加）

- MTR 核心热点已完成一轮收敛：
  - `trace/mtr_scheduler.go:runMTRScheduler` 已改成薄入口，核心状态与分支移动到 `trace/mtr_scheduler_runtime.go`。
  - `trace/mtr_stats.go:Update` / `MigrateStats` 已拆成按 hop 分组、累加器合并、裁剪 helper；新增 `trace/mtr_stats_helpers.go`。
  - `trace/mtr_runner.go:mtrLoop` 已改成薄入口，取消/重置/暂停/预览/backoff 分支移动到 `trace/mtr_loop_runtime.go`。
- MTR 输出层与输入层热点也已收敛：
  - `printer/mtr_tui.go:mtrTUIRenderWithWidth` 已拆成布局扫描、三行头部构建、host part 预构建、MPLS 续行渲染四段。
  - `printer/mtr_table.go` 的 host 组装和 `MTRReportPrint` 已改成共享 host-part 拼接 helper + report header/row helper。
  - `cmd/mtr_ui.go:(*mtrInputParser).Feed` 已拆成按状态分发的 parser helper。
- 最后一批业务流程热点也已拆薄：
  - `trace/globalping.go:GlobalpingTraceroute` 已拆成 client 构建、measurement 请求、结果解码、hop limit 推导、结果组装五段。
  - `trace/mtr_runner.go:(*mtrICMPEngine).onICMP` / `probeRound` 已拆成 reply 校验、notify 清理、目的地 TTL 识别、round 准备、发包 sweep、等待回包、结果构建多个 helper。
  - `fast_trace/fast_trace ipv6.go:FastTestv6`、`reporter/reporter.go:generateRouteReportNode`、`trace/mtr_raw.go:buildMTRRawRecordFromProbe` 也已分别拆成选择分发、route-node 属性构建、raw record metadata 填充 helper。
- 当前本地复杂度扫描结果：
  - `go run /tmp/checkcyclo.go .` 已无 `>15` 函数输出。
  - `go test ./...` 通过。

## 仍需记住的残余风险（非阻断）

- `closeWithCode` 中 `closed.Store(true)` 在 `closeOnce.Do` 外部，理论上有微小竞态窗口（实际无害，因 `sendMu` 保护；且无法简单移入 Once 内部，否则第二个调用者无法设置 closed）。
