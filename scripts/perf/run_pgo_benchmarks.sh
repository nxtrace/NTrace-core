#!/usr/bin/env bash

set -Eeuo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: $0 RESULT_NAME PGO_PROFILE|off" >&2
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CALLER_DIR="${PWD}"
RESULT_NAME="$1"
PGO_PROFILE="$2"
RESULT_DIR="${PERF_RESULT_DIR:-${ROOT_DIR}/.cache/perf}"
BENCH_COUNT="${BENCH_COUNT:-10}"
BENCH_TIME="${BENCH_TIME:-1s}"
BENCH_GOMAXPROCS="${BENCH_GOMAXPROCS:-10}"
REQUIRED_GO_VERSION=go1.27.3

export GOTOOLCHAIN="${REQUIRED_GO_VERSION}"
export GOEXPERIMENT=

if [[ "${RESULT_DIR}" != /* ]]; then
  RESULT_DIR="${CALLER_DIR}/${RESULT_DIR}"
fi
if [[ "${PGO_PROFILE}" != "off" && "${PGO_PROFILE}" != /* ]]; then
  PGO_PROFILE="${CALLER_DIR}/${PGO_PROFILE}"
fi
if [[ "${PGO_PROFILE}" != "off" && ! -f "${PGO_PROFILE}" ]]; then
  echo "PGO profile not found: ${PGO_PROFILE}" >&2
  exit 1
fi

mkdir -p "${RESULT_DIR}"
export GOMAXPROCS="${BENCH_GOMAXPROCS}"

actual_go_version="$(go env GOVERSION)"
if [[ "${actual_go_version}" != "${REQUIRED_GO_VERSION}" ]]; then
  echo "error: ${REQUIRED_GO_VERSION} required, got ${actual_go_version}" >&2
  exit 1
fi

run_group() {
  local group="$1"
  local benchmark_pattern="$2"
  shift 2
  local output="${RESULT_DIR}/${RESULT_NAME}.${group}.bench.txt"

  (
    cd "${ROOT_DIR}"
    go version
    echo "GOEXPERIMENT=$(go env GOEXPERIMENT)"
    echo "GOMAXPROCS=${GOMAXPROCS}"
    echo "PGO_PROFILE=${PGO_PROFILE}"
    go test \
      -pgo="${PGO_PROFILE}" \
      -run '^$' \
      -bench "${benchmark_pattern}" \
      -benchmem \
      -benchtime "${BENCH_TIME}" \
      -count "${BENCH_COUNT}" \
      "$@"
  ) | tee "${output}"
  echo "${group} benchmark results: ${output}"
}

run_group workload \
  '^(BenchmarkPGOProtocolDecodeWorkload|BenchmarkPGOTraceWorkload|BenchmarkPGOWebSocketJSONWorkload)$' \
  ./trace/internal ./trace ./server

run_group guardrail \
  '^(BenchmarkDecodeICMPSocketMessage|BenchmarkDecodeTCPProbePacket|BenchmarkDecodeUDPSocketMessage|BenchmarkMTRAggregatorUpdate64TTL4Paths|BenchmarkMTRAggregatorSnapshot64TTL4Paths|BenchmarkMTRAggregatorPreviewCloneUpdate64TTL4Paths|BenchmarkGeoCacheAccess|BenchmarkWebSocketJSONMarshalEnvelope|BenchmarkWebSocketJSONUnmarshalRequest)$' \
  ./trace/internal ./trace ./server
