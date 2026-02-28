# NTrace-core 项目记忆文件（2026-02 快照，rev-2）

# 供 LLM 在后续会话中快速加载上下文，减少重复分析。

## 项目概览

- 名称：NextTrace (NTrace-core)
- 仓库：github.com/nxtrace/NTrace-core
- 模块：`github.com/nxtrace/NTrace-core`
- 语言：Go（`go 1.26.0`）
- 入口：`main.go -> cmd.Execute()`
- 核心能力：ICMP/TCP/UDP traceroute、GeoIP/RDNS、MTR 连续探测、Web/API、多平台构建

## 构建与测试（必须遵守）

- 由于 Darwin 下 `trace/internal/icmp_darwin.go` 使用 `//go:linkname`，构建/测试必须带：
  - `-ldflags "-checklinkname=0"`
- 常用命令：
  - 构建：`go build -ldflags "-checklinkname=0" ./...`
  - 测试：`go test -ldflags "-checklinkname=0" ./...`
- 交叉编译脚本：`.cross_compile.sh`（`LD_BASE` 已内置 `-checklinkname=0`）

## 当前 CLI 语义（重点）

### 常规 traceroute 路径

- `--table`：现在是"最终汇总表"模式（一次探测完成后输出汇总表），不再是旧的异步 table 刷新模式。
- `--route-path`：仍由 `reporter.New(...).Print()` 负责（与 MTR report 无关）。

### 间隔默认值（分层体系）

- `-z/--send-time`：每包间隔，默认 `defaultPacketIntervalMs = 50` ms。
- `-i/--ttl-time`：
  - **常规 traceroute**：TTL 分组间隔，默认 `defaultTracerouteTTLIntervalMs = 300` ms。
  - **MTR 模式**：`normalizeMTRTraceConfig()` 始终覆盖为 `defaultMTRInternalTTLIntervalMs = 50` ms（每轮内部）。
  - MTR 轮间隔由 `-i` 显式传值 或 默认 1000ms 决定（见下文 `-q/-i` 语义）。

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

### MTR 中 `-q/-i` 的新语义

- `-q/--queries`：
  - 在 MTR report 下表示"轮次"，默认 10（仅当用户未显式传 `-q`）。
  - 在 MTR TUI 下表示"最大轮次"，未显式传时默认无限运行。
- `-i/--ttl-time`：
  - 在 MTR 下表示"轮间隔毫秒"，默认 1000ms（仅当用户未显式传 `-i`）。
  - 每轮内部 TTL 分组间隔固定 50ms（`normalizeMTRTraceConfig` 覆盖）。

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
- 交互控制：`cmd/mtr_ui.go`
  - alternate screen + raw mode
  - 输入状态机 `mtrInputParser`（字节流，吞掉 CSI/SS3/OSC/鼠标/焦点等序列）
  - Enter/Leave 显式关闭输入扩展模式：1000/1002/1003/1006/1015/1004/2004
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
- `y`：切换 Host 显示模式（ASN/City/Owner/Full）
- `n`：切换 Host 基名显示（PTR-or-IP / IP-only）

## MTR 显示与统计规则（当前）

- Host 显示支持：
  - `HostModeASN` / `HostModeCity` / `HostModeOwner` / `HostModeFull`
  - `HostNamePTRorIP` / `HostNameIPOnly`
- 默认语言：`cn`（`--language en` 才优先英文字段）
- waiting 判定：`loss >= 99.95 && IP=="" && Host==""`
  - 显示为 `(waiting for reply)`
  - 指标列（Loss/Snt/Last/Avg/Best/Wrst/StDev）留空
- TUI Host 对齐（重要，已从 tab 改为手动空格）：
  - `buildTUIHostParts(stat, mode, nameMode, lang, showIPs)` 生成结构化 parts
  - `computeTUIASNWidth(stats, ...)` 扫描所有 hop 确定 ASN 列最大宽度
  - `formatTUIHost(parts, asnW)` 用 `padRight(asn, asnW)` + 空格拼接（不用 `\t`）
  - ASN 为空但 IP 已知时填 `"AS???"` 占位符，保证列对齐
  - waiting hop 不填占位符
- compact report host（非 wide report）：
  - `formatCompactReportHost(stat, nameMode, lang)` 仅输出 hostname/IP + ASN
- TUI 其他特性：
  - 终端宽度自适应 + CJK 宽度计算（go-runewidth）
  - 窄屏右锚定指标区
  - 动态 hop 前缀宽度（覆盖 3 位/4 位 TTL）
  - MPLS 独立续行显示

## MTR 引擎关键机制（易踩坑）

- 目的地提前停止：
  - `knownFinalTTL`（跨轮缓存）+ `roundFinalTTL`（本轮检测）用于缩短后续轮次 TTL 上界。
- seq 16 位回卷处理：
  - `seqWillWrap(...)` 触发 `rotateEngine(...)`
  - 轮换 echoID 并重建 listener，协议层隔离新旧回包。
- 额外安全网：
  - onICMP 中有 RTT 合理性检查（`<=0` 或 `>timeout` 丢弃）。
- 流式预览：
  - 仅已发送 TTL 才会参与预览；未发送 TTL 保持 nil 槽位，避免提前计入 Snt/Loss。

## Web Console / WebSocket（server/）

### WS 架构（`server/ws_handler.go`，~449 行）

- **异步写模型**：`wsTraceSession` 使用 `sendCh`（buffered channel，1024）+ `writeLoop` goroutine。
  - 调用方通过 `send(envelope)` 非阻塞投递；channel 满时返回 `errWSSlowConsumer`。
  - `writeLoop` 从 `sendCh` 取消息，`SetWriteDeadline` + `WriteJSON`。
- **关闭路径**：
  - `closeWithCode(code, reason)`：异常关闭（slow consumer / write error），关 `stopCh` + 发 close frame。
  - `finish()`：正常结束，`sendMu` 下关 `sendCh`，等 `writerDone`，再关 conn。
  - 两者均幂等（`closeOnce` / `finishOnce`）。
- **可测试性**：`wsConn` 接口 + `fakeWSConn` mock（`server/ws_handler_test.go`）。
- **常量**：`wsSendQueueSize=1024`，`wsWriteTimeout=5s`。

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
  - `LookupHostForGeo(ctx, host)`：IP 字面量短路 -> DoT -> 失败时按配置 fallback 系统 DNS
- `cmd/cmd.go` 在早期阶段（fast-trace / ws 初始化之前）注入 DoT 解析策略，避免早期分支绕过。
- Geo HTTP 请求统一走 `util.NewGeoHTTPClient(...)`（`util/http_client_geo.go`）。

## LeoMoe FastIP 与 MTR 首行

- `util/latency.go`：
  - `FastIPMetaCache` 缓存节点元数据（IP/Latency/NodeName）
  - `SuppressFastIPOutput` 可抑制彩色横幅
- MTR 模式在进入 TUI 前会设 `SuppressFastIPOutput=true`，避免污染主终端历史。
- MTR TUI/report 首行 `APIInfo` 由 `cmd/mtr_mode.go` 的 `buildAPIInfo(...)` 生成（仅 LeoMoeAPI 且有元数据时显示）。
- MTR raw 首行由 `buildRawAPIInfoLine(...)` 生成（格式略不同，包含延迟信息）。

## `--source` / `--dev` 现状

- `--dev` 在 `cmd/cmd.go` 先解析网卡并推导 `srcAddr`（已处理非 `*net.IPNet` 地址类型，避免 panic）。
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

## 仍需记住的残余风险（非阻断）

- `closeWithCode` 中 `closed.Store(true)` 在 `closeOnce.Do` 外部，理论上有微小竞态窗口（实际无害，因 `sendMu` 保护；且无法简单移入 Once 内部，否则第二个调用者无法设置 closed）。
