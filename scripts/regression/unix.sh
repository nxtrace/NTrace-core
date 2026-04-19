#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

PLATFORM="${1:-}"
if [[ -z "${PLATFORM}" ]]; then
  case "$(uname -s)" in
    Linux) PLATFORM="linux" ;;
    Darwin) PLATFORM="macos" ;;
    *)
      echo "unsupported platform: $(uname -s)" >&2
      exit 1
      ;;
  esac
else
  shift || true
fi

case "${PLATFORM}" in
  linux|macos) ;;
  *)
    echo "usage: $0 [linux|macos]" >&2
    exit 1
    ;;
esac

ART_ROOT="${ART_ROOT:-${REPO_ROOT}/.tmp/ntrace-regression-${PLATFORM}-$(date +%Y%m%d-%H%M%S)}"
BIN_DIR="${ART_ROOT}/bin"
ART_DIR="${ART_ROOT}/artifacts"
SUMMARY="${ART_ROOT}/summary.tsv"
TARGETS="${ART_ROOT}/targets.txt"
DEFAULT_TMP_DIR="${ART_ROOT}/tmp"
DEFAULT_LOG_PATH="${DEFAULT_TMP_DIR}/trace.log"

BIN="${BIN_DIR}/nexttrace-current"
TINY="${BIN_DIR}/nexttrace-tiny-current"
NTR="${BIN_DIR}/ntr-current"

mkdir -p "${BIN_DIR}" "${ART_DIR}"
mkdir -p "${DEFAULT_TMP_DIR}"
: > "${SUMMARY}"

if ! command -v go >/dev/null 2>&1; then
  echo "go is required" >&2
  exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

NEED_SUDO=0
if [[ "$(id -u)" -ne 0 ]]; then
  if ! command -v sudo >/dev/null 2>&1; then
    echo "sudo is required to run runtime regression checks" >&2
    exit 1
  fi
  sudo -v
  NEED_SUDO=1
fi

record() {
  printf '%s\t%s\t%s\n' "$1" "$2" "$3" | tee -a "${SUMMARY}"
}

record_ipv6_skip() {
  record "$1" SKIP "$2; ${IPV6_SKIP_REASON}"
}

display_path() {
  local path="$1"
  if [[ -n "${HOME:-}" && "${path}" == "${HOME}"* ]]; then
    printf '~%s\n' "${path#"$HOME"}"
    return
  fi
  printf '%s\n' "${path}"
}

wrap_privileged_command() {
  local cmd="$1"
  if (( NEED_SUDO )); then
    printf 'sudo -E bash -lc %q' "${cmd}"
  else
    printf 'bash -lc %q' "${cmd}"
  fi
}

run_timeout_plain() {
  local seconds="$1"
  local command_string="$2"
  python3 - "$seconds" "$command_string" <<'PY'
import os
import signal
import subprocess
import sys

timeout = float(sys.argv[1])
command = sys.argv[2]
proc = subprocess.Popen(command, shell=True, start_new_session=True)
try:
    raise SystemExit(proc.wait(timeout=timeout))
except subprocess.TimeoutExpired:
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        proc.wait(timeout=2)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        proc.wait()
    raise SystemExit(124)
PY
}

run_timeout_cmd() {
  local seconds="$1"
  local command_string="$2"
  local wrapped
  wrapped="$(wrap_privileged_command "${command_string}")"
  run_timeout_plain "${seconds}" "${wrapped}"
}

make_runner_script() {
  local command_string="$1"
  local script_path
  script_path="$(mktemp "${ART_ROOT}/runner.XXXXXX")"
  {
    printf '%s\n' '#!/usr/bin/env bash'
    printf '%s\n' 'set -euo pipefail'
    printf '%s\n' 'trap '\''rm -f -- "$0"'\'' EXIT'
    if (( NEED_SUDO )); then
      printf 'sudo -E bash -lc %q\n' "${command_string}"
    else
      printf 'bash -lc %q\n' "${command_string}"
    fi
  } > "${script_path}"
  chmod +x "${script_path}"
  printf '%s\n' "${script_path}"
}

make_deploy_runner_script() {
  local bin_path="$1"
  local pid_file="$2"
  local inner_cmd
  local script_path
  script_path="$(mktemp "${ART_ROOT}/deploy-runner.XXXXXX")"
  inner_cmd="$(printf "printf '%%s\\n' \"\\\$\\\$\" > %q; exec %q --listen 127.0.0.1:0 --deploy" "${pid_file}" "${bin_path}")"
  {
    printf '%s\n' '#!/usr/bin/env bash'
    printf '%s\n' 'set -euo pipefail'
    printf '%s\n' 'trap '\''rm -f -- "$0"'\'' EXIT'
    if (( NEED_SUDO )); then
      printf 'sudo -E bash -lc %q\n' "${inner_cmd}"
    else
      printf 'bash -lc %q\n' "${inner_cmd}"
    fi
  } > "${script_path}"
  chmod +x "${script_path}"
  printf '%s\n' "${script_path}"
}

run_cmd() {
  local name="$1"
  local note="$2"
  local command_string="$3"
  run_cmd_check "${name}" "${note}" "${command_string}" "" "" "0"
}

allowed_exit_matches() {
  local rc="$1"
  local allowed_csv="${2:-0}"
  local allowed
  IFS=',' read -r -a allowed <<< "${allowed_csv}"
  local candidate
  for candidate in "${allowed[@]}"; do
    if [[ "${rc}" == "${candidate}" ]]; then
      return 0
    fi
  done
  return 1
}

run_cmd_check() {
  local name="$1"
  local note="$2"
  local command_string="$3"
  local success_pattern="${4:-}"
  local forbidden_pattern="${5:-}"
  local allowed_csv="${6:-0}"
  local out="${ART_DIR}/${name}.txt"
  local rc=0
  if run_timeout_cmd 150 "${command_string}" >"${out}" 2>&1; then
    rc=0
  else
    rc=$?
  fi
  if ! allowed_exit_matches "${rc}" "${allowed_csv}"; then
    record "${name}" FAIL "${note}; exit=${rc}"
    return
  fi
  if [[ -n "${success_pattern}" ]] && ! grep -Eq -- "${success_pattern}" "${out}"; then
    record "${name}" FAIL "${note}; output mismatch"
    return
  fi
  if [[ -n "${forbidden_pattern}" ]] && grep -Eq -- "${forbidden_pattern}" "${out}"; then
    record "${name}" FAIL "${note}; forbidden output matched"
    return
  fi
  record "${name}" PASS "${note}"
}

check_json_pure() {
  local name="$1"
  local note="$2"
  local command_string="$3"
  local expected_pattern="${4:-}"
  local allowed_csv="${5:-0}"
  local out="${ART_DIR}/${name}.txt"
  local service_err='request failed - please try again later'
  local pow_log_re='^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} pow token fetch failed: .*$'
  local rc=0
  if run_timeout_cmd 180 "${command_string}" >"${out}" 2>&1; then
    rc=0
  else
    rc=$?
  fi
  if grep -Fq "${service_err}" "${out}"; then
    record "${name}" SKIP "${note}; external service unavailable"
    return
  fi
  if ! allowed_exit_matches "${rc}" "${allowed_csv}"; then
    record "${name}" FAIL "${note}; exit=${rc}"
    return
  fi
  if python3 - <<'PY' "${out}" "${pow_log_re}" && ! grep -Fq 'preferred API IP' "${out}"; then
import json
import re
import sys

text = open(sys.argv[1], 'r', encoding='utf-8', errors='replace').read()
text = re.sub(sys.argv[2], '', text, flags=re.MULTILINE)
decoder = json.JSONDecoder()
idx = 0
length = len(text)
while idx < length and text[idx].isspace():
    idx += 1
if idx >= length:
    raise SystemExit(1)
_, end = decoder.raw_decode(text, idx)
if text[end:].strip():
    raise SystemExit(1)
PY
    if [[ -n "${expected_pattern}" ]] && ! grep -Eq -- "${expected_pattern}" "${out}"; then
      record "${name}" FAIL "${note}; JSON content mismatch"
      return
    fi
    record "${name}" PASS "${note}"
  else
    record "${name}" FAIL "${note}; stdout not pure JSON"
  fi
}

check_no_clear_ansi() {
  local name="$1"
  local note="$2"
  local command_string="$3"
  local out="${ART_DIR}/${name}.txt"
  if ! run_timeout_cmd 120 "${command_string}" >"${out}" 2>&1; then
    record "${name}" FAIL "${note}; command failed"
    return
  fi
  if LC_ALL=C grep -q $'\033\[H\033\[2J' "${out}"; then
    record "${name}" FAIL "${note}; found clear-screen ANSI"
  else
    record "${name}" PASS "${note}"
  fi
}

check_output_file() {
  local name="$1"
  local note="$2"
  local command_string="$3"
  local path="$4"
  local out="${ART_DIR}/${name}.txt"
  rm -f "${path}" "${DEFAULT_LOG_PATH}"
  if ! run_timeout_cmd 150 "${command_string}" >"${out}" 2>&1; then
    record "${name}" FAIL "${note}; command failed"
    return
  fi
  if [[ ! -s "${path}" ]]; then
    record "${name}" FAIL "${note}; log file missing"
    return
  fi
  record "${name}" PASS "${note}"
}

check_mtu_tty_color() {
  local name="$1"
  local note="$2"
  local command_string="$3"
  local out="${ART_DIR}/${name}.txt"
  if ! command -v script >/dev/null 2>&1; then
    record "${name}" SKIP "${note}; script command not available"
    return
  fi
  local runner
  runner="$(make_runner_script "${command_string}")"
  local script_cmd
  if [[ "${PLATFORM}" == "macos" ]]; then
    script_cmd="script -q /dev/null $(printf '%q' "${runner}")"
  else
    script_cmd="script -qfec $(printf '%q' "${runner}") /dev/null"
  fi
  if ! run_timeout_plain 180 "${script_cmd}" >"${out}" 2>&1; then
    record "${name}" FAIL "${note}; command failed"
    return
  fi
  if grep -q $'\033\[' "${out}" && grep -Fq 'Path MTU:' "${out}"; then
    record "${name}" PASS "${note}"
  else
    record "${name}" FAIL "${note}; ANSI color not observed"
  fi
}

check_mtu_non_tty_plain() {
  local name="$1"
  local note="$2"
  local command_string="$3"
  local out="${ART_DIR}/${name}.txt"
  if ! run_timeout_cmd 180 "${command_string}" >"${out}" 2>&1; then
    record "${name}" FAIL "${note}; command failed"
    return
  fi
  if grep -q $'\033\[' "${out}"; then
    record "${name}" FAIL "${note}; unexpected ANSI"
    return
  fi
  if ! grep -Fq 'Path MTU:' "${out}"; then
    record "${name}" FAIL "${note}; missing expected MTU"
    return
  fi
  record "${name}" PASS "${note}"
}

detect_capture_iface() {
  local dest="${1:-1.1.1.1}"
  if [[ "${PLATFORM}" == "macos" ]]; then
    if [[ "${dest}" == *:* ]]; then
      route -n get -inet6 "${dest}" 2>/dev/null | awk '/interface:/{print $2; exit}' || true
    else
      route -n get "${dest}" 2>/dev/null | awk '/interface:/{print $2; exit}' || true
    fi
  else
    if [[ "${dest}" == *:* ]]; then
      ip -6 route get "${dest}" 2>/dev/null | sed -n 's/.* dev \([^ ]*\).*/\1/p' | head -n1 || true
    else
      ip route get "${dest}" 2>/dev/null | sed -n 's/.* dev \([^ ]*\).*/\1/p' | head -n1 || true
    fi
  fi
}

detect_ipv6_available() {
  if [[ "${PLATFORM}" == "macos" ]]; then
    route -n get -inet6 2606:4700:4700::1111 >/dev/null 2>&1
  else
    command -v ip >/dev/null 2>&1 && ip -6 route get 2606:4700:4700::1111 >/dev/null 2>&1
  fi
}

capture_psize_tos() {
  local name="$1"
  local note="$2"
  local filter_host="$3"
  local command_string="$4"
  local expect1="$5"
  local expect2="$6"
  local dump="${ART_DIR}/${name}.tcpdump.txt"
  local out="${ART_DIR}/${name}.cmd.txt"
  if ! command -v tcpdump >/dev/null 2>&1; then
    record "${name}" SKIP "${note}; tcpdump not available"
    return
  fi
  local iface
  iface="$(detect_capture_iface "${filter_host}")"
  if [[ -z "${iface}" ]]; then
    record "${name}" SKIP "${note}; capture interface not detected"
    return
  fi
  rm -f "${dump}" "${out}"
  if (( NEED_SUDO )); then
    sudo -E bash -lc 'exec tcpdump -i "$1" -nn -vvv -c 1 "dst host $2" >"$3" 2>&1' _ "${iface}" "${filter_host}" "${dump}" &
  else
    tcpdump -i "${iface}" -nn -vvv -c 1 "dst host ${filter_host}" >"${dump}" 2>&1 &
  fi
  local tcpdump_pid=$!
  sleep 1
  run_timeout_cmd 60 "${command_string}" >"${out}" 2>&1 || true
  sleep 2
  if kill -0 "${tcpdump_pid}" >/dev/null 2>&1; then
    kill -INT "${tcpdump_pid}" >/dev/null 2>&1 || true
  fi
  wait "${tcpdump_pid}" >/dev/null 2>&1 || true
  if grep -Fq "${expect1}" "${dump}" && grep -Fq "${expect2}" "${dump}"; then
    record "${name}" PASS "${note}"
  else
    record "${name}" FAIL "${note}; packet capture mismatch"
  fi
}

wait_http_ready() {
  local url="$1"
  for _ in $(seq 1 30); do
    if curl -fsS --connect-timeout 2 --max-time 5 "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

extract_http_url() {
  local log_path="$1"
  grep -Eo 'http://[^[:space:]]+' "${log_path}" 2>/dev/null | head -n1 || true
}

wait_deploy_base_url() {
  local log_path="$1"
  local url
  for _ in $(seq 1 30); do
    url="$(extract_http_url "${log_path}")"
    if [[ -n "${url}" ]]; then
      printf '%s\n' "${url}"
      return 0
    fi
    sleep 1
  done
  return 1
}

cleanup_deploy_process() {
  local child_pid=""
  if [[ -n "${DEPLOY_CHILD_PID_FILE:-}" && -f "${DEPLOY_CHILD_PID_FILE}" ]]; then
    child_pid="$(tr -d '[:space:]' < "${DEPLOY_CHILD_PID_FILE}")"
    if [[ "${child_pid}" =~ ^[0-9]+$ ]]; then
      if (( NEED_SUDO )); then
        sudo kill "${child_pid}" >/dev/null 2>&1 || true
      else
        kill "${child_pid}" >/dev/null 2>&1 || true
      fi
    fi
  fi
  if [[ -n "${DEPLOY_PID:-}" ]]; then
    kill "${DEPLOY_PID}" >/dev/null 2>&1 || true
    wait "${DEPLOY_PID}" >/dev/null 2>&1 || true
    DEPLOY_PID=""
  fi
  if [[ -n "${DEPLOY_CHILD_PID_FILE:-}" ]]; then
    rm -f "${DEPLOY_CHILD_PID_FILE}" >/dev/null 2>&1 || true
    DEPLOY_CHILD_PID_FILE=""
  fi
  if [[ -n "${DEPLOY_RUNNER:-}" ]]; then
    rm -f "${DEPLOY_RUNNER}" >/dev/null 2>&1 || true
    DEPLOY_RUNNER=""
  fi
}

echo "artifacts: $(display_path "${ART_ROOT}")"
echo "building binaries..."
(cd "${REPO_ROOT}" && go build -trimpath -o "${BIN}" .)
(cd "${REPO_ROOT}" && go build -trimpath -tags flavor_tiny -o "${TINY}" .)
(cd "${REPO_ROOT}" && go build -trimpath -tags flavor_ntr -o "${NTR}" .)

echo "running unit tests..."
(cd "${REPO_ROOT}" && go test ./...)

IPV6_AVAILABLE=0
IPV6_SKIP_REASON='IPv6 not available on this machine'
if [[ "${NTRACE_SKIP_IPV6:-0}" == "1" ]]; then
  IPV6_SKIP_REASON='IPv6 checks disabled by NTRACE_SKIP_IPV6'
elif detect_ipv6_available; then
  IPV6_AVAILABLE=1
fi
{
  printf '1.1.1.1 Cloudflare-v4\n'
  if (( IPV6_AVAILABLE )); then
    printf '2606:4700:4700::1111 Cloudflare-v6\n'
  fi
} > "${TARGETS}"
echo "ipv6_available=${IPV6_AVAILABLE}"
if (( ! IPV6_AVAILABLE )); then
  echo "ipv6_skip_reason=${IPV6_SKIP_REASON}"
fi

run_cmd icmp4_basic 'ICMP IPv4 basic trace' "\"${BIN}\" --no-color -q 1 -m 3 --timeout 1000 1.1.1.1"
run_cmd tcp4_basic 'TCP IPv4 basic trace' "\"${BIN}\" --no-color -T -q 1 -m 3 --timeout 1000 1.1.1.1"
run_cmd udp4_basic 'UDP IPv4 basic trace' "\"${BIN}\" --no-color -U -q 1 -m 3 --timeout 1000 1.1.1.1"
if (( IPV6_AVAILABLE )); then
  run_cmd icmp6_basic 'ICMP IPv6 basic trace' "\"${BIN}\" --no-color -6 -q 1 -m 3 --timeout 1000 2606:4700:4700::1111"
  run_cmd tcp6_basic 'TCP IPv6 basic trace' "\"${BIN}\" --no-color -6 -T -q 1 -m 3 --timeout 1000 2606:4700:4700::1111"
  run_cmd udp6_basic 'UDP IPv6 basic trace' "\"${BIN}\" --no-color -6 -U -q 1 -m 3 --timeout 1000 2606:4700:4700::1111"
else
  record_ipv6_skip icmp6_basic 'ICMP IPv6 basic trace'
  record_ipv6_skip tcp6_basic 'TCP IPv6 basic trace'
  record_ipv6_skip udp6_basic 'UDP IPv6 basic trace'
fi
run_cmd raw_output 'Raw hop rows' "\"${BIN}\" --no-color --raw -q 1 -m 2 --timeout 1000 1.1.1.1"
run_cmd classic_output 'Classic printer' "\"${BIN}\" --no-color --classic -q 1 -m 2 --timeout 1000 1.1.1.1"
run_cmd route_path 'Route-path summary' "\"${BIN}\" --no-color --route-path -q 1 -m 2 --timeout 1000 1.1.1.1"
run_cmd provider_lang 'IP.SB + sakura + en' "\"${BIN}\" --no-color -q 1 -m 2 --timeout 1000 --data-provider IP.SB --pow-provider sakura --language en 1.1.1.1"
run_cmd dot_resolver 'DoT resolver via aliyun' "\"${BIN}\" --no-color --dot-server aliyun -q 1 -m 1 --timeout 1000 ipv4.pek-4134.endpoint.nxtrace.org"
run_cmd disable_geoip 'disable-geoip path' "\"${BIN}\" --no-color --data-provider disable-geoip -M -q 1 -m 2 --timeout 1000 1.1.1.1"
run_cmd dn42_mode 'DN42 mode switch' "\"${BIN}\" --no-color --dn42 -q 1 -m 2 --timeout 1000 1.1.1.1"
check_json_pure json_trace 'Traceroute JSON stdout purity' "\"${BIN}\" --no-color --json -q 1 -m 3 --timeout 1000 1.1.1.1"
check_json_pure json_mtu 'MTU JSON stdout purity' "\"${BIN}\" --no-color --mtu --json --timeout 1000 -q 1 -m 3 1.1.1.1"
check_json_pure json_globalping 'Globalping JSON stdout purity' "\"${BIN}\" --no-color --json --from Germany -q 1 -m 3 --timeout 1000 1.1.1.1"
check_no_clear_ansi table_non_tty 'Table output without clear-screen ANSI in non-TTY' "\"${BIN}\" --no-color --table -q 1 -m 2 --timeout 1000 1.1.1.1"
check_output_file output_custom 'Custom output file path' "\"${BIN}\" --no-color -q 1 -m 2 --timeout 1000 -o \"${ART_ROOT}/custom.log\" 1.1.1.1" "${ART_ROOT}/custom.log"
check_output_file output_default 'Default output file path' "TMPDIR=\"${DEFAULT_TMP_DIR}\" \"${BIN}\" --no-color -q 1 -m 2 --timeout 1000 -O 1.1.1.1" "${DEFAULT_LOG_PATH}"
run_cmd mtu_text 'MTU text mode' "\"${BIN}\" --no-color --mtu --timeout 1000 -q 1 -m 3 1.1.1.1"
check_mtu_tty_color mtu_tty_color 'MTU TTY colorized output' "\"${BIN}\" --mtu --timeout 1000 -q 1 -m 3 1.1.1.1"
check_mtu_non_tty_plain mtu_non_tty_plain 'MTU non-TTY output has no ANSI' "\"${BIN}\" --mtu --timeout 1000 -q 1 -m 3 1.1.1.1"
run_cmd mtr_report 'MTR report ICMP' "\"${BIN}\" --no-color -r -q 2 -i 300 --timeout 1000 -m 4 1.1.1.1"
run_cmd mtr_wide 'MTR wide + show-ips' "\"${BIN}\" --no-color -w --show-ips -q 2 -i 300 --timeout 1000 -m 4 1.1.1.1"
run_cmd mtr_raw 'MTR raw stream' "\"${BIN}\" --no-color -r --raw -q 2 -i 300 --timeout 1000 -m 4 1.1.1.1"
run_cmd fast_trace_file 'Fast trace via --file' "\"${BIN}\" --no-color --file \"${TARGETS}\" -q 1 -m 2 --timeout 1000"
run_cmd_check help_speed_main 'Main help exposes only top-level speed entry' "\"${BIN}\" --help" --speed --speed-provider
run_cmd_check help_speed_detail 'Speed help is dedicated and detailed' "\"${BIN}\" --speed --help" --speed-provider 'hops max, .*ICMP mode'
check_json_pure speed_apple_json 'Apple speed JSON path' "\"${BIN}\" --speed --json --no-metadata --non-interactive --max 64KiB --timeout 2000 --threads 2 --latency-count 2" '"provider":"apple"' '0,2'
check_json_pure speed_cloudflare_json 'Cloudflare speed JSON path' "\"${BIN}\" --speed --speed-provider cloudflare --json --no-metadata --non-interactive --max 64KiB --timeout 2000 --threads 2 --latency-count 2" '"provider":"cloudflare"' '0,2'
run_cmd tiny_smoke 'nexttrace-tiny smoke' "\"${TINY}\" --no-color -q 1 -m 2 --timeout 1000 1.1.1.1"
run_cmd ntr_report 'ntr report smoke' "\"${NTR}\" --no-color -r -q 2 -i 300 --timeout 1000 -m 4 1.1.1.1"
run_cmd_check tiny_speed_reject 'nexttrace-tiny rejects --speed' "\"${TINY}\" --speed" '--speed is not available' 'panic|goroutine [0-9]+|stack trace' '1'
run_cmd_check ntr_speed_reject 'ntr rejects --speed' "\"${NTR}\" --speed" '--speed is not available' 'panic|goroutine [0-9]+|stack trace' '1'

capture_psize_tos psize_tos_icmp4 'ICMPv4 psize/tos packet capture' '1.1.1.1' "\"${BIN}\" --no-color -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 1.1.1.1" 'tos 0x2e' 'length 84'
capture_psize_tos psize_tos_udp4 'UDPv4 psize/tos packet capture' '1.1.1.1' "\"${BIN}\" --no-color -U -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 1.1.1.1" 'tos 0x2e' 'length 84'
capture_psize_tos psize_tos_tcp4 'TCPv4 psize/tos packet capture' '1.1.1.1' "\"${BIN}\" --no-color -T -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 1.1.1.1" 'tos 0x2e' 'length 84'
if (( IPV6_AVAILABLE )); then
  capture_psize_tos psize_tos_icmp6 'ICMPv6 psize/tos packet capture' '2606:4700:4700::1111' "\"${BIN}\" --no-color -6 -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 2606:4700:4700::1111" 'class 0x2e' 'payload length: 44'
  capture_psize_tos psize_tos_udp6 'UDPv6 psize/tos packet capture' '2606:4700:4700::1111' "\"${BIN}\" --no-color -6 -U -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 2606:4700:4700::1111" 'class 0x2e' 'payload length: 44'
  capture_psize_tos psize_tos_tcp6 'TCPv6 psize/tos packet capture' '2606:4700:4700::1111' "\"${BIN}\" --no-color -6 -T -q 1 -m 1 --timeout 1000 --psize 84 -Q 46 2606:4700:4700::1111" 'class 0x2e' 'payload length: 44'
else
  record_ipv6_skip psize_tos_icmp6 'ICMPv6 psize/tos packet capture'
  record_ipv6_skip psize_tos_udp6 'UDPv6 psize/tos packet capture'
  record_ipv6_skip psize_tos_tcp6 'TCPv6 psize/tos packet capture'
fi

DEPLOY_BASE_URL=""
DEPLOY_LOG="${ART_DIR}/deploy_server.txt"
DEPLOY_CHILD_PID_FILE="${ART_DIR}/deploy_server.pid"
rm -f "${DEPLOY_CHILD_PID_FILE}"
DEPLOY_RUNNER="$(make_deploy_runner_script "${BIN}" "${DEPLOY_CHILD_PID_FILE}")"
"${DEPLOY_RUNNER}" >"${DEPLOY_LOG}" 2>&1 &
DEPLOY_PID=$!
trap cleanup_deploy_process INT TERM EXIT
if DEPLOY_BASE_URL="$(wait_deploy_base_url "${DEPLOY_LOG}")" && wait_http_ready "${DEPLOY_BASE_URL}/"; then
  if curl -fsS --connect-timeout 2 --max-time 5 "${DEPLOY_BASE_URL}/" >"${ART_DIR}/deploy_root.txt" 2>&1; then
    record deploy_root PASS 'Web root reachable'
  else
    record deploy_root FAIL 'Web root curl failed'
  fi
  if curl -fsS --connect-timeout 2 --max-time 5 "${DEPLOY_BASE_URL}/api/options" >"${ART_DIR}/deploy_options.txt" 2>&1 && grep -Fq '"packet_size":null' "${ART_DIR}/deploy_options.txt" && grep -Fq '"tos":0' "${ART_DIR}/deploy_options.txt"; then
    record deploy_options PASS 'Options API exposes packet_size=null and tos=0'
  else
    record deploy_options FAIL 'Options API check failed'
  fi
  if curl -fsS --connect-timeout 2 --max-time 5 -X POST -H 'Content-Type: application/json' --data '{"target":"1.1.1.1","queries":1,"max_hops":3,"timeout_ms":1000}' "${DEPLOY_BASE_URL}/api/trace" >"${ART_DIR}/deploy_trace.txt" 2>&1 && grep -Fq '"resolved_ip"' "${ART_DIR}/deploy_trace.txt"; then
    record deploy_trace PASS 'REST trace endpoint works'
  else
    record deploy_trace FAIL 'REST trace endpoint failed'
  fi
else
  record deploy_root FAIL 'deploy server not ready'
  record deploy_options FAIL 'deploy server not ready'
  record deploy_trace FAIL 'deploy server not ready'
fi
cleanup_deploy_process
trap - INT TERM EXIT

echo "__SUMMARY__"
cat "${SUMMARY}"
awk -F'\t' '{total++; if ($2=="PASS") pass++; else if ($2=="FAIL") fail++; else skip++} END {printf "pass=%d fail=%d skip=%d total=%d\n", pass, fail, skip, total}' "${SUMMARY}"
echo "artifacts_root=$(display_path "${ART_ROOT}")"
fail_count="$(awk -F'\t' '$2=="FAIL"{c++} END {print c+0}' "${SUMMARY}")"
if (( fail_count > 0 )); then
  exit 1
fi
