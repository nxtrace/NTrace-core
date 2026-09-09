<div align="center">

<img src="assets/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

<h1 align="center">
  <br>NextTrace<br>
</h1>

<h4 align="center">An open source visual routing tool that pursues light weight, developed using Golang.</h4>

---

<h6 align="center">HomePage: www.nxtrace.org</h6>

<p align="center">
  <a href="https://github.com/nxtrace/NTrace-dev/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/nxtrace/NTrace-dev/build.yml?branch=main&style=flat-square" alt="Github Actions">
  </a>
  <a href="https://github.com/nxtrace/NTrace-dev/actions/workflows/golangci-lint.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/nxtrace/NTrace-dev/golangci-lint.yml?branch=main&style=flat-square&label=golangci-lint" alt="golangci-lint">
  </a>
  <a href="https://github.com/nxtrace/NTrace-dev/releases">
    <img src="https://img.shields.io/github/release/nxtrace/NTrace-dev/all.svg?style=flat-square">
  </a>
</p>

## Default-mode migration: no earlier than 2027

> **Downstream developers:** NextTrace will switch the default operating and display mode of `nexttrace` and `nexttrace-tiny` to MTR **no earlier than 2027**. Using `--raw` alone will switch to MTR RAW at the same time. Traditional traceroute and its RAW output will remain available through `-k/--traceroute`. Programs relying on today's behavior should adopt `--traceroute` or `--traceroute --raw` now. **This release does not change the default; the switching release will be announced separately.** `ntr` remains MTR-only.

## IAAS Sponsor

<div style="text-align: center;">
    <a href="https://dmit.io">
        <img src="https://assets.nxtrace.org/dmit.svg" width="170.7" height="62.9">
    </a>
    &nbsp;&nbsp;&nbsp;&nbsp;
    <a href="https://misaka.io" >
        <img src="https://assets.nxtrace.org/misaka.svg" width="170.7" height="62.9">
    </a>
    &nbsp;&nbsp;&nbsp;&nbsp;
    <a href="https://portal.saltyfish.io" >
        <img src="https://assets.nxtrace.org/snapstack.svg" width="170.7" height="62.9">
    </a>
</div>

We are extremely grateful to [DMIT](https://dmit.io), [Misaka](https://misaka.io) and [SnapStack](https://portal.saltyfish.io) for providing the network infrastructure that powers this project.

## How To Use

Document Language: English | [简体中文](README_zh_CN.md)

⚠️ Please note: We welcome PR submissions from the community, but please submit your PRs to the [NTrace-dev](https://github.com/nxtrace/NTrace-dev) repository instead of [NTrace-core](https://github.com/nxtrace/NTrace-core) repository.<br>
Regarding the NTrace-dev and NTrace-core repositories:<br>
Both will largely remain consistent with each other. All development work is done within the NTrace-dev repository. The NTrace-dev repository releases new versions first. After running stably for an undetermined period, we will synchronize that version to NTrace-core. This means that the NTrace-dev repository serves as a "beta" or "testing" version.<br>
Please note, there are exceptions to this synchronization. If a version of NTrace-dev encounters a serious bug, NTrace-core will skip that flawed version and synchronize directly to the next version that resolves the issue.

### Automated Install

- Debian / Ubuntu
  - Recommended: install from the official `nexttrace-debs` APT repository
    - Supports: `amd64`, `i386`, `arm64`, `armel`, `armhf`, `loong64`, `mipsel`, `mips64el`, `ppc64el`, `riscv64`, `s390x`
    - Add the repository and install the default package:
      ```shell
      sudo install -d -m 0755 /etc/apt/keyrings
      curl -fsSL -o /tmp/nexttrace-archive-keyring.gpg https://github.com/nxtrace/nexttrace-debs/releases/latest/download/nexttrace-archive-keyring.gpg
      sudo install -m 0644 /tmp/nexttrace-archive-keyring.gpg /etc/apt/keyrings/nexttrace.gpg
      rm -f /tmp/nexttrace-archive-keyring.gpg
      printf '%s\n' 'Types: deb' 'URIs: https://github.com/nxtrace/nexttrace-debs/releases/latest/download/' 'Suites: ./' 'Signed-By: /etc/apt/keyrings/nexttrace.gpg' | sudo tee /etc/apt/sources.list.d/nexttrace.sources >/dev/null
      sudo apt update
      sudo apt install nexttrace
      ```
    - Optionally install additional flavors:
      ```shell
      sudo apt install nexttrace-tiny
      sudo apt install ntr
      ```
    - Packages can be installed side by side. Commands: `nexttrace`, `nexttrace-tiny`, `ntr`

- Linux / macOS / BSD
  - One-click installation script (Full, default)

    ```shell
    curl -sL https://nxtrace.org/nt | bash
    ```

  - One-click installation script (Tiny)

    ```shell
    curl -sL https://nxtrace.org/nt | bash -s -- --flavor tiny
    ```

  - One-click installation script (NTR)

    ```shell
    curl -sL https://nxtrace.org/nt | bash -s -- --flavor ntr
    ```

  - Installed command names: Full `nexttrace`, Tiny `nexttrace-tiny`, NTR `ntr`

  - Arch Linux AUR installation command
    - Directly download bin package (only supports amd64)
      ```shell
      yay -S nexttrace-bin
      ```
    - Build from source (only supports amd64)
      ```shell
      yay -S nexttrace
      ```
    - The AUR builds are maintained by ouuan, huyz

  - Linuxbrew's installation command

    Same as the macOS Homebrew installation method. The homebrew-core formula provides the Full flavor (`nexttrace`); the `nxtrace/nexttrace` tap provides all three flavors.

  - deepin installation command
    ```shell
    apt install nexttrace
    ```
  - [x-cmd](https://www.x-cmd.com/pkg/nexttrace) installation command

    ```shell
    x env use nexttrace
    ```

  - Termux installation command
    ```shell
    pkg install root-repo
    pkg install nexttrace
    ```
  - ImmortalWrt installation command
    ```shell
    opkg install nexttrace
    ```

- macOS
  - macOS Homebrew's installation command
    - Homebrew-core version
      ```shell
      brew install nexttrace
      ```
    - `nxtrace/nexttrace` tap version (periodically synced from the latest NTrace-core release)
      ```shell
      brew tap nxtrace/nexttrace
      brew install nxtrace/nexttrace/nexttrace
      brew install nxtrace/nexttrace/nexttrace-tiny
      brew install nxtrace/nexttrace/ntr
      ```
    - The homebrew-core build is maintained by chenrui333, please note that this version's updates may lag behind the repository Action automatically version

- Windows
  - Windows WinGet installation command
    - WinGet version
      ```powershell
      winget install nexttrace
      ```
    - WinGet build maintained by Dragon1573

  - Windows Scoop installation command
    - Scoop-extras version
      ```powershell
      scoop bucket add extras && scoop install extras/nexttrace
      ```
    - Scoop-extra is maintained by soenggam

Please note:

- The `nexttrace-debs` APT repository is maintained by nxtrace and wcbing.
- Other package sources above are maintained by open-source enthusiasts. Availability and timely updates are not guaranteed. If you encounter problems, please contact the repository maintainer to solve them, or use the binary packages provided by the official build of this project.

### Manual Install

- Download the precompiled executable

  For users not covered by the above methods, please go directly to [Release](https://www.nxtrace.org/downloads) to download the compiled binary executable.
  - `Release` provides compiled binary executables for many systems and different architectures. If none are available, you can compile it yourself.
  - Some essential dependencies of this project are not fully implemented on `Windows` by `Golang`, so currently, `NextTrace` is in an experimental support phase on the `Windows` platform.

### Build Variants

Starting from this release, NextTrace is published in **three flavors** under the same tag. Choose the one that best fits your use case:

| Feature               | `nexttrace` (Full) | `nexttrace-tiny` |    `ntr`     |
| --------------------- | :----------------: | :--------------: | :----------: |
| Normal traceroute     |         ✅         |        ✅        |      —       |
| Environment check (`--doctor`) | ✅ | ✅ | ✅ |
| Standalone MTU (`--mtu`) |      ✅         |        ✅        |      —       |
| DNS client (`-l` / `--dns`) |     ✅       |        —         |      —       |
| CDN Speed (`--speed`) |         ✅         |        —         |      —       |
| IP annotation (`--nali`) |       ✅       |        —         |      —       |
| MTR TUI               |         ✅         |        ✅         | ✅ (default) |
| MTR report (`-r`)     |         ✅         |        ✅         |      ✅      |
| MTR wide (`-w`)       |         ✅         |        ✅         |      ✅      |
| MTR raw (`--raw`)     |         ✅         |        ✅         |      ✅      |
| MTR JSON / NDJSON | ✅ | ✅ | ✅ |
| MTR custom columns (`--mtr-columns`) | ✅ | ✅ | ✅ |
| MTR recording / replay (`--mtr-record` / `--mtr-replay`) | ✅ | ✅ | ✅ |
| Linux policy routing (`--fwmark`, local probes only) | ✅ | ✅ | ✅ |
| Globalping (`--from`) |         ✅         |        —         |      —       |
| WebUI (`--deploy`) / MCP (`--deploy --mcp`) |        ✅         |        —         |      —       |
| Fast Trace (`-F`)     |         ✅         |        ✅        |      —       |
| Default mode          |     traceroute     |    traceroute    |   MTR TUI    |
| Binary name           |    `nexttrace`     | `nexttrace-tiny` |    `ntr`     |

> **Note:** `APT (nexttrace-debs)` and the `Homebrew tap (nxtrace/nexttrace)` provide all three flavors: **Full** (`nexttrace`), **Tiny** (`nexttrace-tiny`), and **NTR** (`ntr`). `homebrew-core`, AUR, Scoop, and other package managers currently install the **Full** (`nexttrace`) version only.

### Feature Matrix

- **`nexttrace`** — Full-featured build. Includes traceroute, the standalone DNS client and MTU modes, CDN speed test, IP annotation, MTR, Globalping, Fast Trace, WebUI, and deploy MCP.
- **`nexttrace-tiny`** — Lightweight build. Keeps normal traceroute, standalone MTU, and Fast Trace. Includes optional MTR TUI, reports, and RAW. No DNS client / CDN speed test / IP annotation / Globalping / WebUI / MCP. Suitable for embedded or minimal environments.
- **`ntr`** — MTR-focused build. Runs MTR TUI by default. No normal traceroute mode, standalone DNS client or `--mtu`, CDN speed test, IP annotation, Globalping, Fast Trace, WebUI, or MCP.

### Manual Build

Build from source with Go 1.27.1+ installed. Release-equivalent builds use
`GOEXPERIMENT=nojsonv2`; the Go 1.27 default JSON runtime remains covered by CI,
but its WebSocket encoding cost does not meet the release performance threshold.
See the [JSON runtime ADR](docs/adr/0001-go127-json-runtime.md).

```bash
export GOTOOLCHAIN=go1.27.1
export GOEXPERIMENT=nojsonv2

# Full (all features)
go build -trimpath -o dist/nexttrace -ldflags "-w -s" .

# Tiny (optional MTR; no DNS client, no Globalping, no WebUI)
go build -tags flavor_tiny -trimpath -o dist/nexttrace-tiny -ldflags "-w -s" .

# NTR (MTR-only)
go build -tags flavor_ntr -trimpath -o dist/ntr -ldflags "-w -s" .
```

On macOS, source builds require macOS 13.0 or later, Xcode Command Line Tools, and cgo because TCP/UDP packet capture links against the system `libpcap`. For a native build, `go env CGO_ENABLED` must report `1`. If it reports `0`, remove any persistent override with `go env -u CGO_ENABLED`, ensure `xcode-select -p` succeeds, and build with `CGO_ENABLED=1`.

Cross-compile example:

```bash
# Linux arm64, Tiny flavor
GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=nojsonv2 \
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags flavor_tiny -trimpath -o dist/nexttrace-tiny_linux_arm64 -ldflags "-w -s" .
```

Benchmark tooling is pinned in `go.mod`; compare saved results with `go tool benchstat before.txt after.txt`.

The `tiny` and `ntr` flavors use **compile-time build tags** to exclude modules — this is not a runtime switch. You can verify with `go version -m <binary>` that `gin`, `globalping-cli`, and `github.com/natesales/q` are absent from `nexttrace-tiny` and `ntr`.

The `.cross_compile.sh` script supports building flavors:

```bash
./.cross_compile.sh all     # Build all three flavors for all platforms
./.cross_compile.sh full    # Build only nexttrace (Full)
./.cross_compile.sh tiny    # Build only nexttrace-tiny
./.cross_compile.sh ntr     # Build only ntr
```

### Release Assets Naming

Release binaries follow this naming convention:

```
{binary}_{os}_{arch}[v{arm}][.exe][_softfloat]
```

Examples:

- `nexttrace_linux_amd64`, `nexttrace-tiny_linux_amd64`, `ntr_linux_amd64`
- `nexttrace_darwin_universal`, `nexttrace-tiny_darwin_universal`, `ntr_darwin_universal`
- `nexttrace_windows_amd64.exe`, `ntr_windows_amd64.exe`

### Get Started

`NextTrace` uses the `ICMP` protocol to perform TraceRoute requests by default, which supports both `IPv4` and `IPv6`

```bash
# IPv4 ICMP Trace
nexttrace 1.0.0.1
# URL
nexttrace http://example.com:8080/index.html?q=1

# Table output (report mode): runs trace once and prints a final summary table
nexttrace --table 1.0.0.1

# Machine-readable RAW output: pipe-delimited lines
nexttrace --traceroute --raw 1.0.0.1

# JSON output: stdout is a single JSON document
nexttrace --json 1.0.0.1

# Realtime trace output to a custom file
nexttrace --output ./trace.log 1.0.0.1

# Realtime trace output to the default log file
nexttrace --output-default 1.0.0.1

# IPv4/IPv6 Resolve Only, and automatically select the first IP when there are multiple IPs
nexttrace --ipv4 g.co
nexttrace --ipv6 g.co

# IPv6 ICMP Trace
nexttrace 2606:4700:4700::1111

# Developer mode: set the ENV variable NEXTTRACE_DEVMODE=1 to make fatal errors panic with a stack trace
export NEXTTRACE_DEVMODE=1

# Set TTL-group interval in normal traceroute mode (default: 300ms)
nexttrace -i 300 1.1.1.1

# Disable Path Visualization With the -M parameter
nexttrace koreacentral.blob.core.windows.net
# MapTrace URL: https://api.nxtrace.org/tracemap/html/c14e439e-3250-5310-8965-42a1e3545266.html

# Disable MPLS display using the --disable-mpls / -e parameter or the NEXTTRACE_DISABLEMPLS environment variable
nexttrace --disable-mpls example.com
export NEXTTRACE_DISABLEMPLS=1
```

Normal traceroute reports why it stopped: destination reached, a terminal unreachable response (including its marker), or the configured maximum hop count. `--json` keeps the existing top-level result shape and adds optional `StopReason` with lowercase nested fields `hop`, `reason`, `responses`, and `markers`; `responses` contains human-readable descriptions while `markers` contains machine-readable codes. Classic/raw/JSON modes do not receive an extra human-readable footer. `--output` writes the same plain stop line to the log without ANSI escapes.

When multiple normal-trace output modes are selected, precedence is `--json` > `--table` > `--classic` > `--raw` > `--output` > realtime output. If a higher-priority mode overrides an explicit `--output` or `--output-default`, NextTrace reports that choice on stderr and does not create the ignored log file.

#### Probe environment check

```sh
nexttrace --doctor example.com
nexttrace --doctor --tcp --port 443 --language en example.com
nexttrace-tiny --doctor -6 ::1
ntr --doctor --dev eth0 example.com
```

`--doctor` generates a plain-text report and exits without tracing. It separates requested settings, DNS/source selection, system route predictions, and actual local backend initialization. It does not send probe packets, read captured traffic, query GeoIP/API services, or verify target reachability. DNS/DoT queries can use the network.

All three builds support `--doctor`. Options are limited to `--ipv4`/`--ipv6`, `--tcp`/`--udp`, `--port`, `--source`, `--source-port`, `--dev`, `--tos`, `--dot-server`, `--timeout`, `--language`, `--no-color`, and Windows `--icmp-mode`, plus their listed short aliases. Use `--doctor --help` for details. The target must be a domain or IP literal; URLs and IPv6 zone suffixes are not accepted. Doctor is incompatible with other execution modes, JSON/RAW/file output, and unrelated probe options. It rejects `--fwmark`, so it cannot verify marked routing or `SO_MARK` privileges. Doctor accepts source ports `0..65535`, not the probe mode's random value `-1`.

The default is ICMP, Chinese text, and 5000 ms per network check. Multiple DNS candidates are listed; the first matching the requested family is selected without prompting. Target DoT failures do not fall back to system DNS. A failed check does not prevent independent checks from completing. The report goes to stdout; usage and output-write errors go to stderr. Reports include target/source/interface addresses, but no token or proxy credentials.

Route queries use Linux netlink, macOS routing sockets, and Windows `GetBestRoute2`. Linux IPv4 UDP instead selects its source with a raw socket `connect` without transmitting: netlink cannot query its kernel protocol 255. This exposes the source only; `--doctor` leaves the route interface and gateway unknown. Unavailable information remains unknown. macOS/Windows route predictions cannot express all protocol, port, source-policy or TOS constraints; the report states these limits. Opening a socket or capture filter does not establish actual egress, successful packet delivery, or TOS application. Network waits have deadlines; synchronous native initialization calls have no forced-cancellation guarantee.

On Windows, `--dev` selects a source address; it does not prove device binding. Every doctor WinDivert open uses `NO_INSTALL`: it does not unpack/install a driver or run `--init`. A not-yet-installed driver is reported as unverified, since ordinary tracing may install it. Socket alternatives are reported separately. The reported backend follows the build architecture; WinDivert probe paths are currently compiled for Windows amd64.

Exit codes: **0** required checks completed without failure, **1** definite check failure, **2** invalid arguments, **3** required evidence incomplete, **130/143** interrupted by SIGINT/SIGTERM. Unverified target reachability alone does not make the exit code 3.

#### Downstream migration and explicit traditional mode

`-k/--traceroute` selects traditional traceroute in full and tiny builds; it is not an alias for the `--classic` layout. It supports existing traditional output formats, Fast Trace, and file targets; the full build also supports Globalping. Combining it with `--mtr/-t`, `--report/-r`, `--wide/-w`, or standalone DNS, MTU, speed, IP annotation, or deploy mode is an error.

```bash
nexttrace --traceroute example.com
nexttrace -k example.com
nexttrace --traceroute --raw example.com
nexttrace --mtr --raw -q 10 example.com
```

| Invocation | Today | After the default switch |
|---|---|---|
| Target only | Traditional traceroute | MTR |
| `--raw` | Traditional RAW | MTR RAW |
| `--traceroute` | Traditional traceroute | Traditional traceroute |
| `--traceroute --raw` | Traditional RAW | Traditional RAW |
| `--json/--table/--classic/--output/--output-default/--route-path` | Traditional mode | Selects traditional mode automatically |

Standalone entry points, Fast Trace, file targets, and Globalping (full build only) keep their workflows; `--json` in a standalone mode still belongs to that mode. Explicit MTR continues to reject traditional-only output formats.

When `-q` is omitted, MTR TUI and `--mtr --raw` run continuously; `--report/-r` and `--wide/-w` default to 10 probes per hop, including when combined with `--raw`. Automation should set an explicit `-q`. In MTR, `-i` is the per-hop interval (default 1000ms) and `-z` is ignored. In traditional traceroute, `-q` defaults to 3 samples per hop, `-i` is the TTL-group interval (default 300ms), and `-z` is the packet interval (default 50ms).

Do not distinguish the RAW modes by successful rows alone: traditional RAW uses 12 columns for success and preserves historical 8-column timeout rows; MTR RAW uses fixed 12-column data rows in a continuous event stream. Parsers must also honor each mode's existing information headers and stderr behavior. `NEXTTRACE_UNINTERRUPTED` retains its traditional loop behavior with `--traceroute --raw`.

Bash wrappers supporting older releases can detect the capability before invoking a trace:

```bash
trace_mode=()
nexttrace_help=$(nexttrace --help)
if [[ "$nexttrace_help" == *"--traceroute"* ]]; then
  trace_mode=(--traceroute)
fi
nexttrace "${trace_mode[@]}" --raw example.com
```

Omit the new flag on older releases. Do not retry a failed trace without the flag: that could run the measurement twice.


PS: The route visualization module is an independent component, You can find its source code at [nxtrace/traceMap](https://github.com/nxtrace/traceMap).  
The routing visualization function requires the geographical coordinates of each hop. It is currently available with NextTrace API, IPInfo, and IP-API.com.

#### Mandatory Configuration Steps for `Windows` Users

- **For Normal User Mode:**  
  Only **ICMP mode** can be used, and the firewall must allow `ICMP/ICMPv6` traffic.
  ```powershell
  netsh advfirewall firewall add rule name="All ICMP v4" dir=in action=allow protocol=icmpv4:any,any
  netsh advfirewall firewall add rule name="All ICMP v6" dir=in action=allow protocol=icmpv6:any,any
  ```
- **For Administrator Mode:**  
  **TCP/UDP mode** requires `WinDivert`.  
  **ICMP mode** supports `1=Socket` and `2=WinDivert` (`0=Auto`). If running in Socket mode, the firewall must allow `ICMP/ICMPv6`.  
  On `Windows amd64`, ICMPv4 and ICMPv6 without `--tos` (or with `--tos 0`) keep using the native Socket send path. Nonzero ICMP `--tos` uses `WinDivert` to preserve the complete field and requires administrator privilege, including when `--icmp-mode 1` selects socket reception. Native Windows ICMPv4 was observed sending zero for every tested nonzero TOS value.
  `WinDivert` can be automatically configured using the `--init` parameter, which extracts the runtime to the executable directory.

#### `NextTrace` now supports quick testing, and friends who have a one-time backhaul routing test requirement can use it

```bash
# IPv4 ICMP Fast Test (Beijing + Shanghai + Guangzhou + Hangzhou) in China Telecom / Unicom / Mobile / Education Network
nexttrace --fast-trace

# You can also use TCP SYN for testing
nexttrace --fast-trace --tcp

# You can also quickly test through a customized IP/DOMAIN list file
nexttrace --file /path/to/your/iplist.txt
# CUSTOMIZED IP DOMAIN LIST FILE FORMAT
## One IP/DOMAIN per line + space + description information (optional)
## forExample:
## 106.37.67.1 BEIJING-TELECOM
## 240e:928:101:31a::1 BEIJING-TELECOM
## bj.10086.cn BEIJING-MOBILE
## 2409:8080:0:1::1
## 223.5.5.5
```

#### `NextTrace` already supports route tracing for specified Network Devices

### TOS / IPv6 Traffic Class (`-Q`, `--tos`)

`--tos` sets the complete 8-bit IPv4 TOS / IPv6 Traffic Class field, from `0` to `255`, defaulting to `0`. Compute it as `DSCP * 4 + ECN`: DSCP 46 with ECN 0 is `--tos 184`; `--tos 46` still means the raw field value 46 (DSCP 11, ECN 2). The field is part of the probe IP header; routing and priority depend on network policy.

Linux and macOS senders use native socket options or complete IP headers. On Linux, nonzero TOS selects the automatic source address using the corresponding route, including combinations with `--fwmark`. Explicit `--source` and `--dev` remain constraints, and source selection is local to each session. Route queries match the raw socket lookup and omit transport ports serialized in user space. Source-selection or TOS-configuration failures terminate probing instead of becoming MTR loss or falling back to a default field value.

Existing traceroute, MTR, Fast Trace, file-target and Web/API/MCP probe entrypoints support TOS. DNS/RDNS, GeoIP and API helper traffic do not inherit it. Standalone MTU and Globalping do not support this option. The JSON `tos` field records the requested configuration, not a packet-capture observation.

BSD and Android expose the native options but have not completed this native packet-capture acceptance; probe permissions and system restrictions still apply. The Windows WinDivert send backend is currently compiled only for amd64, so its support does not imply Windows arm64 support.

### Linux policy routing (`--fwmark`)

`--fwmark 256` or `--fwmark 0x100` sets a 32-bit socket mark on local traceroute and MTR probes. It supports ICMP/TCP/UDP over IPv4/IPv6 in all applicable builds. Configure matching Linux `ip rule` and route tables separately; NextTrace does not edit them. The same mark need not change the route when no rule selects a different path.

Automatic source selection uses the marked route. Explicit `--source` and `--dev` remain constraints; a mark does not override them. Marked sessions do not use the legacy process-wide source cache. The query includes the kernel send protocol and TOS (IPv4 UDP with IP_HDRINCL uses protocol 255), but omits TCP/UDP ports to match the native raw sockets: their kernel route lookup does not see the transport headers serialized in user space. Later policy changes and ECMP can still affect path selection.

Values range from `0` to `4294967295`; decimal and `0x` hexadecimal are accepted, without masks. Omission leaves socket marking untouched; explicit `0` sets zero and still requires permission. Linux requires `CAP_NET_ADMIN`, or `CAP_NET_RAW` on Linux 5.17 and newer. Mark initialization failure terminates probing rather than falling back to unmarked traffic.

DNS/RDNS, GeoIP and API requests do not inherit the probe mark. Human-readable and RAW output layouts are unchanged; MTR JSON/NDJSON records an explicitly supplied mark in `effective_parameters.fwmark` as a JSON number. macOS, Windows, BSD and Android reject this option. Independent modes (including doctor/MTU/DNS/speed/deploy), Globalping, Fast Trace and file targets do not support it.

On macOS and Linux, `--dev` binds the requested source interface.
On Windows, `--dev` resolves the source IP from the selected device and uses that source address for ICMP/TCP/UDP probes; it does not bind WinDivert or sockets to a real egress interface, so Windows routing may still choose a different path. The standalone `--mtu` mode follows the same source-address behavior and also uses the device name for local MTU lookup.

```bash
# Use eth0 network interface
nexttrace --dev eth0 2606:4700:4700::1111

# Use eth0 network interface's IP
# When using the network interface's IP for route tracing, note that the IP type to be traced should be the same as network interface's IP type (e.g. both IPv4)
nexttrace --source 204.98.134.56 9.9.9.9
```

#### `NextTrace` can also use `TCP` and `UDP` protocols to perform `Traceroute` requests

```bash
# TCP SYN Trace
nexttrace --tcp www.bing.com

# You can specify the port by yourself [here is 443], the default port is 80
nexttrace --tcp --port 443 2001:4860:4860::8888

# UDP Trace
nexttrace --udp 1.0.0.1

# You can specify the target port yourself [here it is 5353], the default is port 33494
nexttrace --udp --port 5353 1.0.0.1

# For TCP/UDP Trace, you can specify the source port; by default, a fixed random port is used
# (If you need to use a different random source port for each packet, please set the ENV variable NEXTTRACE_RANDOMPORT, or set the source port to -1; this also applies to MTR text, JSON and recording)
nexttrace --tcp --source-port 14514 www.bing.com
```

#### `NextTrace` also supports a standalone DNS client mode

The full `nexttrace` flavor provides a q-compatible DNS client through a NextTrace-owned adapter over the public packages of [natesales/q v0.19.12](https://github.com/natesales/q/releases/tag/v0.19.12). `-l` / `--dns` must be the first argument; everything after it uses q-style flags and positional arguments.

```bash
# Query MX records through an explicit plain DNS server
nexttrace -l example.com MX @1.1.1.1

# Query A records over DNS-over-TLS
nexttrace --dns example.com A @tls://one.one.one.one

# Query over DNS-over-HTTPS and emit JSON
nexttrace --dns example.com A @https://cloudflare-dns.com/dns-query --format=json

# Show the dedicated DNS client help
nexttrace --dns --help
```

- Transports: UDP/TCP, DoT, DoH, DoQ, ODoH, and DNSCrypt; DNS Stamp server forms are supported for plain DNS, DoT, DoH, and DNSCrypt.
- Output formats: `pretty`, `column`, `raw`, `json`, and `yaml`.
- Query features include multiple servers and RR types, reverse lookup, DNSSEC/EDNS, NSID, PTR lookups for A/AAAA answers, and recursive AXFR.
- Configuration follows q conventions for `~/.qrc`, `Q_DEFAULT_SERVER`, `NO_COLOR`, and `SSLKEYLOGFILE`.
- This mode exists only in the full `nexttrace` flavor. `nexttrace-tiny` and `ntr` do not include or register it.
- It is a standalone CLI workflow: it does not replace the DNS resolver used by traceroute, GeoIP/RDNS, WebUI, MCP, or other service paths.

#### `NextTrace` also supports standalone path-MTU discovery mode

```bash
# Tracepath-style UDP PMTU discovery with live hop output
nexttrace --mtu 1.1.1.1

# Reuse the normal GeoIP / RDNS knobs in mtu mode
nexttrace --mtu --data-provider IPInfo --language en 1.1.1.1

# JSON output keeps the standalone mtu schema and now includes hop.geo
nexttrace --mtu --json 1.1.1.1
```

- `--mtu` is an independent UDP-only mode. It does not reuse the normal traceroute engine.
- TTY output updates the current hop in place and adds color for hop state / PMTU highlights; redirected / piped output falls back to finalized line-by-line streaming without ANSI.
- `--mtu --json` prints only the standalone MTU JSON document on stdout.
- GeoIP, RDNS, `--data-provider`, `--language`, `--no-rdns`, `--always-rdns`, and `--dot-server` all apply to this mode.

#### `NextTrace` also supports standalone CDN speed testing mode

```bash
# Apple CDN backend (default)
nexttrace --speed

# Cloudflare backend
nexttrace --speed --speed-provider cloudflare

# Dedicated speed help
nexttrace --speed --help

# Machine-readable output
nexttrace --speed --json --non-interactive --no-metadata

# Pin to a specific candidate IP, or bind a source address / device
nexttrace --speed --endpoint 1.2.3.4
nexttrace --speed --source 192.0.2.10
nexttrace --speed --dev eth0
```

- `--speed` is available only in the full `nexttrace` flavor. `nexttrace-tiny` and `ntr` do not register it.
- Main `nexttrace --help` only exposes the top-level `--speed` entry. Detailed speed flags live under `nexttrace --speed --help`.
- Backends: `apple` (default) and `cloudflare`.
- Reused common flags: `--json`, `--language`, `--no-color`, `--dot-server`, `--timeout`, `--source`, `--dev`.
- Speed-specific flags: `--speed-provider`, `--max`, `--threads`, `--latency-count`, `--non-interactive`, `--endpoint`, `--no-metadata`.
- Default terminal output includes candidate endpoints, the selected endpoint, client/server metadata, idle latency, download/upload single-thread and multi-thread rounds, loaded latency, total traffic, warnings, and degraded status.
- `--json` prints exactly one JSON document to stdout.
- Exit codes: `0` = success, `2` = degraded completion, `1` = failure, `130` = interrupted.

#### `NextTrace` can annotate IP literals in text streams

```bash
# Annotate a single line
nexttrace --nali 1.1.1.1

# Annotate pipeline output
dig example.com +short | nexttrace --nali --data-provider IPInfo --language en
```

- `--nali` is available only in the full `nexttrace` flavor. `nexttrace-tiny` and `ntr` do not register it.
- It only annotates IPv4/IPv6 literals and reuses NextTrace GeoIP providers. CDN/CNAME matching, offline databases, update logic, and nali-specific paths are not bundled.
- Reused common flags: `--data-provider`, `--language`, `--dot-server`, `--timeout`, `--dn42`, `-4`, and `-6`.
- This text annotation mode is inspired by [zu1k/nali](https://github.com/zu1k/nali), which is licensed under the [MIT License](https://github.com/zu1k/nali/blob/master/LICENSE).

#### `NextTrace` also supports some advanced functions, such as ttl control, concurrent probe packet count control, mode switching, etc.

```bash
# Display 2 latency samples per hop
nexttrace --queries 2 www.hkix.net

# Allow up to 10 probe packets per hop to collect those samples
# (NextTrace stops earlier if it has already got the replies requested by --queries)
nexttrace --max-attempts 10 www.hkix.net
# or use the ENV variable NEXTTRACE_MAXATTEMPTS to persist across runs
export NEXTTRACE_MAXATTEMPTS=10

# No concurrent probe packets, only one probe packet is sent at a time
nexttrace --parallel-requests 1 www.hkix.net

# Start Trace with TTL of 5, end at TTL of 10
nexttrace --first 5 --max-hops 10 www.decix.net
# In addition, an ENV is provided to set whether to mask the destination IP and omit its hostname
export NEXTTRACE_ENABLEHIDDENDSTIP=1

# Turn off the IP reverse parsing function
nexttrace --no-rdns www.bbix.net

# Set the probe packet size to 1024 bytes (inclusive IP + probe headers)
nexttrace --psize 1024 example.com

# Randomize each probe packet size up to 1500 bytes
nexttrace --psize -1500 example.com

# Set DSCP 46 with ECN 0 (complete TOS / traffic class value 184)
nexttrace -Q 184 example.com

# Feature: print Route-Path diagram
# Route-Path diagram example:
# AS6453 Tata Communication「Singapore『Singapore』」
#  ╭╯
#  ╰AS9299 Philippine Long Distance Telephone Co.「Philippines『Metro Manila』」
#  ╭╯
#  ╰AS36776 Five9 Inc.「Philippines『Metro Manila』」
#  ╭╯
#  ╰AS37963 Aliyun「ALIDNS.COM『ALIDNS.COM』」
nexttrace --route-path www.time.com.my

# Disable color output
nexttrace --no-color 1.1.1.1
# or use ENV
export NO_COLOR=1
```

#### Advanced tuning quick guide

| Flag | What it controls | Default / starting point | When to change it |
| --- | --- | --- | --- |
| `--queries` | Samples per hop in normal traceroute; explicit probe count per hop in MTR | traceroute: `3`; MTR report/wide (including RAW): `10` when omitted; otherwise MTR TUI/raw: unlimited when omitted | Raise to `5-10` on unstable paths |
| `--max-attempts` | Hard cap on probe packets per hop | auto-sized from `--queries` | Raise on lossy links when replies arrive slowly |
| `--parallel-requests` | Total in-flight probes across TTLs | `18` | Use `1` on multipath/load-balanced paths; keep `6-18` on stable links |
| `--send-time` | Gap between packets inside one TTL group | `50ms` | Raise to `100-200ms` on rate-limited devices; ignored in MTR |
| `--ttl-time` | Gap between TTL groups in traceroute; per-hop interval in MTR | traceroute: `300ms`; MTR: `1000ms` when omitted | Lower to speed up; raise on remote/rate-limited paths |
| `--timeout` | Per-probe timeout | `1000ms` | Raise to `2000-3000ms` for intercontinental or high-loss paths |
| `--psize` | Probe packet size | Protocol/IP-family minimum | Inclusive IP + probe headers; negative values randomize each probe up to `abs(value)`; sizes above the egress/path MTU may fragment on wire |
| `-Q`, `--tos` | IP TOS / traffic class | `0` | Set the full 8-bit IP field (DSCP*4+ECN); nonzero ICMP on Windows amd64 requires `WinDivert` |

These probe knobs are CLI-only today; `nt_config.yaml` does not yet store them. If you want reusable profiles, keep them in shell aliases or small wrapper scripts.

```bash
# Conservative profile for multipath or ECMP networks
nexttrace --parallel-requests 1 --send-time 100 --ttl-time 500 --timeout 2000 example.com

# Faster profile for stable single-path networks
nexttrace --parallel-requests 18 --send-time 20 --ttl-time 150 example.com

# Lossy long-haul profile
nexttrace --queries 5 --max-attempts 10 --timeout 2500 example.com
```

#### `NextTrace` supports MTR (My Traceroute) continuous probing mode

```bash
# MTR mode: continuous probing with ICMP (default), refreshes table in real-time
nexttrace -t 1.1.1.1
# or equivalently:
nexttrace --mtr 1.1.1.1

# MTR mode with TCP SYN probing
nexttrace -t --tcp --port 443 www.bing.com

# MTR mode with UDP probing
nexttrace -t --udp 1.0.0.1

# Set per-hop probe interval (default: 1000ms in MTR; -z/--send-time is ignored in MTR mode)
nexttrace -t -i 500 1.1.1.1

# Limit the max probes per hop (default: infinite in TUI, 10 in report mode)
nexttrace -t -q 20 1.1.1.1

# Report mode: probe each hop N times then print a final summary (like mtr -r)
nexttrace -r 1.1.1.1       # = --mtr --report, 10 probes per hop by default
nexttrace -r -q 5 1.1.1.1  # 5 probes per hop

# Wide report: no host column truncation (like mtr -rw)
nexttrace -w 1.1.1.1       # = --mtr --report --wide

# Show PTR and IP together (PTR first, IP in parentheses) in MTR output
nexttrace --mtr --show-ips 1.1.1.1
nexttrace -r --show-ips 1.1.1.1
nexttrace -w --show-ips 1.1.1.1

# MTR raw stream mode (machine-friendly, one event per line)
nexttrace --mtr --raw 1.1.1.1
nexttrace -r --raw 1.1.1.1

# Combine with other options
nexttrace -t --tcp --max-hops 20 --first 3 --no-rdns 8.8.8.8
```

MTR JSON is available in all three builds:

```bash
nexttrace --mtr --json 1.1.1.1        # continuous NDJSON events
nexttrace --mtr --json -q 10 1.1.1.1  # finite NDJSON events
nexttrace -r --json -q 10 1.1.1.1     # one final JSON report
nexttrace -w --json -q 10 1.1.1.1     # identical to -r --json
ntr --json 1.1.1.1                    # continuous NDJSON events
```

JSON always includes all available metadata (FULL), ignores `-y`, and honors Geo/PTR/provider/language settings. `-r/-w --json` use the same wide collection rules. Streams run indefinitely when `-q` is omitted or nonpositive; JSON reports default to 10 probes per hop and reject explicit nonpositive counts. `--raw` and MTR `--json` are mutually exclusive. Bare `--json` in full/tiny retains traditional traceroute JSON, including after a future default-mode switch.

NDJSON emits `start`, `probe`, `path_end`, and `end` objects with consecutive `seq` values. Reports emit exactly one object, including partial statistics on interruption or failure. Diagnostics go to stderr. Exit codes: completion `0`, runtime/initialization error `1`, invalid arguments `2`, SIGINT `130`, SIGTERM `143`. Completion does not imply reachability; use `path_end`. See the [MTR JSON v1 contract and examples](docs/mtr-json.md).

Save and reopen an MTR session in any build:

```sh
nexttrace --mtr --mtr-record session.jsonl 1.1.1.1
nexttrace --mtr-replay session.jsonl
nexttrace --mtr-replay session.jsonl -r --json
```

`--mtr-record` creates a new private file alongside the selected TUI, report, RAW or JSON output. It does not enable MTR by itself; full/tiny require `-t`, `-r` or `-w`, while ntr uses its default mode. Existing files are never overwritten. A recording write failure stops probing and returns an error, preserving the written prefix. Later recording start/finish errors do not replace the first session error or its stage.

Replay uses recorded probe results and metadata without probing or DNS/Geo/PTR queries. In a terminal it opens paused at the final statistics; Space plays at original speed (from the beginning at EOF), `p` pauses playback, `r` rewinds, and `j/J` seeks to an elapsed `HH:MM:SS[.mmm]`. Existing host, column and history controls remain available. Non-TTY and `-r/-w` output one report; `--json` emits a separate offline report with recording completeness and playback position. Truncated tails are recoverable, explicitly marked incomplete, and return nonzero. The three-minute history window follows the playback position; the file retains the whole recorded session. See the [session format and recovery contract](docs/mtr-session.md).

Select and reorder human-readable MTR columns:

```sh
nexttrace -t --mtr-columns loss,received,avg 1.1.1.1
nexttrace -w --mtr-columns received,snt,last 1.1.1.1
ntr --mtr-columns received 1.1.1.1
```

`--mtr-columns` accepts any nonempty selection of `loss,snt,received,last,avg,best,wrst,stdev,dropped,gmean,jitter,javg,jmax,jint,space`, in the supplied order. Names ignore case and surrounding spaces; unknown names, duplicate metrics and empty entries are errors. `space` adds one display space and may repeat; at least one metric is required. `received` is displayed as `Rcv`. The default remains `Loss%, Snt, Last, Avg, Best, Wrst, StDev`.

The option applies to TUI, non-TTY tables and report/wide output, including offline replay text output. It does not enable MTR: full/tiny require `-t`, `-r` or `-w`; ntr uses its default MTR mode. RAW, JSON, traditional traceroute and other standalone modes reject it before initialization. Custom TUI columns keep complete numbers and at least 8 Host cells; a narrow terminal shows a notice until widened or fewer columns are selected.

Press `o/O` to edit the current column codes: `L=Loss D=Drop R=Received S=Snt N=Last B=Best A=Avg W=Wrst V=StDev G=Gmean J=Jttr M=Javg X=Jmax I=Jint`. Codes ignore case; each space adds one display space. Leading, trailing and repeated spaces are preserved. Enter validates and applies, Esc cancels, Backspace deletes and Ctrl-U clears. Invalid or duplicate metric codes, an empty draft and a spaces-only draft keep the editor open. Bracketed paste converts newlines to spaces without submitting. The draft is limited to 256 ASCII characters.

The `Fields:` page lists every code on a separate line. See [MTR column metrics](docs/mtr-columns.md) for formulas, spacing examples and JSON fields.

While editing, other shortcuts are inactive and Ctrl-C still exits. Editing does not pause probes, reset counters or change the paused state. Applying a selection from history view returns to the statistics table; cancellation preserves the view. History columns stay fixed. Changes last only for this session; editing and resizing work while paused.

When running in a terminal (TTY), MTR mode uses an **interactive full-screen TUI**:

- **`q` / `Q`** — quit (restores terminal, no output left behind)
- **`p`** — pause probing
- **`SPACE`** — resume probing
- **`r`** — reset statistics (counters are cleared, display mode is preserved)
- **`y`** — cycle host display mode: IP/PTR → ASN → City → Owner → Full
- **`n`** — toggle host name display:
  - default: PTR (or IP fallback) ↔ IP only
  - with `--show-ips`: PTR (IP) ↔ IP only
- **`e`** — toggle MPLS label display on/off
- **`o` / `O`** — edit statistic columns
- **`d` / `D`** — toggle the optional history display; the default TUI remains the classic metric table
- **`g` / `G`** — in history display only, cycle History chart mode: heatmap → bars → sparkline
- The TUI header displays **source → destination**, with `--source`/`--dev` information when specified.
- When using NextTrace API and preferred API metadata is available, the preferred API IP address is shown in the header.
- Uses the **alternate screen buffer**, so your previous terminal history is preserved on exit.
- When stdin is not a TTY (e.g. piped), it falls back to a simple table refresh.

History display keeps a rolling 3-minute, timestamp-based probe history while the classic table is shown, then renders `Host`, `Last`, `Avg`, `Loss`, and `History` when toggled with `d`. The History column uses a fixed 100ms latency scale. Unicode blocks/sparklines are used by default; with `--no-color`, plain ASCII is used and timeouts are shown as `x`.

Acknowledgement: the optional MTR history display is inspired by [TraceBar](https://github.com/tracebar-app/tracebar), a macOS continuous traceroute monitor licensed under the [MIT License](https://github.com/tracebar-app/tracebar/blob/main/LICENSE).

The **report mode** (`-r`/`--report`) produces a one-shot summary after all probes complete, suitable for scripting:

```text
Start: 2025-07-14T09:12:00+08:00
HOST: myhost                    Loss%   Snt   Last    Avg   Best   Wrst  StDev
  1. one.one.one.one            0.0%    10    1.23   1.45   0.98   2.10   0.32
  2. 10.0.0.2                 100.0%    10    0.00   0.00   0.00   0.00   0.00
```

Rows shown as `(waiting for reply)` keep the same table layout; the metric cells on that row are left blank.

In non-wide report mode, NextTrace intentionally keeps the host column compact:

- only `PTR/IP` is shown
- no Geo API lookup is performed
- no ASN / owner / location fields are shown
- MPLS labels are hidden

Wide report mode (`-w` / `--wide`) keeps the current full-information behavior, including Geo-derived fields and MPLS output.

When `--raw` is used together with MTR (`--mtr`, `-r`, or `-w`), NextTrace enters **MTR raw stream mode**.

If the active data provider is `NextTrace-API` and preferred API metadata is available, NextTrace first prints one uncolored API info preamble line:

```text
[NextTrace API] preferred API IP - [2403:18c0:1001:462:dd:38ff:fe48:e0c5] - 21.33ms - DMIT.NRT
```

After that, it prints one `|`-delimited event per line:

```
4|84.17.33.106|po66-3518.cr01.nrt04.jp.misaka.io|0.27|60068|Japan|Tokyo|Tokyo||cdn77.com|35.6804|139.7690
```

Field order:

`ttl|ip|ptr|rtt|asn|country|prov|city|district|owner|lat|lng`

Timeout rows keep the same 12-column layout:

`ttl|*||||||||||`

The raw stdout contract remains exactly 12 columns. An unreachable edge in unbounded MTR is provisional: later transit evidence can reopen higher hops, and a new unreachable edge may therefore produce another stderr diagnostic. For bounded runs, the final structured `path_end` is authoritative. Structured Web/API/MCP records expose per-probe `response` and the final `path_end` instead of inferring the edge from responder-IP equality.

In MTR mode (`--mtr`, `-r`, `-w`, including `--raw`), `-i/--ttl-time` sets the **per-hop probe interval**: how long to wait between successive probes to the same hop (default: 1000ms when omitted). `-z/--send-time` is ignored in MTR mode.

> Note: `--show-ips` only takes effect in MTR mode (`--mtr`, `-r`, `-w`); otherwise it is ignored.
>
> Note: `--mtr` cannot be used together with `--traceroute`, `--table`, `--classic`, `--output`, `--output-default`, `--route-path`, `--from`, `--fast-trace`, `--file`, or `--deploy`.

#### `NextTrace` supports users to select their own IP API (currently supports: `NextTrace-API`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`, `IPInfoLocal`, `IPDB.One`, `CHUNZHEN`, `DN42`)

##### LeoMoeAPI name migration

`LeoMoeAPI` is the retired name of the project's official API. The current name is **NextTrace API**, and its machine-readable `data_provider` value is `NextTrace-API`. The former `LeoMoeAPI` and `LeoMoe` values remain accepted as silent, case-insensitive compatibility aliases for existing scripts, but NextTrace always reports the canonical value. The WebSocket/PoW implementation is called **NextTrace API v3**, while the token-authenticated HTTP implementation is called **NextTrace API v4**.

```bash
# You can specify the IP database by yourself [IP-API.com here]; NextTrace API is used by default
nexttrace --data-provider ip-api.com
## Note There are frequency limits for free queries of the ipinfo and IPInsight APIs. You can purchase services from these providers to remove the limits
##      If necessary, you can clone this project, add the token provided by ipinfo or IPInsight and compile it yourself
##      Fill the token to: ipgeo/tokens.go

## Note For the offline database IPInfoLocal, please download it manually and rename it to ipinfoLocal.mmdb. (You can download it from here: https://ipinfo.io/signup?ref=free-database-downloads)
##      Current directory, nexttrace binary directory and FHS directories (Unix-like) will be searched.
##      To customize it, please use environment variables,
export NEXTTRACE_IPINFOLOCALPATH=/xxx/yyy.mmdb
## Please be aware: Due to the serious abuse of IP.SB, you will often be not able to query IP data from this source
## IP-API.com has a stricter restiction on API calls, if you can't query IP data from this source, please try again in a few minutes

# The Pure-FTPd IP database defaults to using http://127.0.0.1:2060 as the query interface. To customize it, please use environment variables
export NEXTTRACE_CHUNZHENURL=http://127.0.0.1:2060
## You can use https://github.com/freshcn/qqwry to build your own Pure-FTPd IP database service

# You can also specify the default IP database by setting an environment variable
export NEXTTRACE_DATAPROVIDER=ipinfo
```

NextTrace API v3 WebSocket/PoW remains the default when no NextTrace API v4 token is available. To use the NextTrace API v4 HTTP GeoIP endpoint for the current shell session, run the setup command and paste your token:

```bash
# Token page:
# GET https://api.nxtrace.org/v4/api-tokens

nexttrace -x
```

`nexttrace -x` stores the token in temporary files: one scoped to the parent process ID, which is normally your current shell, and one same-user fallback file for wrapper commands such as `go run`. Later `nexttrace` commands first read the real `NEXTTRACE_API_V4_TOKEN`, then the parent-PID file, then the fallback file, and load the value into the process-local environment. The command does not write shell profiles, permanent environment variables, or `nt_config.yaml`.

With `NEXTTRACE_API_V4_TOKEN` set and the active provider still `NextTrace-API`, NextTrace queries `GET https://api.nxtrace.org/v4/ipGeo?ip=<ip>` with `X-NextTrace-Token: <token>`. The request has no JSON body. Successful responses are direct GeoIP JSON mapped to the normal output fields; quota metadata is exposed only in headers (`X-NextTrace-Quota-Remaining`, `X-NextTrace-Quota-Expires-At`, `X-NextTrace-Quota-Cost`, `X-NextTrace-Quota-Source`) and does not change the default output format. Error responses prefer `{"error":{"message":"..."}}`; known statuses include `400` for empty/illegal IP, `401` unauthorized, `429` quota exhausted, and `500` internal server error. NextTrace API v4 token failures do not fall back to NextTrace API v3.

#### `NextTrace` supports mixed parameters and shortened parameters

```bash
Example:
nexttrace --data-provider IPAPI.com --max-hops 20 --tcp --port 443 --queries 5 --no-rdns 1.1.1.1
nexttrace -tcp --queries 2 --parallel-requests 1 --table --route-path 2001:4860:4860::8888

Equivalent to:
nexttrace -d ip-api.com -m 20 -T -p 443 -q 5 -n 1.1.1.1
nexttrace -T -q 2 --parallel-requests 1 --table -P 2001:4860:4860::8888
```

### Globalping

[Globalping](https://globalping.io/) provides access to thousands of community-hosted probes to run network tests and measurements.

Run traceroute from a specified location by using the `--from` flag. The location field accepts continents, countries, regions, cities, ASNs, ISPs, or cloud regions.

```bash
nexttrace google.com --from Germany
nexttrace google.com --from comcast+california
```

A limit of 250 tests per hour is set for all anonymous users. To double the limit to 500 per hour please set the `GLOBALPING_TOKEN` environment variable with your token.

```bash
export GLOBALPING_TOKEN=your_token_here
```

### IP Database

We use [bgp.tools](https://bgp.tools) as a data provider for routing tables.

NextTrace BackEnd is now open-source.

https://github.com/sjlleo/nexttrace-backend

NextTrace API v3 utilizes the Proof of Work (PoW) mechanism to prevent abuse, with NextTrace using the powclient library as its client component. Both the PoW client and server are open source. Please direct PoW-related questions to their respective repositories.

- [GitHub - tsosunchia/powclient: Proof of Work CLIENT for NextTrace](https://github.com/tsosunchia/powclient)
- [GitHub - tsosunchia/powserver: Proof of Work SERVER for NextTrace](https://github.com/tsosunchia/powserver)

All NextTrace IP geolocation `API DEMO` can refer to [here](https://github.com/nxtrace/NTrace-core/blob/main/ipgeo/)

### Environment Variables

With `NEXTTRACE_DEBUG`, environment-read logs for known tokens, IPDB credentials and proxy URLs show only the variable name and presence, never their values. Other diagnostics can still contain target/source addresses and interface names.

NextTrace currently reads the following environment variables. For `NEXTTRACE_*` boolean switches, only `1` and `0` are recognized; other values fall back to the built-in default. For consistency, restart NextTrace after changing them.

#### Core Runtime / Network

| Variable | Default | Description |
| --- | --- | --- |
| `NEXTTRACE_DEVMODE` | `0` | Turn fatal errors into panics with stack traces for debugging. |
| `NEXTTRACE_DEBUG` | unset | Print detected environment values while `GetEnv*` helpers parse them. |
| `NEXTTRACE_DISABLEMPLS` | `0` | Disable MPLS display globally, similar to `--disable-mpls`. |
| `NEXTTRACE_ENABLEHIDDENDSTIP` | `0` | Mask the destination IP and omit its hostname in output. |
| `NEXTTRACE_RANDOMPORT` | `0` | Use a different random source port for each TCP/UDP probe. |
| `NEXTTRACE_MAXATTEMPTS` | auto | Provide a default `--max-attempts` value when the CLI flag is not set. |
| `NEXTTRACE_ICMPMODE` | `0` | Provide a default `--icmp-mode` value (`0=auto`, `1=socket`, `2=WinDivert` on Windows). |
| `NEXTTRACE_UNINTERRUPTED` | `0` | When used together with `--raw`, rerun traceroute continuously instead of stopping after one round. |
| `NEXTTRACE_PROXY` | unset | Outbound proxy URL for HTTP / WebSocket requests used by PoW, Geo APIs, tracemap, etc. |
| `NEXTTRACE_DATAPROVIDER` | unset | Override the default IP geolocation provider (for example `ipinfo`). |

#### Service / Web / Backend

| Variable | Default | Description |
| --- | --- | --- |
| `NEXTTRACE_HOSTPORT` | `api.nxtrace.org` | Override the backend host or `host:port` used by NextTrace API v3, tracemap, and FastIP flows. |
| `NEXTTRACE_TOKEN` | unset | Pre-supplied NextTrace API v3 bearer token; when present, token fetching via PoW is skipped. |
| `NEXTTRACE_API_V4_TOKEN` | unset | NextTrace API v4 HTTP GeoIP token. When unset, NextTrace also checks the temporary token files written by `nexttrace -x`; if neither exists, NextTrace API v3 WebSocket/PoW remains active. |
| `NEXTTRACE_POWPROVIDER` | `api.nxtrace.org` | Select the PoW provider for NextTrace API v3. The built-in non-default alias is `sakura`. |
| `NEXTTRACE_DEPLOY_ADDR` | unset | Default listen address for `--deploy` when `--listen` is not provided. |
| `NEXTTRACE_DEPLOY_TOKEN` | unset | Token for `--deploy` WebUI/API/WebSocket/MCP access. CLI `--deploy-token` takes precedence. |
| `NEXTTRACE_ALLOW_CROSS_ORIGIN` | `0` | Only for `--deploy`: allow cross-origin browser access to the Web UI / API. Disabled by default for safety. |

#### IP Database / Third-Party Providers

| Variable | Default | Description |
| --- | --- | --- |
| `NEXTTRACE_IPINFOLOCALPATH` | auto search | Full path to `ipinfoLocal.mmdb` for the `IPInfoLocal` provider. |
| `NEXTTRACE_CHUNZHENURL` | `http://127.0.0.1:2060` | Base URL of the Chunzhen lookup service. |
| `NEXTTRACE_IPINFO_TOKEN` | unset | Token for the `IPInfo` provider. |
| `NEXTTRACE_IPINSIGHT_TOKEN` | unset | Token for the `IPInsight` provider. |
| `NEXTTRACE_IPAPI_BASE` | provider built-in URL | Override the base URL used by compatible IP API clients in the current implementation (`IPInfo`, `IPInsight`, `ip-api.com`). |
| `IPDBONE_BASE_URL` | `https://api.ipdb.one` | Override the IPDB.One API base URL. |
| `IPDBONE_API_ID` | unset | IPDB.One API ID. |
| `IPDBONE_API_KEY` | unset | IPDB.One API key. |
| `GLOBALPING_TOKEN` | unset | Authentication token for Globalping; raises the anonymous hourly limit when provided. |

#### Standalone DNS Client (full flavor only)

These q-compatible variables apply only to the standalone DNS client mode.

| Variable | Default | Description |
| --- | --- | --- |
| `Q_DEFAULT_SERVER` | unset | Default DNS server when neither the command line nor `~/.qrc` selects one. |
| `NO_COLOR` | unset | Disable color in DNS client output when set to any non-empty value. |
| `SSLKEYLOGFILE` | unset | Write TLS session secrets to this file when `--tls-key-log-file` is not set. The file contains sensitive key material. |

#### Config Discovery

| Variable | Default | Description |
| --- | --- | --- |
| `XDG_CONFIG_HOME` | OS / shell default | If set, NextTrace also searches `$XDG_CONFIG_HOME/nexttrace` for `nt_config.yaml`. |

### For full usage list, please refer to the usage menu

The following is full-build help on macOS; Windows also provides `--init` and `--icmp-mode`.

```shell
usage: nexttrace [-h|--help] [-4|--ipv4] [-6|--ipv6] [-T|--tcp] [-U|--udp]
                 [--mtu] [-F|--fast-trace] [-p|--port <integer>] [-q|--queries
                 <integer>] [--max-attempts <integer>] [--parallel-requests
                 <integer>] [-m|--max-hops <integer>] [-d|--data-provider
                 (IP.SB|ip.sb|IPInfo|ipinfo|IPInsight|ipinsight|IPAPI.com|ip-api.com|IPInfoLocal|ipinfolocal|chunzhen|NextTrace-API|ipdb.one|disable-geoip|DN42|dn42)]
                 [--pow-provider (api.nxtrace.org|sakura)] [-n|--no-rdns]
                 [-a|--always-rdns] [-k|--traceroute] [-P|--route-path]
                 [-o|--output "<value>"] [-O|--output-default] [--table]
                 [-j|--json] [-c|--classic] [--dn42] [--raw] [-f|--first
                 <integer>] [-M|--map] [-e|--disable-mpls] [-V|--version]
                 [-x|--setup-api-v4-token] [-l|--dns] [--speed] [--nali]
                 [-s|--source "<value>"] [--fwmark "<value>"] [--source-port <integer>] [-D|--dev
                 "<value>"] [--listen "<value>"] [--deploy-token "<value>"]
                 [--mcp] [--deploy] [-z|--send-time <integer>] [-i|--ttl-time
                 <integer>] [--timeout <integer>] [--psize <integer>] [-Q|--tos
                 <integer>] [--dot-server
                 (dnssb|aliyun|dnspod|google|cloudflare)] [-g|--language
                 (en|cn)] [-C|--no-color] [--from "<value>"] [-t|--mtr]
                 [-r|--report] [-w|--wide] [--show-ips] [--mtr-columns <string>] [-y|--ipinfo <integer>]
                 [--mtr-record "<value>"] [--file "<value>"] [TARGET "<value>"]

                 An open source visual route tracking CLI tool

Arguments:

  -h  --help                         Print help information
  -4  --ipv4                         Use IPv4 only
  -6  --ipv6                         Use IPv6 only
  -T  --tcp                          Use TCP SYN for tracerouting (default
                                     dest-port is 80)
  -U  --udp                          Use UDP SYN for tracerouting (default
                                     dest-port is 33494)
      --mtu                          Run standalone UDP path-MTU discovery mode
                                     with streaming output and GeoIP/RDNS
  -F  --fast-trace                   One-Key Fast Trace to China ISPs
  -p  --port                         Set the destination port to use. With
                                     default of 80 for "tcp", 33494 for "udp"
  -q  --queries                      Traceroute: latency samples per hop
                                     (default 3). MTR: max probes per hop. 0 =
                                     unlimited in TUI/raw/JSON streams. JSON
                                     reports require a positive count. When
                                     omitted: 10 with --report/--wide
                                     (including --raw), otherwise unlimited
      --max-attempts                 Advanced: hard cap on probe packets per
                                     hop. Leave unset for auto sizing; raise on
                                     lossy links if --queries is not enough
      --parallel-requests            Advanced: total concurrent in-flight
                                     probes across TTLs. Use 1 on
                                     multipath/load-balanced paths; 6-18 is a
                                     good starting range on stable links.
                                     Default: 18
  -m  --max-hops                     Set the max number of hops (max TTL to be
                                     reached). Default: 30
  -d  --data-provider                Choose IP Geographic Data Provider
                                     [NextTrace-API, IP.SB, IPInfo, IPInsight,
                                     IP-API.com, IPInfoLocal, ipdb.one,
                                     chunzhen, disable-geoip, DN42]. Default:
                                     NextTrace-API
      --pow-provider                 Choose PoW Provider for NextTrace API v3
                                     [api.nxtrace.org, sakura] For China
                                     mainland users, please use sakura.
                                     Default: api.nxtrace.org
  -n  --no-rdns                      Do not resolve IP addresses to their
                                     domain names
  -a  --always-rdns                  Always resolve IP addresses to their
                                     domain names
  -k  --traceroute                   Use traditional traceroute mode; with
                                     --raw, preserve traditional raw output
  -P  --route-path                   Print traceroute hop path by ASN and
                                     location
  -o  --output                       Write realtime trace output and final stop
                                     reason to FILE
  -O  --output-default               Write realtime trace output and final stop
                                     reason to the default log file
                                     (/tmp/trace.log)
      --table                        Output trace results as a final summary
                                     table (traceroute report mode)
  -j  --json                         Output JSON; MTR streams NDJSON unless
                                     --report/--wide is selected
  -c  --classic                      Classic Output trace results like
                                     BestTrace
      --dn42                         DN42 Mode
      --raw                          Machine-friendly output; use --traceroute
                                     --raw to preserve traditional raw output.
                                     With MTR (--mtr/-r/-w), enables streaming
                                     raw event mode
  -f  --first                        Start from the first_ttl hop (instead of
                                     1). Default: 1
  -M  --map                          Disable Print Trace Map
  -e  --disable-mpls                 Disable MPLS
  -V  --version                      Print version info and exit
  -x  --setup-api-v4-token           Store a session-only NextTrace API v4
                                     token in a temporary file
  -l  --dns                          Run DNS client mode. See `nexttrace --dns
                                     --help` for details
      --speed                        Run CDN speed test mode. See `nexttrace
                                     --speed --help` for details
      --nali                         Annotate IP literals in text using
                                     NextTrace GeoIP data
  -s  --source                       Use source address src_addr for outgoing
                                     packets
      --fwmark                       Linux probe socket mark (decimal or 0x
                                     hex); traceroute/MTR only
      --source-port                  Use source port src_port for outgoing
                                     packets
  -D  --dev                          Use the specified network device for
                                     explicit source selection. On Windows,
                                     this selects the device source address;
                                     routing may still choose the egress
                                     interface
      --listen                       Set listen address for web console (e.g.
                                     127.0.0.1:30080)
      --deploy-token                 Set bearer token for --deploy
                                     WebUI/API/WebSocket/MCP access
      --mcp                          Enable MCP endpoint under --deploy at /mcp
      --deploy                       Start the Gin powered web console
  -z  --send-time                    Advanced: per-packet gap [ms] inside the
                                     same TTL group. Lower is faster; raise to
                                     100-200ms on rate-limited links. Ignored
                                     in MTR mode. Default: 50
  -i  --ttl-time                     Advanced: TTL-group interval [ms] in
                                     normal traceroute. In MTR mode
                                     (--mtr/-r/-w, including --raw), this
                                     becomes per-hop probe interval. 500-1000ms
                                     is a good MTR starting range
      --timeout                      Per-probe timeout [ms]. Raise to 2000-3000
                                     on slow intercontinental or high-loss
                                     paths. Default: 1000
      --psize                        Probe packet size in bytes, inclusive IP
                                     and active probe headers. Default is the
                                     minimum legal size for the chosen protocol
                                     and IP family; raise for MTU or
                                     large-packet testing. Negative values
                                     randomize each probe up to abs(value)
  -Q  --tos                          Set the full 8-bit IP type-of-service /
                                     traffic class [0-255]: DSCP*4+ECN (DSCP
                                     46, ECN 0 = 184). Default: 0
      --dot-server                   Use DoT Server for DNS Parse [dnssb,
                                     aliyun, dnspod, google, cloudflare]
  -g  --language                     Choose the language for displaying [en,
                                     cn]. Default: cn
  -C  --no-color                     Disable Colorful Output
      --from                         Run traceroute via Globalping
                                     (https://globalping.io/network) from a
                                     specified location. The location field
                                     accepts continents, countries, regions,
                                     cities, ASNs, ISPs, or cloud regions.
  -t  --mtr                          Enable MTR (My Traceroute) continuous
                                     probing mode
  -r  --report                       MTR report mode (non-interactive, implies
                                     --mtr); can trigger MTR without --mtr
  -w  --wide                         MTR wide report mode (implies --mtr
                                     --report); alone equals --mtr --report
                                     --wide
      --mtr-columns                  MTR text columns in order: loss, snt,
                                     received, last, avg, best, wrst, stdev,
                                     dropped, gmean, jitter, javg, jmax, jint,
                                     space (does not enable MTR)
      --show-ips                     MTR only: display both PTR hostnames and
                                     numeric IPs (PTR first, IP in parentheses)
  -y  --ipinfo                       Set initial MTR TUI host info mode (0-4).
                                     TUI only; ignored in --report/--raw/--json.
                                     0:IP/PTR 1:ASN 2:City 3:Owner 4:Full.
                                     Default: 0
      --mtr-record                   Save this MTR session to a new file for
                                     offline replay
      --file                         Read IP Address or domain name from file
      TARGET                         Trace target: IPv4 address (e.g. 8.8.8.8),
                                     IPv6 address (e.g. 2001:db8::1), domain
                                     name (e.g. example.com), or URL (e.g.
                                     https://example.com)

  --doctor  Check local probe prerequisites without sending probes; see --doctor --help
  --mtr-replay FILE  Open a saved MTR session offline; see --mtr-replay --help
```

## Project screenshot

![image](https://user-images.githubusercontent.com/13616352/216064486-5e0a4ad5-01d6-4b3c-85e9-2e6d2519dc5d.png)

![image](https://user-images.githubusercontent.com/59512455/218501311-1ceb9b79-79e6-4eb6-988a-9d38f626cdb8.png)

## OpenTrace

`OpenTrace` is the cross-platform `GUI` version of `NextTrace` developed by @Archeb, bringing a familiar but more powerful user experience.

This software is still in the early stages of development and may have many flaws and errors. We value your feedback.

[https://github.com/Archeb/opentrace](https://github.com/Archeb/opentrace)

## GlobalTrace

`GlobalTrace` is an open-source `Globalping x NextTrace` web traceroute project. It uses Globalping's worldwide probe network to run `MTR` measurements from multiple regions, then enriches hop IPs with the NextTrace / NTrace backbone IP database for GeoIP, ASN, and network ownership details.

Website: [https://lg.nxtrace.org](https://lg.nxtrace.org)

Project: [nxtrace/GlobalTrace](https://github.com/nxtrace/GlobalTrace)

## NextTrace Web

`NextTrace Web` is a web-based server implementation of `NextTrace` in the `MTR` style, offering various deployment options including `Docker`.

[https://github.com/nxtrace/nexttraceweb](https://github.com/nxtrace/nexttraceweb)

## Deploy WebUI and MCP

The full `nexttrace` binary can expose the local WebUI/API/WebSocket server:

```bash
nexttrace --deploy
```

MCP is a deploy submode and is exposed over the same network stack at `/mcp`:

```bash
nexttrace --deploy --mcp
nexttrace --deploy --mcp --listen 0.0.0.0:1080 --deploy-token "$TOKEN"
```

Loopback listen addresses (`127.0.0.1`, `::1`, `localhost`) are tokenless by default. External listen addresses require a token; if none is set with `--deploy-token` or `NEXTTRACE_DEPLOY_TOKEN`, NextTrace generates one and prints it to stdout. API, WebSocket, and MCP clients may use `Authorization: Bearer <token>` or `X-NextTrace-Token`; browser WebUI users can sign in at `/auth/login`.

### Register MCP in Agent clients

Start NextTrace first. The MCP endpoint is Streamable HTTP, not stdio:

```text
http://127.0.0.1:1080/mcp
```

For external listeners or manually configured tokens, pass the token in an HTTP header. Do not put deploy tokens in URL query strings.

Generic MCP client config:

```json
{
  "mcp": {
    "servers": {
      "nexttrace": {
        "url": "http://127.0.0.1:1080/mcp",
        "transport": "streamable-http",
        "headers": {
          "Authorization": "Bearer <token>"
        }
      }
    }
  }
}
```

OpenClaw can save the same server definition with [`openclaw mcp set`](https://docs.openclaw.ai/cli/mcp):

```bash
openclaw mcp set nexttrace '{
  "url": "http://127.0.0.1:1080/mcp",
  "transport": "streamable-http",
  "headers": {
    "Authorization": "Bearer <token>"
  }
}'
```

`openclaw mcp set` only saves the MCP server definition. It does not start NextTrace or verify that the endpoint is reachable, so run `nexttrace --deploy --mcp` first.

Useful first tool calls for Agents:

- `nexttrace_capabilities`
- `nexttrace_traceroute`
- `nexttrace_globalping_trace`
- `nexttrace_globalping_limits`

## NextTraceroute

`NextTraceroute` is a root-free Android route tracing application that defaults to using the `NextTrace API`, developed by @surfaceocean.  
Thank you to all the test users for your enthusiastic support. This app has successfully passed the closed testing phase and is now officially available on the Google Play Store.

[https://github.com/nxtrace/NextTraceroute](https://github.com/nxtrace/NextTraceroute)  
<a href='https://play.google.com/store/apps/details?id=com.surfaceocean.nexttraceroute&pcampaignid=pcampaignidMKT-Other-global-all-co-prtnr-py-PartBadge-Mar2515-1'><img alt='Get it on Google Play' width="128" height="48" src='https://play.google.com/intl/en_us/badges/static/images/badges/en_badge_web_generic.png'/></a>

## NextTrace API Credits

NextTrace focuses on Golang traceroute implementations, and its NextTrace API geolocation information is not supported by raw data, so a commercial version is not possible.

The NextTrace API data is subject to copyright restrictions from multiple data sources and is only used to display traceroute geolocation.

1. We would like to credit samleong123 for providing nodes in Malaysia, TOHUNET Looking Glass for global nodes, and Ping.sx from Misaka, where more than 80% of reliable calibration data comes from ping/mtr reports.

2. At the same time, we would like to credit isyekong for their contribution to rDNS-based calibration ideas and data. NextTrace API is accelerating the development of rDNS resolution and has already achieved automated geolocation resolution for some backbone networks, though some results remain inaccurate. We hope that NextTrace will become a One-Man ISP-friendly traceroute tool in the future, and we are working on improving the calibration of these ASN micro-backbones as much as possible.

3. In terms of development, I would like to credit missuo and zhshch for their help with Go cross-compilation, design concepts and TCP/UDP Traceroute refactoring, and tsosunchia for their support on TraceMap.

4. I would also like to credit FFEE_CO, TheresaQWQ, stydxm and others for their help. NextTrace API has received a lot of support since its first release, so I would like to credit them all!

We hope you can give us as much feedback as possible on IP geolocation errors (see issue) so that it can be calibrated in the first place and others can benefit from it.

## Cloudflare Support

This project is sponsored by [Project Alexandria](http://www.cloudflare.com/oss-credits).

<img src="https://cf-assets.www.cloudflare.com/slt3lc6tev37/2I3y49Uz9Y61lBS0kIPZu6/db6df1e6f99a8659267c442b75a0dff9/image.png" alt="Cloudflare Logo" width="331">

## AIWEN TECH Support

This project is sponsored by [AIWEN TECH](https://www.ipplus360.com). We’re pleased to enhance the accuracy and completeness of this project’s GEOIP lookups using `AIWEN TECH City-Level IP Database`, and to make it freely available to the public.

<img src="https://www.ipplus360.com/img/LOGO.c86cd0e1.svg" title="" alt="AIWEN TECH IP Geolocation Data" width="331">

## JetBrain Support

This Project uses [JetBrain Open-Source Project License](https://jb.gg/OpenSourceSupport). We Proudly Develop By `Goland`.

<img src="https://resources.jetbrains.com/storage/products/company/brand/logos/GoLand.png" title="" alt="GoLand logo" width="331">

## Credits

[Gubo](https://www.gubo.org) Reliable Host Recommendation Website

[IPInfo](https://ipinfo.io) Provided most of the data support for this project free of charge

[BGP.TOOLS](https://bgp.tools) Provided some data support for this project free of charge

[PeeringDB](https://www.peeringdb.com) Provided some data support for this project free of charge

[Globalping](https://globalping.io) An open-source and free project that provides global access to run network tests like traceroute

[sjlleo](https://github.com/sjlleo) The perpetual leader, founder, and core contributors

[tsosunchia](https://github.com/tsosunchia) The project chair, infra maintainer, and core contributors

[Yunlq](https://github.com/Yunlq) An active community contributor

[Vincent Young](https://github.com/missuo)

[zhshch2002](https://github.com/zhshch2002)

[Sam Sam](https://github.com/samleong123)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p)

[bobo liu](https://github.com/fakeboboliu)

[YekongTAT](https://github.com/isyekong)

### Others

- Although other third-party APIs are integrated in this project, please refer to the official website of the third-party APIs for specific TOS and AUP. If you encounter IP data errors, please contact them directly to correct them.

- For feedback related to corrections about IP information, we currently have two channels available:

  > - [IP 错误报告汇总帖](https://github.com/orgs/nxtrace/discussions/222) in the GITHUB ISSUES section of this project (Recommended)
  > - This project's dedicated correction email: `correct#nxtrace.org` (Please note that this email is only for correcting IP-related information. For other feedback, please submit an ISSUE)

- How to obtain the freshly baked binary executable of the latest commit?

  > Please go to the most recent [Build & Release](https://github.com/nxtrace/NTrace-dev/actions/workflows/build.yml) workflow in GitHub Actions.

- Common questions
  - On Windows, ICMP mode requires manual firewall allowance for ICMP/ICMPv6
  - On macOS, only ICMP mode does not require elevated privileges
  - In some cases, running multiple instances of NextTrace simultaneously may interfere with each other’s results (observed so far only in TCP mode)

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=nxtrace/NTrace-core&type=Date)](https://star-history.dera.page/#nxtrace/NTrace-core&type=Date)
