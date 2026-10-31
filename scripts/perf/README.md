# Performance evidence

Run benchmarks on the same idle machine, with the same Go toolchain and power
settings. The scripts default to `GOMAXPROCS=10`, ten samples, one second per
sample, and store raw results below the ignored `.cache/perf` directory.

```sh
scripts/perf/run_benchmarks.sh base
scripts/perf/run_benchmarks.sh head
scripts/perf/compare_benchmarks.sh .cache/perf/base.bench.txt .cache/perf/head.bench.txt
scripts/perf/measure_binaries.sh head
```

Release builds, CI, and performance scripts use the default Go JSON runtime.

Use `run_json_benchmarks.sh` to compare the WebSocket marshal/unmarshal hot
paths without running unrelated benchmarks:

```sh
scripts/perf/run_json_benchmarks.sh base-json
scripts/perf/run_json_benchmarks.sh head-json
go tool benchstat .cache/perf/base-json.bench.txt .cache/perf/head-json.bench.txt
```

If a result crosses a merge threshold without a statistically clear result,
repeat both sides with `BENCH_COUNT=20`. Generate CPU and heap profiles for a
single representative benchmark with:

```sh
scripts/perf/profile_benchmark.sh ./trace BenchmarkMTRAggregatorSnapshot64TTL4Paths profile-name
```

Record `go version`, `go env GOEXPERIMENT`, the exact commit IDs, machine,
operating system, `GOMAXPROCS`, sample count, and any profiler or UPX version in
the pull request. Keep raw profiles and binaries as CI artifacts rather than
committing them.

`measure_binaries.sh` records stripped local-platform sizes. Set `UPX_BIN` to an
exact UPX executable when that target format is supported; use the same binary
and flags for both sides of a comparison. Set `PERF_LDFLAGS` when reproducing
release-specific linker metadata such as Darwin's `-macos=13.0`.

On macOS, `measure_help_startup.sh` records ten no-warmup `--help` process
launches and the maximum resident set size reported by `/usr/bin/time -l`:

```sh
scripts/perf/measure_help_startup.sh go127 \
  nexttrace /path/to/nexttrace \
  nexttrace-tiny /path/to/nexttrace-tiny \
  ntr /path/to/ntr
```

## Default PGO evaluation

Generate a 30-second candidate profile from one deterministic composite
workload with three equally timed phases. It covers the protocol decoders,
fake-prober MTR aggregation with direct and cached fake Geo provider paths and
JSON output, and WebSocket JSON encode/decode:

```sh
scripts/perf/generate_pgo_profile.sh candidate
```

Compare `-pgo=off` with the candidate using ten one-second samples. The
`workload` files contain only the three equally weighted profile workloads; the
`guardrail` files contain the individual critical paths:

```sh
scripts/perf/run_pgo_benchmarks.sh pgo-off off
scripts/perf/run_pgo_benchmarks.sh pgo-on .cache/perf/candidate.candidate.pgo
go tool benchstat .cache/perf/pgo-off.workload.bench.txt .cache/perf/pgo-on.workload.bench.txt
go tool benchstat .cache/perf/pgo-off.guardrail.bench.txt .cache/perf/pgo-on.guardrail.bench.txt
```

The scripts use the default Go JSON runtime, `GOMAXPROCS=10`, and seed
`20260905` by default. PGO scripts clear inherited `GOEXPERIMENT` settings so
profile generation and evaluation use the same default runtime.
If a threshold is crossed without statistical confidence,
rerun both sides with `BENCH_COUNT=20`.

Set `PERF_BINARY_DIR` to keep the exact binaries built by
`measure_binaries.sh` for startup and RSS comparison:

```sh
GOFLAGS=-pgo=off \
  PERF_BINARY_DIR=.cache/perf/pgo-off-bin scripts/perf/measure_binaries.sh pgo-off
GOFLAGS=-pgo=$PWD/.cache/perf/candidate.candidate.pgo \
  PERF_BINARY_DIR=.cache/perf/pgo-on-bin scripts/perf/measure_binaries.sh pgo-on
```
