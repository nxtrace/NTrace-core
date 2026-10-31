# Go 1.27.1 性能与产物基线

> 历史测量记录。2026-11-01 起统一使用默认 JSON 后端，当前决策见
> [JSON 运行时 ADR](../adr/0001-go127-json-runtime.md)。

## 范围与结论

本基线比较仅改变工具链和发布约束，不包含运行时算法优化：

- 生产代码基线：`40ac803ca233b0bc3d4b3bf1cacc8f7e0d8368f2`
- Go 1.26.8 测量点：`2cab379`，只在生产基线上增加 benchmark、golden 和性能脚本
- Go 1.27.1 测量点：`b771e8d`
- 发布构建暂时使用 `GOEXPERIMENT=nojsonv2`
- 最低支持版本改为 macOS 13.0

Go 1.27.1 缩小了三种 flavor 的产物，并改善了部分解析路径；同时，现有
WebSocket JSON marshal、Geo cache 和 Geo HTTP client 路径出现了可重复的回退。
这些结果作为后续 PR-1、PR-2、PR-3、PR-4、PR-5 和 PR-8A 的优化基线，
不在工具链升级中混入算法调整。

## 测量环境

| 项目 | 值 |
| --- | --- |
| 主机 | Apple M5，10 核，32 GB 内存 |
| 系统 | macOS 26.6.2 (25G83)，AC 供电 |
| 工具链 | Go 1.26.8 与 Go 1.27.1，darwin/arm64 |
| 并发 | `GOMAXPROCS=10` |
| benchmark | 每组 10 次，每次 1 秒 |
| 热点复测 | 反向运行顺序，各追加 10 次；合并后 `n=20` |
| 统计工具 | `benchstat v0.0.0-20260825160852-19be9d8e6c70` |
| 压缩工具 | UPX 5.2.1，`-9` |

第一轮先运行 Go 1.26.8，再运行 Go 1.27.1。选取可能受持续负载影响的热点，
随后按 Go 1.27.1、Go 1.26.8 的反向顺序各补 10 次；下表的 `n=20` 项目使用
合并结果。`~` 表示 benchstat 未确认显著差异。

## Go 1.26.8 与 Go 1.27.1

两侧均使用 `GOEXPERIMENT=nojsonv2`。

| Benchmark | Go 1.26.8 | Go 1.27.1 | 变化 |
| --- | ---: | ---: | ---: |
| GeoFeed CSV parse | 1.190 ms | 1.149 ms | -3.44% (`n=10`) |
| GeoFeed IPv4 miss | 23.61 µs | 22.90 µs | -3.00% (`n=20`) |
| MTR Update, 64×4 | 83.13 µs | 82.92 µs | ~ (`n=10`) |
| MTR Clone, 64×4 | 37.12 µs | 37.28 µs | ~ (`n=10`) |
| MTR Snapshot, 64×4 | 27.75 µs | 27.26 µs | -1.75% (`n=10`) |
| MTR preview clone/update, 64×4 | 68.85 µs | 68.64 µs | ~ (`n=10`) |
| WebSocket JSON marshal | 470.9 ns | 532.4 ns | +13.05% (`n=20`) |
| WebSocket JSON unmarshal | 2.574 µs | 2.481 µs | -3.61% (`n=10`) |
| Geo cache hit | 6.706 ns | 7.444 ns | +11.01% (`n=20`) |
| Geo cache miss | 4.018 ns | 4.823 ns | +20.02% (`n=20`) |
| Geo HTTP client construct | 207.7 ns | 246.5 ns | +18.69% (`n=20`) |
| Geo HTTP sequential request | 30.65 µs | 33.06 µs | +7.85% (`n=20`) |
| Geo HTTP concurrent request | 43.40 µs | 48.13 µs | +10.90% (`n=20`) |
| Geo HTTP multi-client sequential | 28.79 µs | 31.86 µs | +10.67% (`n=20`) |
| Geo HTTP multi-client concurrent | 69.65 µs | 78.74 µs | +13.04% (`n=20`) |
| ICMPv4 decoder | 95.94 ns | 87.43 ns | -8.87% (`n=10`) |
| UDPv4 decoder | 92.80 ns | 107.50 ns | +15.85% (`n=20`) |
| MTU embedded UDP IPv4 | 10.70 ns | 10.37 ns | -3.09% (`n=10`) |
| MTU embedded UDP IPv6 ext | 14.66 ns | 13.24 ns | -9.72% (`n=10`) |
| nali span scan | 4.246 µs | 4.272 µs | +0.62% (`n=10`) |

Geo HTTP 构造由 6 增至 7 alloc/op，单次分配量增加 160 B；其余表中路径的
alloc/op 未因工具链改变。Geo cache 的相对回退较大，但绝对差值分别只有
0.738 ns/op 和 0.805 ns/op。UDP decoder 与 Geo HTTP 网络 benchmark 的离散度
高于纯 CPU benchmark；反向复测后方向未消失，但仍须在后续 PR 和不同平台复核。

## Go 1.27 JSON 后端预检

同一 Go 1.27.1 head 上直接比较默认 JSON 后端与 `nojsonv2`：

| Benchmark | `nojsonv2` | 默认后端 | 默认后端变化 |
| --- | ---: | ---: | ---: |
| WebSocket marshal | 494.9 ns, 416 B, 2 allocs | 730.8 ns, 896 B, 5 allocs | +47.66% |
| WebSocket unmarshal | 2.427 µs, 656 B, 15 allocs | 1.204 µs, 368 B, 3 allocs | -50.41% |

两种后端均通过现有 JSON golden。marshal 与 unmarshal 的方向相反，PR-0 不据此
切换正式发布后端；发布继续使用 `nojsonv2`，默认后端和 `nojsonv2` 测试 lane
同时保留，最终决策留给 PR-8A 的完整 schema 与 workload 证据。

## Profile

`BenchmarkMTRAggregatorSnapshot64TTL4Paths` 分别采集 30 秒 CPU 与 heap profile：

| 指标 | Go 1.26.8 | Go 1.27.1 |
| --- | ---: | ---: |
| 分配 | 106,544 B/op | 106,544 B/op |
| 分配次数 | 458 allocs/op | 458 allocs/op |

两版的应用热点结构一致。`snapshotLocked` 占分配空间约 90%，
`buildMTRHopStat` 约占 7.7%；CPU profile 中 `snapshotLocked` 的累计占比约
27%–28%。这两个位置是 PR-5 的主要优化目标。本地 Apple M5 原始 profile
不入库；性能工作流会另行生成并保存 Linux cross-reference profile artifact。

## 产物大小

产物使用相同参数：`-buildvcs=false -trimpath -ldflags '-s -w -buildid='`。

### Darwin arm64，未压缩

| Flavor | Go 1.26.8 | Go 1.27.1 | 变化 |
| --- | ---: | ---: | ---: |
| nexttrace | 30,756,194 B | 29,994,386 B | -2.48% |
| nexttrace-tiny | 11,182,482 B | 10,901,186 B | -2.52% |
| ntr | 11,182,482 B | 10,901,186 B | -2.52% |

### Linux arm64，未压缩 / UPX `-9`

| Flavor | Go 1.26.8 | Go 1.27.1 | 未压缩变化 | UPX 变化 |
| --- | ---: | ---: | ---: | ---: |
| nexttrace | 30,408,830 / 9,844,644 B | 29,622,396 / 9,682,520 B | -2.59% | -1.65% |
| nexttrace-tiny | 11,075,710 / 4,119,380 B | 10,813,564 / 4,063,688 B | -2.37% | -1.35% |
| ntr | 11,075,710 / 4,119,076 B | 10,813,564 / 4,063,672 B | -2.37% | -1.35% |

Darwin 三 flavor 的 `LC_BUILD_VERSION` 均验证为 `minos 13.0`。

## 验证与复现

本地已分别通过 Go 1.26.8 `nojsonv2`、Go 1.27.1 默认 JSON 和 Go 1.27.1
`nojsonv2` 的 `go test -count=1 ./...`。单次测试耗时不作性能结论，性能工作流
会为每个 exact head 重新保存原始时间。

性能工作流保存两侧原始 benchmark、benchstat、测试耗时、CPU/heap profile、
三 flavor stripped binary、大小和校验和。仓库内的 `scripts/perf` 可在相同环境
重现同一采集流程。
