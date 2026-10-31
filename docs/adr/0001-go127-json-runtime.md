# ADR 0001：Go 1.27 JSON 运行时

- 状态：接受
- 日期：2026-11-01（修订 2026-09-05 决策）
- 工具链：Go 1.27.3
- Darwin 最低版本：macOS 13.0

## 背景

Go 1.27 的 `encoding/json` v1 API 默认由新的 v2 实现支撑，并允许通过
`GOEXPERIMENT=nojsonv2` 暂时恢复旧实现。Go 官方说明现有 v1 API 继续受支持，
但迁移时不应依赖完整错误文本；直接使用 `encoding/json/v2` 还涉及额外行为差异。

- [Go 1.27 Release Notes](https://go.dev/doc/go1.27)
- [Go 1.27.1 Release History](https://go.dev/doc/devel/release)
- [JSON v1 to v2 Migration Guide](https://go.dev/doc/jsonv2-migration)

NextTrace 的 REST、WebSocket、MTR、service 与 MCP 输出属于公开合同，继续由
golden 验证字节和语义。JSON 后端的性能取舍结合实际调用量、总耗时和服务并发评估，
不再以单项微基准显著回退超过 1% 作为切换否决条件。

## 决策

1. 项目固定使用 Go 1.27.3，Darwin 正式产物最低支持 macOS 13.0。
2. 源码继续使用 `encoding/json` v1 API，不直接导入 `encoding/json/v2`。
3. 正式发布、`.cross_compile.sh`、性能和 PGO 脚本统一使用 Go 默认 JSON 后端，
   移除 `nojsonv2` 固定项和 PGO 专用 JSON 后端配置。
4. CI 统一验证默认 JSON 后端，移除旧实现专用测试 lane。
5. Golden 锁定字段名、顺序、`omitempty`、`null`、数字、转义和尾换行；不锁定
   `encoding/json` 的完整错误文本。

## 证据

默认后端和 `nojsonv2` 在相同 fixture 上逐字节通过 REST、WebSocket、
trace/MTR service 响应和 MCP `structuredContent` golden。

2026-10-31 在 Apple M5、Go 1.27.3、`GOMAXPROCS=10` 上复测。使用现有
`server/ws_json_benchmark_test.go`，源码基于 `642852f` 加工具链版本更新；
预编译两侧测试程序后按旧、默认、默认、旧的顺序串行运行，每段 5 次、每次 1 秒，
每侧合计 10 次。`benchstat` 结果：

| 路径 | `nojsonv2` | 默认后端 | 默认后端变化 |
| --- | ---: | ---: | ---: |
| WebSocket marshal | 490.8 ns/op，416 B，2 allocs | 742.5 ns/op，896 B，5 allocs | +51.28%，p<0.001 |
| WebSocket unmarshal | 2.459 µs/op，656 B，15 allocs | 1.224 µs/op，368 B，3 allocs | -50.22%，p<0.001 |
| 一次编码 + 一次解码 | 3.083 µs/op | 2.019 µs/op | -34.50%，p<0.001 |

编码样本的绝对差约为 0.252 µs/op，不能将该微基准的百分比直接解释为 CLI
总耗时变化。混合负载也不代表实际 Web/MTR 消息比例；本次没有测量端到端
CLI 耗时或服务并发吞吐。接受已知编码和分配开销，统一使用 Go 默认实现以减少
构建配置与测试分支的维护成本。

原始复测数据保存在本地 `.cache/perf/go1273-json-20261031/`。Go 1.27.1 的产物、
启动、RSS 和 profile 历史证据见
[Go 1.27.1 JSON 运行时评估](../performance/go127-json-runtime.md)。

## 影响与回退

- 开发、发布与 CI 使用相同 JSON 后端，既有 golden 持续验证用户可见合同。
- Web/MTR 高并发场景仍可能受编码和分配开销影响，需要结合实际负载复测。
- 出现可复现的合同或端到端性能回归时，按具体影响处理；必要时可临时显式使用
  `GOEXPERIMENT=nojsonv2` 回退，不长期维护第二套发布配置和测试 lane。
