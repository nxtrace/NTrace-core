#!/usr/bin/env bash

set -Eeuo pipefail

# -------- Config --------
DIST_PREFIX="nexttrace"
DEBUG_MODE="${1:-}"                # 支持 ./script.sh debug
TARGET_DIR="dist"
PLATFORMS="linux/386 linux/amd64 linux/arm64 linux/mips linux/mips64 linux/mipsle linux/mips64le windows/amd64 windows/arm64 openbsd/amd64 openbsd/arm64 freebsd/amd64 freebsd/arm64"
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
         -w -s -checklinkname=0"

GO_BUILD_FLAGS=(-trimpath)
if [[ "${DEBUG_MODE}" == "debug" ]]; then
  GO_BUILD_FLAGS=(-trimpath -gcflags "all=-N -l")
fi

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

# -------- Prepare out dir --------
rm -rf -- "${TARGET_DIR}"
mkdir -p -- "${TARGET_DIR}"

# -------- Pure Go targets (CGO off) --------
for pl in ${PLATFORMS}; do
  export CGO_ENABLED=0
  GOOS="${pl%%/*}"
  GOARCH="${pl#*/}"
  export GOOS GOARCH

  TARGET="${TARGET_DIR}/${DIST_PREFIX}_${GOOS}_${GOARCH}"
  if [[ "${GOOS}" == "windows" ]]; then
    TARGET="${TARGET}.exe"
  fi

  echo "build => ${TARGET}"
  go build "${GO_BUILD_FLAGS[@]}" -o "${TARGET}" -ldflags "${LD_BASE}"
  compress_with_upx "${TARGET}" "${GOOS}" "${GOARCH}"

  # Extra soft-float variants for linux/mips and linux/mipsle
  if [[ "${GOOS}" == "linux" && ( "${GOARCH}" == "mips" || "${GOARCH}" == "mipsle" ) ]]; then
    TARGET_SOFT="${TARGET_DIR}/${DIST_PREFIX}_${GOOS}_${GOARCH}_softfloat"
    echo "build => ${TARGET_SOFT} (GOMIPS=softfloat)"
    GOMIPS=softfloat go build "${GO_BUILD_FLAGS[@]}" -o "${TARGET_SOFT}" -ldflags "${LD_BASE}"
    compress_with_upx "${TARGET_SOFT}" "${GOOS}" "${GOARCH}"
  fi
done

# -------- linux/armv7（CGO off）--------
export CGO_ENABLED=0
export GOOS='linux'
export GOARCH='arm'
export GOARM='7'
TARGET="${TARGET_DIR}/${DIST_PREFIX}_${GOOS}_${GOARCH}v7"
echo "build => ${TARGET}"
go build "${GO_BUILD_FLAGS[@]}" -o "${TARGET}" -ldflags "${LD_BASE}"
compress_with_upx "${TARGET}" "${GOOS}" "${GOARCH}" "${GOARM}"

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

    TARGET="${TARGET_DIR}/${DIST_PREFIX}_${GOOS}_${GOARCH}"
    echo "build => ${TARGET}"
    go build "${GO_BUILD_FLAGS[@]}" -o "${TARGET}" -ldflags "${LD_BASE}"
    compress_with_upx "${TARGET}" "${GOOS}" "${GOARCH}"
  done

  # 合并 Universal 2（存在 lipo 才合并）
  if command -v lipo >/dev/null 2>&1; then
    if [[ -f "${TARGET_DIR}/${DIST_PREFIX}_darwin_amd64" && -f "${TARGET_DIR}/${DIST_PREFIX}_darwin_arm64" ]]; then
      lipo -create -output "${TARGET_DIR}/${DIST_PREFIX}_darwin_universal" \
        "${TARGET_DIR}/${DIST_PREFIX}_darwin_amd64" \
        "${TARGET_DIR}/${DIST_PREFIX}_darwin_arm64"
      echo "build => ${TARGET_DIR}/${DIST_PREFIX}_darwin_universal"
    else
      echo "warn: missing one of darwin slices; skip universal lipo." >&2
    fi
  else
    echo "warn: lipo not found; skip universal binary." >&2
  fi
fi
