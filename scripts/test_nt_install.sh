#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${ROOT_DIR}/nt_install.sh"
REAL_UNAME="$(command -v uname)"

fail() {
  printf '%s\n' "FAIL: $*" >&2
  exit 1
}

assert_file_exists() {
  [ -f "$1" ] || fail "missing file: $1"
}

assert_contains() {
  file="$1"
  needle="$2"
  grep -Fq "$needle" "$file" || fail "expected '$needle' in $file"
}

setup_case_dir() {
  case_dir="$(mktemp -d)"
  mkdir -p "${case_dir}/mockbin" "${case_dir}/home" "${case_dir}/system-bin"

  cat > "${case_dir}/mockbin/uname" <<EOF
#!/bin/sh
case "\${1-}" in
  -s) printf '%s\n' "\${MOCK_UNAME_S:-Linux}" ;;
  -m) printf '%s\n' "\${MOCK_UNAME_M:-x86_64}" ;;
  *) exec "${REAL_UNAME}" "\$@" ;;
esac
EOF
  chmod +x "${case_dir}/mockbin/uname"

  cat > "${case_dir}/mockbin/curl" <<'EOF'
#!/bin/sh
set -eu

log_file="${MOCK_CURL_LOG:-}"
output=""
url=""

while [ $# -gt 0 ]; do
  case "$1" in
    -o|--output)
      output="$2"
      shift 2
      ;;
    --connect-timeout|--max-time|--retry|--retry-delay)
      shift 2
      ;;
    -f|-s|-S|-L|-fsSL|-fsSLI)
      shift
      ;;
    -*)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

[ -n "$log_file" ] && printf '%s\n' "$url" >> "$log_file"

case "$url" in
  *"/api/dist/core/"*)
    if [ "${MOCK_API_FAIL:-0}" = "1" ]; then
      exit 22
    fi
    printf '%s' "${MOCK_API_RESPONSE:-}"
    ;;
  *)
    if [ "${MOCK_DOWNLOAD_FAIL:-0}" = "1" ]; then
      exit 22
    fi
    [ -n "$output" ] || exit 1
    cat > "$output" <<'EOS'
#!/bin/sh
if [ "${1-}" = "--version" ]; then
  echo "nexttrace mock 0.0.0"
  exit 0
fi
exit 0
EOS
    chmod +x "$output"
    ;;
esac
EOF
  chmod +x "${case_dir}/mockbin/curl"

  printf '%s\n' "${case_dir}"
}

run_installer() {
  case_dir="$1"
  path_prefix="${2:-}"
  shift 2 || true

  run_path="${case_dir}/mockbin:/usr/bin:/bin"
  if [ -n "${path_prefix}" ]; then
    run_path="${path_prefix}:${run_path}"
  fi

  set +e
  (
    export HOME="${case_dir}/home"
    export PATH="${run_path}"
    export NT_INSTALL_SYSTEM_BIN_DIR="${case_dir}/system-bin"
    export NT_INSTALL_USER_BIN_DIR="${case_dir}/home/.local/bin"
    export MOCK_CURL_LOG="${case_dir}/curl.log"
    cat "${INSTALLER}" | bash -s -- "$@"
  ) >"${case_dir}/stdout.log" 2>"${case_dir}/stderr.log"
  status=$?
  set -e
  return "${status}"
}

test_default_full() {
  case_dir="$(setup_case_dir)"
  export MOCK_API_RESPONSE="https://mirror.invalid/nexttrace_linux_amd64|https://backup.invalid/nexttrace_linux_amd64"
  run_installer "${case_dir}" ""
  assert_file_exists "${case_dir}/system-bin/nexttrace"
  assert_contains "${case_dir}/stdout.log" "Command: nexttrace"
  rm -rf "${case_dir}"
}

test_tiny_flavor() {
  case_dir="$(setup_case_dir)"
  export MOCK_API_RESPONSE="https://mirror.invalid/nexttrace-tiny_linux_amd64|https://backup.invalid/nexttrace-tiny_linux_amd64"
  run_installer "${case_dir}" "" --flavor tiny
  assert_file_exists "${case_dir}/system-bin/nexttrace-tiny"
  assert_contains "${case_dir}/stdout.log" "Command: nexttrace-tiny"
  rm -rf "${case_dir}"
}

test_ntr_flavor() {
  case_dir="$(setup_case_dir)"
  export MOCK_API_RESPONSE="https://mirror.invalid/ntr_linux_amd64|https://backup.invalid/ntr_linux_amd64"
  run_installer "${case_dir}" "" --flavor ntr
  assert_file_exists "${case_dir}/system-bin/ntr"
  assert_contains "${case_dir}/stdout.log" "Command: ntr"
  rm -rf "${case_dir}"
}

test_darwin_universal_asset() {
  case_dir="$(setup_case_dir)"
  export MOCK_UNAME_S="Darwin"
  export MOCK_UNAME_M="arm64"
  export MOCK_API_RESPONSE="https://mirror.invalid/nexttrace_darwin_universal|https://backup.invalid/nexttrace_darwin_universal"
  run_installer "${case_dir}" ""
  assert_contains "${case_dir}/curl.log" "/api/dist/core/nexttrace_darwin_universal"
  assert_file_exists "${case_dir}/system-bin/nexttrace"
  rm -rf "${case_dir}"
  unset MOCK_UNAME_S MOCK_UNAME_M
}

test_user_mode() {
  case_dir="$(setup_case_dir)"
  export MOCK_API_RESPONSE="https://mirror.invalid/nexttrace_linux_amd64"
  run_installer "${case_dir}" "" --user
  assert_file_exists "${case_dir}/home/.local/bin/nexttrace"
  rm -rf "${case_dir}"
}

test_system_mode() {
  case_dir="$(setup_case_dir)"
  export MOCK_API_RESPONSE="https://mirror.invalid/nexttrace_linux_amd64"
  run_installer "${case_dir}" "" --system
  assert_file_exists "${case_dir}/system-bin/nexttrace"
  rm -rf "${case_dir}"
}

test_bin_dir_mode() {
  case_dir="$(setup_case_dir)"
  custom_dir="${case_dir}/custom-bin"
  export MOCK_API_RESPONSE="https://mirror.invalid/nexttrace_linux_amd64"
  run_installer "${case_dir}" "" --bin-dir "${custom_dir}"
  assert_file_exists "${custom_dir}/nexttrace"
  rm -rf "${case_dir}"
}

test_github_fallback() {
  case_dir="$(setup_case_dir)"
  export MOCK_API_FAIL=1
  run_installer "${case_dir}" ""
  assert_file_exists "${case_dir}/system-bin/nexttrace"
  assert_contains "${case_dir}/curl.log" "https://github.com/nxtrace/NTrace-dev/releases/latest/download/nexttrace_linux_amd64"
  rm -rf "${case_dir}"
  unset MOCK_API_FAIL
}

test_existing_unwritable_binary_rejected() {
  case_dir="$(setup_case_dir)"
  existing_dir="${case_dir}/existing-bin"
  mkdir -p "${existing_dir}"
  cat > "${existing_dir}/nexttrace" <<'EOF'
#!/bin/sh
exit 0
EOF
  chmod 0555 "${existing_dir}/nexttrace"
  chmod 0555 "${existing_dir}"

  export MOCK_API_RESPONSE="https://mirror.invalid/nexttrace_linux_amd64"
  if run_installer "${case_dir}" "${existing_dir}"; then
    fail "installer should reject unwritable existing binary"
  fi
  assert_contains "${case_dir}/stderr.log" "existing nexttrace"
  [ ! -f "${case_dir}/system-bin/nexttrace" ] || fail "unexpected second install in system bin"
  [ ! -f "${case_dir}/home/.local/bin/nexttrace" ] || fail "unexpected second install in user bin"
  chmod 0755 "${existing_dir}" "${existing_dir}/nexttrace"
  rm -rf "${case_dir}"
}

main() {
  test_default_full
  test_tiny_flavor
  test_ntr_flavor
  test_darwin_universal_asset
  test_user_mode
  test_system_mode
  test_bin_dir_mode
  test_github_fallback
  test_existing_unwritable_binary_rejected
  printf '%s\n' "nt_install smoke tests passed"
}

main "$@"
