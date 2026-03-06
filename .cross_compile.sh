#!/usr/bin/env bash

set -Eeuo pipefail

# -------- Config --------
# Usage: .cross_compile.sh [full|tiny|ntr|all] [debug]
#   full  — build nexttrace (Full, includes WebUI + Globalping + MTR)
#   tiny  — build nexttrace-tiny (no WebUI, no Globalping, no MTR)
#   ntr   — build ntr (MTR-only, default MTR mode)
#   all   — build all three flavors (default)
#   debug — enable debug symbols (can combine: .cross_compile.sh all debug)

FLAVOR_ARG="${1:-all}"
DEBUG_MODE="${2:-}"

# Allow "debug" as first arg for backward compat
if [[ "${FLAVOR_ARG}" == "debug" ]]; then
  FLAVOR_ARG="all"
  DEBUG_MODE="debug"
fi

# Define flavor specs: "bin_name:build_tags"
declare -a FLAVOR_SPECS
case "${FLAVOR_ARG}" in
  full) FLAVOR_SPECS=("nexttrace:") ;;
  tiny) FLAVOR_SPECS=("nexttrace-tiny:flavor_tiny") ;;
  ntr)  FLAVOR_SPECS=("ntr:flavor_ntr") ;;
  all)  FLAVOR_SPECS=("nexttrace:" "nexttrace-tiny:flavor_tiny" "ntr:flavor_ntr") ;;
  *)
    echo "Usage: $0 [full|tiny|ntr|all] [debug]" >&2
    exit 1
    ;;
esac

TARGET_DIR="dist"
PLATFORMS="linux/386 linux/amd64 linux/arm64 linux/mips linux/mips64 linux/mipsle linux/mips64le linux/loong64 windows/amd64 windows/arm64 openbsd/amd64 openbsd/arm64 freebsd/amd64 freebsd/arm64"
UPX_BIN="${UPX_BIN:-$(command -v upx 2>/dev/null || true)}"
UPX_FLAGS="${UPX_FLAGS:--9}"

# -------- Build metadata (robust) --------
BUILD_VERSION="$(git describe --tags --always 2>/dev/null || true)"
BUILD_VERSION="${BUILD_VERSION:-dev}"
BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
COMMIT_SHA1="$(git rev-parse --short HEAD 2>/dev/null || true)"
COMMIT_SHA1="${COMMIT_SHA1:-unknown}"

# 通用 ldflags（去掉了内部单引号）
LD_BASE="-X github.com/nxtrace/NTrace-core/config.Version=${BUILD_VERSION} \
         -X github.com/nxtrace/NTrace-core/config.BuildDate=${BUILD_DATE} \
         -X github.com/nxtrace/NTrace-core/config.CommitID=${COMMIT_SHA1} \
         -w -s"

GO_BUILD_FLAGS=(-trimpath)
if [[ "${DEBUG_MODE}" == "debug" ]]; then
  GO_BUILD_FLAGS=(-trimpath -gcflags "all=-N -l")
fi

# build_one BIN TAGS GOOS GOARCH [EXTRA_ENV...]
build_one() {
  local bin="$1" tags="$2" goos="$3" goarch="$4"
  shift 4
  local target="${TARGET_DIR}/${bin}_${goos}_${goarch}"
  local target_arm=""
  # Apply extra env vars (e.g. GOARM=7 suffix)
  for ev in "$@"; do
    local key="${ev%%=*}" val="${ev#*=}"
    if [[ "${key}" == "GOARM" ]]; then
      target_arm="${val}"
      target="${target}v${val}"
    elif [[ "${key}" == "GOMIPS" && "${val}" == "softfloat" ]]; then
      target="${target}_softfloat"
    fi
  done
  if [[ "${goos}" == "windows" ]]; then
    target="${target}.exe"
  fi

  local tags_flag=()
  if [[ -n "${tags}" ]]; then
    tags_flag=(-tags "${tags}")
  fi

  echo "build => ${target}  (tags: ${tags:-none})"
  env "$@" go build "${GO_BUILD_FLAGS[@]}" "${tags_flag[@]}" -o "${target}" -ldflags "${LD_BASE}"
  compress_with_upx "${target}" "${goos}" "${goarch}" "${target_arm}" "quiet"
}

compress_with_upx() {
  local binary="${1:-}"
  local target_os="${2:-}"
  local target_arch="${3:-}"
  local target_arm="${4:-}"
  local note="${5:-}"
  if [[ "${target_os}" != "linux" ]]; then
    return
  fi
  case "${target_arch}" in
    386|amd64|arm64)
      ;;
    arm)
      if [[ "${target_arm}" != "7" ]]; then
        return
      fi
      ;;
    *)
      return
      ;;
  esac
  if [[ -z "${UPX_BIN}" ]]; then
    return
  fi
  if [[ ! -f "${binary}" ]]; then
    return
  fi
  if [[ "${note}" != "quiet" ]]; then
    echo "upx => ${binary}"
  fi
  if ! "${UPX_BIN}" ${UPX_FLAGS} "${binary}" >/dev/null; then
    echo "warn: upx failed for ${binary}, keeping uncompressed" >&2
  fi
}

if [[ -z "${UPX_BIN}" ]]; then
  echo "info: upx not found; set UPX_BIN or install upx to enable binary compression" >&2
else
  echo "info: using upx at ${UPX_BIN} with flags ${UPX_FLAGS}" >&2
fi

echo "info: building flavor(s): ${FLAVOR_ARG}" >&2

# -------- Prepare out dir --------
rm -rf -- "${TARGET_DIR}"
mkdir -p -- "${TARGET_DIR}"

# -------- Pure Go targets (CGO off) --------
for pl in ${PLATFORMS}; do
  export CGO_ENABLED=0
  GOOS="${pl%%/*}"
  GOARCH="${pl#*/}"
  export GOOS GOARCH

  for SPEC in "${FLAVOR_SPECS[@]}"; do
    BIN="${SPEC%%:*}"
    TAGS="${SPEC#*:}"
    build_one "${BIN}" "${TAGS}" "${GOOS}" "${GOARCH}"
  done

  # Extra soft-float variants for linux/mips and linux/mipsle
  if [[ "${GOOS}" == "linux" && ( "${GOARCH}" == "mips" || "${GOARCH}" == "mipsle" ) ]]; then
    for SPEC in "${FLAVOR_SPECS[@]}"; do
      BIN="${SPEC%%:*}"
      TAGS="${SPEC#*:}"
      build_one "${BIN}" "${TAGS}" "${GOOS}" "${GOARCH}" "GOMIPS=softfloat"
    done
  fi
done

# -------- linux/armv7（CGO off）--------
export CGO_ENABLED=0
export GOOS='linux'
export GOARCH='arm'
export GOARM='7'
for SPEC in "${FLAVOR_SPECS[@]}"; do
  BIN="${SPEC%%:*}"
  TAGS="${SPEC#*:}"
  build_one "${BIN}" "${TAGS}" "${GOOS}" "${GOARCH}" "GOARM=7"
done

# -------- Darwin targets with CGO + SDK libpcap --------
if [[ "$(uname)" == "Darwin" ]]; then
  if ! command -v xcrun >/dev/null 2>&1; then
    echo "error: xcrun not found. Please install Xcode Command Line Tools: xcode-select --install" >&2
    exit 1
  fi
  SDKROOT="$(xcrun --sdk macosx --show-sdk-path)"

  for GOARCH in amd64 arm64; do
    export CGO_ENABLED=1
    export GOOS=darwin
    export CC=clang
    export CXX=clang++

    if [[ "${GOARCH}" == "amd64" ]]; then
      ARCH_FLAG="-arch x86_64"
    else
      ARCH_FLAG="-arch arm64"
    fi

    # 仅提供 SDK/架构/最低系统版本；-lpcap 交由源码中的 #cgo LDFLAGS 处理，避免重复
    export CGO_CFLAGS="-isysroot ${SDKROOT} ${ARCH_FLAG} -mmacosx-version-min=11.0"
    export CGO_LDFLAGS="-isysroot ${SDKROOT} ${ARCH_FLAG} -mmacosx-version-min=11.0"

    for SPEC in "${FLAVOR_SPECS[@]}"; do
      BIN="${SPEC%%:*}"
      TAGS="${SPEC#*:}"
      build_one "${BIN}" "${TAGS}" "${GOOS}" "${GOARCH}"
    done
  done

  # 合并 Universal 2（存在 lipo 才合并）
  if command -v lipo >/dev/null 2>&1; then
    for SPEC in "${FLAVOR_SPECS[@]}"; do
      BIN="${SPEC%%:*}"
      if [[ -f "${TARGET_DIR}/${BIN}_darwin_amd64" && -f "${TARGET_DIR}/${BIN}_darwin_arm64" ]]; then
        lipo -create -output "${TARGET_DIR}/${BIN}_darwin_universal" \
          "${TARGET_DIR}/${BIN}_darwin_amd64" \
          "${TARGET_DIR}/${BIN}_darwin_arm64"
        echo "build => ${TARGET_DIR}/${BIN}_darwin_universal"
      else
        echo "warn: missing one of darwin slices for ${BIN}; skip universal lipo." >&2
      fi
    done
  else
    echo "warn: lipo not found; skip universal binary." >&2
  fi
fi
