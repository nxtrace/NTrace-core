#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CALLER_DIR="${PWD}"
RESULT_NAME="${1:-$(git -C "${ROOT_DIR}" rev-parse --short HEAD)-pgo}"
RESULT_DIR="${PERF_RESULT_DIR:-${ROOT_DIR}/.cache/perf}"
PROFILE_COMPONENT_TIME="${PROFILE_COMPONENT_TIME:-10s}"
BENCH_GOMAXPROCS="${BENCH_GOMAXPROCS:-10}"
PGO_SEED=20260905
REQUIRED_GO_VERSION=go1.27.3

export GOTOOLCHAIN="${REQUIRED_GO_VERSION}"
export GOEXPERIMENT=

if [[ "${RESULT_DIR}" != /* ]]; then
  RESULT_DIR="${CALLER_DIR}/${RESULT_DIR}"
fi

mkdir -p "${RESULT_DIR}"
export GOMAXPROCS="${BENCH_GOMAXPROCS}"

actual_go_version="$(go env GOVERSION)"
if [[ "${actual_go_version}" != "${REQUIRED_GO_VERSION}" ]]; then
  echo "error: ${REQUIRED_GO_VERSION} required, got ${actual_go_version}" >&2
  exit 1
fi

if [[ -n "$(git -C "${ROOT_DIR}" status --porcelain)" ]]; then
  echo "error: commit the workload before generating its PGO profile" >&2
  exit 1
fi

profiles=()
sha256_file() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  echo "error: no SHA-256 utility found" >&2
  return 1
}

profile_component() {
  local package="$1"
  local benchmark="$2"
  local label="$3"
  local profile="${RESULT_DIR}/${RESULT_NAME}.${label}.cpu.pprof"
  local benchmark_list

  benchmark_list="$(
    cd "${ROOT_DIR}"
    go test -pgo=off -run '^$' -list "^${benchmark}$" "${package}"
  )"
  if ! grep -qx "${benchmark}" <<<"${benchmark_list}"; then
    echo "benchmark not found in ${package}: ${benchmark}" >&2
    exit 1
  fi

  (
    cd "${ROOT_DIR}"
    go test \
      -pgo=off \
      -run '^$' \
      -bench "^${benchmark}$" \
      -benchtime "${PROFILE_COMPONENT_TIME}" \
      -count 1 \
      -cpuprofile "${profile}" \
      "${package}"
  )
  profiles+=("${profile}")
}

profile_component ./trace/internal BenchmarkPGOProtocolDecodeWorkload protocol-decode
profile_component ./trace BenchmarkPGOTraceWorkload trace-mtr-geo-json
profile_component ./server BenchmarkPGOWebSocketJSONWorkload websocket-json

candidate="${RESULT_DIR}/${RESULT_NAME}.candidate.pgo"
go tool pprof -proto -output "${candidate}" "${profiles[@]}"

candidate_sha256="$(sha256_file "${candidate}")" || {
  echo "error: failed to calculate SHA-256 for ${candidate}" >&2
  exit 1
}

manifest="${RESULT_DIR}/${RESULT_NAME}.profile.txt"
{
  printf 'commit=%s\n' "$(git -C "${ROOT_DIR}" rev-parse HEAD)"
  printf 'go_version=%s\n' "$(go version)"
  printf 'goexperiment=%s\n' "$(go env GOEXPERIMENT)"
  printf 'gomaxprocs=%s\n' "${GOMAXPROCS}"
  printf 'seed=%s\n' "${PGO_SEED}"
  printf 'component_time=%s\n' "${PROFILE_COMPONENT_TIME}"
  printf 'components=3\n'
  printf 'profile_1=./trace/internal:BenchmarkPGOProtocolDecodeWorkload\n'
  printf 'profile_2=./trace:BenchmarkPGOTraceWorkload\n'
  printf 'profile_3=./server:BenchmarkPGOWebSocketJSONWorkload\n'
  printf 'candidate=%s\n' "${candidate}"
  printf 'candidate_sha256=%s\n' "${candidate_sha256}"
} >"${manifest}"

cat "${manifest}"
echo "Candidate PGO profile: ${candidate}"
