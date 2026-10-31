# Go 1.27.1 JSON 运行时评估

> 历史测量记录。2026-11-01 起统一使用默认 JSON 后端，当前决策见
> [JSON 运行时 ADR](../adr/0001-go127-json-runtime.md)。

## 范围与结论

- 历史源码点：`2cab379924bdfdcfe0cb927156cceadd9b545ced`
- JSON benchmark 源码点：`164eeabc3a22ed2a5743a1b65c3dfd5bb8d4a28f`
- 最终测量源码点：`d00b254c9148f673737fdba46621d8a72bba9f84`
- 工具链：Go 1.27.1；历史产物使用 Go 1.26.8
- 正式发布结论：保留 `GOEXPERIMENT=nojsonv2`

两种 Go 1.27 JSON 后端生成的既有和新增 golden 完全一致。默认后端虽然显著改善
WebSocket 请求解码，但 WebSocket 响应编码显著回退，未满足任一关键 benchmark
不得显著回退超过 1% 的切换门槛。

## JSON 合同

| 出口 | 字节合同 |
| --- | --- |
| REST traceroute | 字段顺序、`omitempty`、数字、HTML/Unicode 转义；响应无尾换行 |
| WebSocket | start、MTR raw、path end、complete；每个 `WriteJSON` 消息带一个尾换行 |
| service | traceroute、MTR report、MTR raw；覆盖空字段、浮点数、转义和数组 |
| MCP | 实际 MCP session 返回的 traceroute、MTR report、MTR raw `structuredContent` |
| 既有合同 | MTR snapshot、CLI/报告和其它已有 golden 保持不变 |

相同 fixture 分别在 `GOEXPERIMENT=` 与 `GOEXPERIMENT=nojsonv2` 下运行并逐字节
通过。测试不依赖 `encoding/json` 的完整错误文本。

## 性能

| 项目 | 值 |
| --- | --- |
| 主机 | Apple M5，10 核，32 GB |
| 系统 | macOS 26.6.2 (25G83)，AC 供电 |
| 工具链 | Go 1.27.1 darwin/arm64 |
| 并发 | `GOMAXPROCS=10` |
| benchmark | 每种后端 10 次，每次 1 秒 |
| 统计工具 | `benchstat v0.0.0-20260825160852-19be9d8e6c70` |

`nojsonv2` 为发布基准，默认后端为比较侧：

| Benchmark | `nojsonv2` | 默认后端 | 默认后端变化 |
| --- | ---: | ---: | ---: |
| WebSocket marshal | 524.3 ns/op | 793.6 ns/op | +51.37%（p=0.000） |
| WebSocket unmarshal | 2.439 µs/op | 1.285 µs/op | -47.33%（p=0.000） |
| marshal bytes | 416 B/op | 896 B/op | +115.38% |
| marshal allocs | 2 allocs/op | 5 allocs/op | +150.00% |
| unmarshal bytes | 656 B/op | 368 B/op | -43.90% |
| unmarshal allocs | 15 allocs/op | 3 allocs/op | -80.00% |

差异在 10 次样本时已明确，不需要追加到 20 次。30 秒 CPU/heap profile 也确认：

- `nojsonv2` 的编码累计热点位于 `encoding/json.structEncoder.encode`，约 66.0%；
  heap 中 `encoding/json.Marshal` 占分配空间约 84.7%。
- 默认后端的编码累计热点位于 `encoding/json/v2.makeStructArshaler.func2`，约 45.5%；
  heap 主要新增于 `reflect.unsafe_New` 与 `bytes.Clone`，合计约 92.6%。

原始 benchmark 与 profile 保存在本地 `.cache/perf`，CI 应作为 artifact 保存，
不提交二进制或 pprof。

## Darwin 产物

两侧均使用 macOS 13.0 deployment target、
`-buildvcs=false -trimpath -ldflags '-s -w -buildid='`；Go 1.27.1 额外显式传
`-macos=13.0`。压缩统一使用 UPX 5.2.1 `--force-macos -9`。六个产物的
`LC_BUILD_VERSION` 均为 `minos 13.0`。

| Flavor | Go 1.26.8 未压缩 / UPX | Go 1.27.1 未压缩 / UPX | 未压缩变化 | UPX 变化 |
| --- | ---: | ---: | ---: | ---: |
| nexttrace | 30,772,658 / 10,256,400 B | 30,143,154 / 10,158,096 B | -2.05% | -0.96% |
| nexttrace-tiny | 11,182,434 / 4,259,856 B | 11,066,466 / 4,276,240 B | -1.04% | +0.38% |
| ntr | 11,182,434 / 4,259,856 B | 11,066,466 / 4,276,240 B | -1.04% | +0.38% |

## `--help` 启动与峰值 RSS

每个新构建产物无预热运行 10 次。表中“首次启动”是新产物的第一次执行，
“后续中位数”是其余 9 次的中位数；RSS 是 10 次
`/usr/bin/time -l` 的最大观测值。首次执行受 macOS 首次验证影响，只作记录。

| Flavor | Go 1.26.8 首次 / 后续中位数 | Go 1.27.1 首次 / 后续中位数 | Go 1.26.8 / 1.27.1 最大 RSS |
| --- | ---: | ---: | ---: |
| nexttrace | 653.337 / 18.526 ms | 552.365 / 19.312 ms | 30,162,944 / 30,359,552 B |
| nexttrace-tiny | 444.063 / 16.888 ms | 444.218 / 17.117 ms | 23,461,888 / 23,773,184 B |
| ntr | 447.042 / 16.915 ms | 451.242 / 17.116 ms | 23,314,432 / 23,363,584 B |

## 复现

```sh
GOEXPERIMENT= scripts/perf/run_json_benchmarks.sh default-json
GOEXPERIMENT=nojsonv2 scripts/perf/run_json_benchmarks.sh nojsonv2
go tool benchstat .cache/perf/nojsonv2.bench.txt .cache/perf/default-json.bench.txt

MACOSX_DEPLOYMENT_TARGET=13.0 \
GOEXPERIMENT=nojsonv2 \
PERF_LDFLAGS='-s -w -buildid= -macos=13.0' \
UPX_BIN=/opt/homebrew/bin/upx UPX_FLAGS='--force-macos -9' \
scripts/perf/measure_binaries.sh go1271-nojsonv2-darwin
```

启动/RSS 使用 `scripts/perf/measure_help_startup.sh`；profile 使用
`scripts/perf/profile_benchmark.sh`。最终 exact-head 仍须通过默认 JSON 与
`nojsonv2` 的全量测试及既有跨平台门禁。
