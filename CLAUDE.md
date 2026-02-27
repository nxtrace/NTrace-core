# NTrace-core 项目记忆文件（2026-02 快照）
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
- `--table`：现在是“最终汇总表”模式（一次探测完成后输出汇总表），不再是旧的异步 table 刷新模式。
- `--route-path`：仍由 `reporter.New(...).Print()` 负责（与 MTR report 无关）。

### MTR 相关参数
- `-t/--mtr`：开启 MTR 交互模式（TTY 全屏 TUI）。
- `-r/--report`：MTR 报告模式（非交互），隐式开启 MTR。
- `-w/--wide`：宽报告模式，隐式等价 `--mtr --report --wide`。
- 有效 MTR 开关：`effectiveMTR = mtr || report || wide`。
- MTR 冲突参数（会直接报错退出）：`--table` `--raw` `--classic` `--json` `--output` `--route-path` `--from` `--fast-trace` `--file` `--deploy`。

### MTR 中 `-q/-i` 的新语义
- `-q/--queries`：
  - 在 MTR report 下表示“轮次”，默认 10（仅当用户未显式传 `-q`）。
  - 在 MTR TUI 下表示“最大轮次”，未显式传时默认无限运行。
- `-i/--ttl-time`：
  - 在 MTR 下表示“轮间隔毫秒”，默认 1000ms（仅当用户未显式传 `-i`）。

## MTR 运行链路（重要文件）
- 入口与调度：`cmd/mtr_mode.go`
  - `runMTRTUI(...)`
  - `runMTRReport(...)`
- 交互控制：`cmd/mtr_ui.go`
  - alternate screen + raw mode
  - 输入状态机解析（吞掉 CSI/SS3/OSC/鼠标/焦点等序列）
  - Enter/Leave 显式关闭输入扩展模式：1000/1002/1003/1006/1015/1004/2004
- 核心探测循环：`trace/mtr_runner.go`
  - `RunMTR` / `mtrLoop`
  - 支持暂停、重置、流式预览（`ProgressThrottle` 默认 200ms）
  - ICMP 持久引擎 + TCP/UDP fallback
- 统计聚合：`trace/mtr_stats.go`
  - `MTRAggregator` / `MTRHopStat`
  - unknown 合并策略：单路径时把 unknown 合并到唯一已知路径，避免同 TTL 分裂成 waiting + 真实 IP 两行
- 输出层：
  - TUI：`printer/mtr_tui.go`
  - table/report：`printer/mtr_table.go`

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
- TUI 支持：
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

## DoT 与 Geo DNS（近期大改）
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
- MTR 首行 `APIInfo` 由 `cmd/mtr_mode.go` 的 `buildAPIInfo(...)` 生成（仅 LeoMoeAPI 且有元数据时显示）。

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
- CLI 主调度：`cmd/cmd.go`
- MTR 参数/流程：`cmd/mtr_mode.go`
- MTR 交互输入：`cmd/mtr_ui.go`
- MTR 引擎：`trace/mtr_runner.go`
- MTR 聚合：`trace/mtr_stats.go`
- MTR TUI：`printer/mtr_tui.go`
- MTR table/report：`printer/mtr_table.go`
- Geo DoT 解析：`util/dns_resolver.go`
- Geo HTTP 客户端：`util/http_client_geo.go`
- FastIP：`util/latency.go`

## 仍需记住的残余风险（非阻断）
- `MTRReportPrint` 的 waiting 指标留空目前主要通过代码路径覆盖，
  还没有单独一条“report 渲染输出断言”来专门锁死该行为。
