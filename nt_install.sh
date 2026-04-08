#!/bin/sh

# This file is also served at https://nxtrace.org/nt .
# Keep README one-key install snippets in sync with this script.

set -eu

API_HOST="www.nxtrace.org"
API_PATH="/api/dist/core"
# Fallback deliberately targets the stable NTrace-core release channel.
GITHUB_RELEASE_BASE="https://github.com/nxtrace/NTrace-core/releases/latest/download"
DEFAULT_PROTOCOL="https"
DEFAULT_FLAVOR="full"

SYSTEM_BIN_DIR_DEFAULT="/usr/local/bin"
USER_BIN_DIR_DEFAULT="${HOME:-}/.local/bin"

PROTOCOL="${DEFAULT_PROTOCOL}"
FLAVOR="${DEFAULT_FLAVOR}"
INSTALL_MODE="auto"
BIN_DIR=""
ARCH_OVERRIDE=""
USE_SOFTFLOAT=0
DOWNLOADER=""
BIN_NAME=""
TARGET_OS=""
TARGET_ARCH=""
ASSET_NAME=""
INSTALL_PATH=""
TEMP_FILE=""

SYSTEM_BIN_DIR="${NT_INSTALL_SYSTEM_BIN_DIR:-${SYSTEM_BIN_DIR_DEFAULT}}"
USER_BIN_DIR="${NT_INSTALL_USER_BIN_DIR:-${USER_BIN_DIR_DEFAULT}}"

info() {
    printf '%s\n' "==> $*"
}

warn() {
    printf '%s\n' "warning: $*" >&2
}

die() {
    printf '%s\n' "error: $*" >&2
    exit 1
}

cleanup() {
    if [ -n "${TEMP_FILE}" ] && [ -f "${TEMP_FILE}" ]; then
        rm -f "${TEMP_FILE}" >/dev/null 2>&1 || true
    fi
}

trap cleanup EXIT INT TERM

print_help() {
    cat <<'EOF'
NextTrace one-key installer

Usage:
  curl -sL https://nxtrace.org/nt | bash
  curl -sL https://nxtrace.org/nt | bash -s -- --flavor tiny
  curl -sL https://nxtrace.org/nt | bash -s -- --flavor ntr

Options:
  --flavor full|tiny|ntr  Install the selected release flavor
  --user                  Install to ~/.local/bin
  --system                Install to /usr/local/bin
  --bin-dir <path>        Install to a specific directory
  --arch <arch>           Override detected release arch
  --softfloat             Use _softfloat asset for linux mips/mipsle
  --help                  Show this help message

Compatibility:
  http                    Legacy compatibility flag. Use http for the API endpoint.
EOF
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

normalize_arch() {
    case "$1" in
        amd64|x86_64) printf '%s\n' "amd64" ;;
        386|i386|i486|i586|i686) printf '%s\n' "386" ;;
        arm64|aarch64) printf '%s\n' "arm64" ;;
        armv5|armv5l|armv5tel) printf '%s\n' "armv5" ;;
        armv6|armv6l) printf '%s\n' "armv6" ;;
        armv7|armv7l|armv7ml|armv8l) printf '%s\n' "armv7" ;;
        mips) printf '%s\n' "mips" ;;
        mipsel|mipsle) printf '%s\n' "mipsle" ;;
        mips64) printf '%s\n' "mips64" ;;
        mips64el|mips64le) printf '%s\n' "mips64le" ;;
        loongarch64|loong64) printf '%s\n' "loong64" ;;
        ppc64) printf '%s\n' "ppc64" ;;
        ppc64le) printf '%s\n' "ppc64le" ;;
        riscv64) printf '%s\n' "riscv64" ;;
        s390x) printf '%s\n' "s390x" ;;
        *)
            return 1
            ;;
    esac
}

detect_os() {
    case "$(uname -s 2>/dev/null || printf 'unknown')" in
        Linux) printf '%s\n' "linux" ;;
        Darwin) printf '%s\n' "darwin" ;;
        FreeBSD) printf '%s\n' "freebsd" ;;
        OpenBSD) printf '%s\n' "openbsd" ;;
        DragonFly) printf '%s\n' "dragonfly" ;;
        *)
            return 1
            ;;
    esac
}

detect_arch() {
    if [ -n "${ARCH_OVERRIDE}" ]; then
        normalize_arch "${ARCH_OVERRIDE}" || die "unsupported arch override: ${ARCH_OVERRIDE}. Use one of the release arch names."
        return 0
    fi
    normalize_arch "$(uname -m 2>/dev/null || printf 'unknown')" || die "unsupported architecture: $(uname -m 2>/dev/null || printf 'unknown'). Use --arch or download the release manually."
}

resolve_bin_name() {
    case "${FLAVOR}" in
        full) printf '%s\n' "nexttrace" ;;
        tiny) printf '%s\n' "nexttrace-tiny" ;;
        ntr) printf '%s\n' "ntr" ;;
        *)
            die "unsupported flavor: ${FLAVOR}"
            ;;
    esac
}

resolve_asset_name() {
    if [ "${TARGET_OS}" = "darwin" ]; then
        printf '%s\n' "${BIN_NAME}_darwin_universal"
        return 0
    fi

    asset="${BIN_NAME}_${TARGET_OS}_${TARGET_ARCH}"
    if [ "${USE_SOFTFLOAT}" -eq 1 ]; then
        case "${TARGET_OS}:${TARGET_ARCH}" in
            linux:mips|linux:mipsle)
                asset="${asset}_softfloat"
                ;;
            *)
                die "--softfloat is only supported for linux mips/mipsle."
                ;;
        esac
    fi
    printf '%s\n' "${asset}"
}

pick_downloader() {
    if command_exists curl; then
        DOWNLOADER="curl"
        return 0
    fi
    if command_exists wget; then
        DOWNLOADER="wget"
        return 0
    fi
    die "curl or wget is required to download NextTrace."
}

fetch_text() {
    url="$1"
    if [ "${DOWNLOADER}" = "curl" ]; then
        curl -fsSL --connect-timeout 10 --max-time 30 "${url}"
        return $?
    fi
    wget -q -O - --timeout=10 "${url}"
}

download_file() {
    url="$1"
    output="$2"
    if [ "${DOWNLOADER}" = "curl" ]; then
        curl -fsSL --connect-timeout 10 --max-time 180 --retry 3 --retry-delay 1 -o "${output}" "${url}"
        return $?
    fi
    wget -q -O "${output}" --timeout=10 --tries=3 --waitretry=1 "${url}"
}

mkdir_or_die() {
    dir="$1"
    label="$2"
    if [ -d "${dir}" ]; then
        return 0
    fi
    if ! mkdir -p "${dir}" >/dev/null 2>&1; then
        die "unable to create ${label}: ${dir}"
    fi
}

ensure_writable_dir() {
    dir="$1"
    label="$2"
    mkdir_or_die "${dir}" "${label}"
    [ -w "${dir}" ] || die "${label} is not writable: ${dir}"
}

can_use_dir() {
    dir="$1"
    if [ ! -d "${dir}" ]; then
        mkdir -p "${dir}" >/dev/null 2>&1 || return 1
    fi
    [ -w "${dir}" ]
}

find_existing_binary() {
    existing="$(command -v "${BIN_NAME}" 2>/dev/null || true)"
    if [ -n "${existing}" ] && [ -f "${existing}" ]; then
        printf '%s\n' "${existing}"
    fi
}

resolve_install_path() {
    case "${INSTALL_MODE}" in
        system)
            ensure_writable_dir "${SYSTEM_BIN_DIR}" "system install directory"
            printf '%s/%s\n' "${SYSTEM_BIN_DIR}" "${BIN_NAME}"
            return 0
            ;;
        user)
            [ -n "${HOME:-}" ] || die "HOME is not set. Use --bin-dir or --system."
            ensure_writable_dir "${USER_BIN_DIR}" "user install directory"
            printf '%s/%s\n' "${USER_BIN_DIR}" "${BIN_NAME}"
            return 0
            ;;
        auto)
            existing="$(find_existing_binary)"
            if [ -n "${existing}" ]; then
                existing_dir="$(dirname "${existing}")"
                if [ -w "${existing_dir}" ]; then
                    printf '%s\n' "${existing}"
                    return 0
                fi
                die "existing ${BIN_NAME} at ${existing} is not writable. Re-run with sudo, or use --user / --bin-dir."
            fi
            if can_use_dir "${SYSTEM_BIN_DIR}"; then
                printf '%s/%s\n' "${SYSTEM_BIN_DIR}" "${BIN_NAME}"
                return 0
            fi
            [ -n "${HOME:-}" ] || die "HOME is not set. Use --bin-dir or run with sudo for a system install."
            ensure_writable_dir "${USER_BIN_DIR}" "user install directory"
            printf '%s/%s\n' "${USER_BIN_DIR}" "${BIN_NAME}"
            return 0
            ;;
        *)
            die "unknown install mode: ${INSTALL_MODE}"
            ;;
    esac
}

append_candidate() {
    url="$1"
    if [ -n "${url}" ]; then
        printf '%s\n' "${url}"
    fi
}

build_candidate_list() {
    api_url="${PROTOCOL}://${API_HOST}${API_PATH}/${ASSET_NAME}"
    response="$(fetch_text "${api_url}" 2>/dev/null || true)"
    if [ -n "${response}" ]; then
        old_ifs="${IFS}"
        IFS='|'
        set -- ${response}
        IFS="${old_ifs}"
        for candidate in "$@"; do
            append_candidate "${candidate}"
        done
    else
        warn "unable to fetch mirror list from ${api_url}, falling back to GitHub release."
    fi
    append_candidate "${GITHUB_RELEASE_BASE}/${ASSET_NAME}"
}

path_in_path_env() {
    needle="$1"
    case ":${PATH:-}:" in
        *:"${needle}":*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

verify_temp_binary() {
    [ -s "${TEMP_FILE}" ] || return 1
    chmod 0755 "${TEMP_FILE}" >/dev/null 2>&1 || return 1
    "${TEMP_FILE}" --version >/dev/null 2>&1
}

download_and_install() {
    install_dir="$(dirname "${INSTALL_PATH}")"
    mkdir_or_die "${install_dir}" "install directory"
    TEMP_FILE="$(mktemp "${install_dir}/.${BIN_NAME}.tmp.XXXXXX")" || die "unable to create temporary file in ${install_dir}"
    candidate_file="$(mktemp "${install_dir}/.${BIN_NAME}.urls.XXXXXX")" || die "unable to create candidate list in ${install_dir}"
    build_candidate_list >"${candidate_file}"

    downloaded_from=""
    while IFS= read -r candidate; do
        [ -n "${candidate}" ] || continue
        if download_file "${candidate}" "${TEMP_FILE}" >/dev/null 2>&1; then
            if verify_temp_binary; then
                downloaded_from="${candidate}"
                break
            fi
        fi
        rm -f "${TEMP_FILE}" >/dev/null 2>&1 || true
        TEMP_FILE="$(mktemp "${install_dir}/.${BIN_NAME}.tmp.XXXXXX")" || die "unable to recreate temporary file in ${install_dir}"
    done <"${candidate_file}"
    rm -f "${candidate_file}" >/dev/null 2>&1 || true

    [ -n "${downloaded_from}" ] || die "failed to download a working ${ASSET_NAME}. Try again later or install from the release page manually."

    mv -f "${TEMP_FILE}" "${INSTALL_PATH}" || die "failed to move ${BIN_NAME} into place: ${INSTALL_PATH}"
    TEMP_FILE=""
    info "Downloaded from: ${downloaded_from}"
}

print_post_install() {
    install_dir="$(dirname "${INSTALL_PATH}")"
    info "Flavor: ${FLAVOR}"
    info "Installed: ${INSTALL_PATH}"
    info "Command: ${BIN_NAME}"
    info "Version check:"
    "${INSTALL_PATH}" --version || die "installed binary failed version check: ${INSTALL_PATH}"
    if ! path_in_path_env "${install_dir}"; then
        warn "${install_dir} is not in PATH."
        printf '%s\n' "Add it with: export PATH=\"${install_dir}:\$PATH\""
    fi
    printf '%s\n' "Run: ${BIN_NAME} --help"
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            http)
                PROTOCOL="http"
                shift
                ;;
            --flavor)
                [ $# -ge 2 ] || die "--flavor requires a value"
                FLAVOR="$2"
                shift 2
                ;;
            --user)
                [ "${INSTALL_MODE}" = "auto" ] || die "choose only one of --user / --system / --bin-dir"
                INSTALL_MODE="user"
                shift
                ;;
            --system)
                [ "${INSTALL_MODE}" = "auto" ] || die "choose only one of --user / --system / --bin-dir"
                INSTALL_MODE="system"
                shift
                ;;
            --bin-dir)
                [ $# -ge 2 ] || die "--bin-dir requires a path"
                [ "${INSTALL_MODE}" = "auto" ] || die "choose only one of --user / --system / --bin-dir"
                BIN_DIR="$2"
                INSTALL_MODE="bin-dir"
                shift 2
                ;;
            --arch)
                [ $# -ge 2 ] || die "--arch requires a value"
                ARCH_OVERRIDE="$2"
                shift 2
                ;;
            --softfloat)
                USE_SOFTFLOAT=1
                shift
                ;;
            --help|-h)
                print_help
                exit 0
                ;;
            *)
                die "unknown argument: $1"
                ;;
        esac
    done
}

main() {
    parse_args "$@"
    TARGET_OS="$(detect_os)" || die "unsupported operating system: $(uname -s 2>/dev/null || printf 'unknown'). Please download the release manually."
    TARGET_ARCH="$(detect_arch)"
    BIN_NAME="$(resolve_bin_name)"
    ASSET_NAME="$(resolve_asset_name)"
    pick_downloader

    if [ "${INSTALL_MODE}" = "bin-dir" ]; then
        ensure_writable_dir "${BIN_DIR}" "install directory"
        INSTALL_PATH="${BIN_DIR}/${BIN_NAME}"
    else
        INSTALL_PATH="$(resolve_install_path)"
    fi

    info "Preparing ${ASSET_NAME}"
    info "Installing to ${INSTALL_PATH}"

    download_and_install
    print_post_install
}

main "$@"
