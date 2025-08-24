#!/usr/bin/env bash
set -euo pipefail

# -------- Config --------
DIST_PREFIX="nexttrace"
DEBUG_MODE="${1:-}"                # 支持 ./script.sh debug
TARGET_DIR="dist"
PLATFORMS="linux/386 linux/amd64 linux/arm64 linux/mips linux/mips64 linux/mipsle linux/mips64le windows/amd64 windows/arm64 openbsd/amd64 openbsd/arm64 freebsd/amd64 freebsd/arm64"

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

# -------- Prepare out dir --------
rm -rf -- "${TARGET_DIR}"
mkdir -p -- "${TARGET_DIR}"

# -------- Pure Go targets (CGO off) --------
for pl in ${PLATFORMS}; do
  export CGO_ENABLED=0
  export GOOS="$(echo "${pl}" | cut -d'/' -f1)"
  export GOARCH="$(echo "${pl}" | cut -d'/' -f2)"

  TARGET="${TARGET_DIR}/${DIST_PREFIX}_${GOOS}_${GOARCH}"
  if [[ "${GOOS}" == "windows" ]]; then
    TARGET="${TARGET}.exe"
  fi

  echo "build => ${TARGET}"
  go build "${GO_BUILD_FLAGS[@]}" -o "${TARGET}" -ldflags "${LD_BASE}"
done

# -------- linux/armv7（CGO off）--------
export CGO_ENABLED=0
export GOOS='linux'
export GOARCH='arm'
export GOARM='7'
TARGET="${TARGET_DIR}/${DIST_PREFIX}_${GOOS}_${GOARCH}v7"
echo "build => ${TARGET}"
go build "${GO_BUILD_FLAGS[@]}" -o "${TARGET}" -ldflags "${LD_BASE}"

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
