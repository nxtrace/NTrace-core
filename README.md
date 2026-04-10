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
  <a href="https://goreportcard.com/report/github.com/nxtrace/NTrace-dev">
    <img src="https://goreportcard.com/badge/github.com/nxtrace/NTrace-dev?style=flat-square">
  </a>
  <a href="https://github.com/nxtrace/NTrace-dev/releases">
    <img src="https://img.shields.io/github/release/nxtrace/NTrace-dev/all.svg?style=flat-square">
  </a>
</p>

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
    - Add the repository and install:
      ```shell
      sudo install -d -m 0755 /etc/apt/keyrings
      curl -fsSL -o /tmp/nexttrace-archive-keyring.gpg https://github.com/nxtrace/nexttrace-debs/releases/latest/download/nexttrace-archive-keyring.gpg
      sudo install -m 0644 /tmp/nexttrace-archive-keyring.gpg /etc/apt/keyrings/nexttrace.gpg
      rm -f /tmp/nexttrace-archive-keyring.gpg
      sudo tee /etc/apt/sources.list.d/nexttrace.sources >/dev/null <<'EOF'
Types: deb
URIs: https://github.com/nxtrace/nexttrace-debs/releases/latest/download/
Suites: ./
Signed-By: /etc/apt/keyrings/nexttrace.gpg
EOF
      sudo apt update
      sudo apt install nexttrace
      ```
    - Install a specific flavor:
      ```shell
      sudo apt install nexttrace
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

    Same as the macOS Homebrew's installation method (homebrew-core version only supports amd64)

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
    - This repository's ACTIONS automatically built version (updates faster)
      ```shell
      brew tap nxtrace/nexttrace && brew install nxtrace/nexttrace/nexttrace
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
- Other package sources above are maintained by open source enthusiasts. Availability and timely updates are not guaranteed. If you encounter problems, please contact the repository maintainer to solve them, or use the binary packages provided by the official build of this project.

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
| Standalone MTU (`--mtu`) |      ✅         |        ✅        |      —       |
| MTR TUI               |         ✅         |        —         | ✅ (default) |
| MTR report (`-r`)     |         ✅         |        —         |      ✅      |
| MTR wide (`-w`)       |         ✅         |        —         |      ✅      |
| MTR raw (`--raw`)     |         ✅         |        —         |      ✅      |
| Globalping (`--from`) |         ✅         |        —         |      —       |
| WebUI (`--deploy`)    |         ✅         |        —         |      —       |
| Fast Trace (`-F`)     |         ✅         |        ✅        |      —       |
| Default mode          |     traceroute     |    traceroute    |   MTR TUI    |
| Binary name           |    `nexttrace`     | `nexttrace-tiny` |    `ntr`     |

> **Note:** `APT (nexttrace-debs)` provides all three flavors: **Full** (`nexttrace`), **Tiny** (`nexttrace-tiny`), and **NTR** (`ntr`). Other package managers (Homebrew, AUR, Scoop, etc.) currently install the **Full** (`nexttrace`) version only.

### Feature Matrix

- **`nexttrace`** — Full-featured build. Includes everything: traceroute, MTR, Globalping, and WebUI.
- **`nexttrace-tiny`** — Lightweight build. Normal traceroute only, no MTR / Globalping / WebUI. Suitable for embedded or minimal environments.
- **`ntr`** — MTR-focused build. Runs MTR TUI by default. No Globalping / WebUI; no normal traceroute mode and no standalone `--mtu` mode.

### Manual Build

Build from source with Go 1.22+ installed:

```bash
# Full (all features)
go build -trimpath -o dist/nexttrace -ldflags "-w -s" .

# Tiny (no MTR, no Globalping, no WebUI)
go build -tags flavor_tiny -trimpath -o dist/nexttrace-tiny -ldflags "-w -s" .

# NTR (MTR-only)
go build -tags flavor_ntr -trimpath -o dist/ntr -ldflags "-w -s" .
```

Cross-compile example:

```bash
# Linux arm64, Tiny flavor
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags flavor_tiny -trimpath -o dist/nexttrace-tiny_linux_arm64 -ldflags "-w -s" .
```

The `tiny` and `ntr` flavors use **compile-time build tags** to exclude modules — this is not a runtime switch. You can verify with `go version -m <binary>` that `gin` and `globalping-cli` are absent from `nexttrace-tiny` and `ntr`.

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

# Machine-readable output: stdout is a single JSON document
nexttrace --raw 1.0.0.1
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

PS: The route visualization module is an independent component, You can find its source code at [nxtrace/traceMap](https://github.com/nxtrace/traceMap).  
The routing visualization function requires the geographical coordinates of each Hop, but third-party APIs generally do not provide this information, so this function is currently only supported when used with LeoMoeAPI.

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
  On `Windows`, `ICMPv6` without `--tos` (or with `--tos 0`) keeps using the native Socket send path. A non-zero `ICMPv6 --tos` requires `WinDivert` send support in addition to administrator privilege.  
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

On macOS and Linux, `--dev` binds the requested source interface.
On Windows, `--dev` only selects the source address and does not guarantee the actual egress interface.
`TCP + --dev` remains explicitly unsupported on Windows and returns an error.

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
# (If you need to use a different random source port for each packet, please set the ENV variable NEXTTRACE_RANDOMPORT, or set the source port to -1)
nexttrace --tcp --source-port 14514 www.bing.com
```

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

# Set the TOS / traffic class field
nexttrace -Q 46 example.com

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
| `--queries` | Samples per hop in normal traceroute; explicit probe count per hop in MTR | traceroute: `3`; MTR report: `10` when omitted; MTR TUI/raw: unlimited when omitted | Raise to `5-10` on unstable paths |
| `--max-attempts` | Hard cap on probe packets per hop | auto-sized from `--queries` | Raise on lossy links when replies arrive slowly |
| `--parallel-requests` | Total in-flight probes across TTLs | `18` | Use `1` on multipath/load-balanced paths; keep `6-18` on stable links |
| `--send-time` | Gap between packets inside one TTL group | `50ms` | Raise to `100-200ms` on rate-limited devices; ignored in MTR |
| `--ttl-time` | Gap between TTL groups in traceroute; per-hop interval in MTR | traceroute: `300ms`; MTR: `1000ms` when omitted | Lower to speed up; raise on remote/rate-limited paths |
| `--timeout` | Per-probe timeout | `1000ms` | Raise to `2000-3000ms` for intercontinental or high-loss paths |
| `--psize` | Probe packet size | Protocol/IP-family minimum | Inclusive IP + probe headers; negative values randomize each probe up to `abs(value)`; sizes above the egress/path MTU may fragment on wire |
| `-Q`, `--tos` | IP TOS / traffic class | `0` | Set DSCP/TOS style marking in the IP header; on Windows only `ICMPv6` with a non-zero value requires `WinDivert` |

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

When running in a terminal (TTY), MTR mode uses an **interactive full-screen TUI**:

- **`q` / `Q`** — quit (restores terminal, no output left behind)
- **`p`** — pause probing
- **`SPACE`** — resume probing
- **`r`** — reset statistics (counters are cleared, display mode is preserved)
- **`y`** — cycle host display mode: ASN → City → Owner → Full
- **`n`** — toggle host name display:
  - default: PTR (or IP fallback) ↔ IP only
  - with `--show-ips`: PTR (IP) ↔ IP only
- **`e`** — toggle MPLS label display on/off
- The TUI header displays **source → destination**, with `--source`/`--dev` information when specified.
- When using LeoMoeAPI, the preferred API IP address is shown in the header.
- Uses the **alternate screen buffer**, so your previous terminal history is preserved on exit.
- When stdin is not a TTY (e.g. piped), it falls back to a simple table refresh.

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

If the active data provider is `LeoMoeAPI`, NextTrace first prints one uncolored API info preamble line:

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

In MTR mode (`--mtr`, `-r`, `-w`, including `--raw`), `-i/--ttl-time` sets the **per-hop probe interval**: how long to wait between successive probes to the same hop (default: 1000ms when omitted). `-z/--send-time` is ignored in MTR mode.

> Note: `--show-ips` only takes effect in MTR mode (`--mtr`, `-r`, `-w`); otherwise it is ignored.
>
> Note: `--mtr` cannot be used together with `--table`, `--classic`, `--json`, `--output`, `--output-default`, `--route-path`, `--from`, `--fast-trace`, `--file`, or `--deploy`.

#### `NextTrace` supports users to select their own IP API (currently supports: `LeoMoeAPI`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`, `IPInfoLocal`, `CHUNZHEN`)

```bash
# You can specify the IP database by yourself [IP-API.com here], if not specified, LeoMoeAPI will be used
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

NextTrace `LeoMoeAPI` now utilizes the Proof of Work (POW) mechanism to prevent abuse, where NextTrace introduces the powclient library as a client-side component. Both the POW CLIENT and SERVER are open source, and everyone is welcome to use them. (Please direct any POW module-related questions to the corresponding repositories)

- [GitHub - tsosunchia/powclient: Proof of Work CLIENT for NextTrace](https://github.com/tsosunchia/powclient)
- [GitHub - tsosunchia/powserver: Proof of Work SERVER for NextTrace](https://github.com/tsosunchia/powserver)

All NextTrace IP geolocation `API DEMO` can refer to [here](https://github.com/nxtrace/NTrace-core/blob/main/ipgeo/)

### Environment Variables

NextTrace currently reads the following environment variables. For boolean switches, only `1` and `0` are recognized; other values fall back to the built-in default. For consistency, restart NextTrace after changing them.

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
| `NEXTTRACE_HOSTPORT` | `api.nxtrace.org` | Override the backend host or `host:port` used by LeoMoeAPI, tracemap, and FastIP flows. |
| `NEXTTRACE_TOKEN` | unset | Pre-supplied LeoMoeAPI bearer token; when present, token fetching via PoW is skipped. |
| `NEXTTRACE_POWPROVIDER` | `api.nxtrace.org` | Select the PoW provider. The built-in non-default alias is `sakura`. |
| `NEXTTRACE_DEPLOY_ADDR` | unset | Default listen address for `--deploy` when `--listen` is not provided. |
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

#### Config Discovery

| Variable | Default | Description |
| --- | --- | --- |
| `XDG_CONFIG_HOME` | OS / shell default | If set, NextTrace also searches `$XDG_CONFIG_HOME/nexttrace` for `nt_config.yaml`. |

### For full usage list, please refer to the usage menu

```shell
Usage: nexttrace [-h|--help] [--init] [-4|--ipv4] [-6|--ipv6] [-T|--tcp]
                 [-U|--udp] [-F|--fast-trace] [-p|--port <integer>]
                 [--icmp-mode <integer>] [-q|--queries <integer>]
                 [--max-attempts <integer>] [--parallel-requests <integer>]
                 [-m|--max-hops <integer>] [-d|--data-provider
                 (IP.SB|ip.sb|IPInfo|ipinfo|IPInsight|ipinsight|IPAPI.com|ip-api.com|IPInfoLocal|ipinfolocal|chunzhen|LeoMoeAPI|leomoeapi|ipdb.one|disable-geoip)]
                 [--pow-provider (api.nxtrace.org|sakura)] [-n|--no-rdns]
                 [-a|--always-rdns] [-P|--route-path] [--dn42] [-o|--output
                 "<value>"] [-O|--output-default] [--table] [--raw]
                 [-j|--json] [-c|--classic] [-f|--first <integer>] [-M|--map]
                 [-e|--disable-mpls] [-V|--version]
                 [-s|--source "<value>"] [--source-port <integer>] [-D|--dev
                 "<value>"] [--listen "<value>"] [--deploy] [-z|--send-time
                 <integer>] [-i|--ttl-time <integer>] [--timeout <integer>]
                 [--psize <integer>] [--dot-server
                 (dnssb|aliyun|dnspod|google|cloudflare)] [-g|--language
                 (en|cn)] [-C|--no-color] [--from "<value>"] [-t|--mtr]
                 [-r|--report] [-w|--wide] [--show-ips] [-y|--ipinfo <integer>]
                 [--file "<value>"] [TARGET "<value>"]

Arguments:

  -h  --help                         Print help information
      --init                         Windows ONLY: Extract WinDivert runtime to
                                     executable directory
  -4  --ipv4                         Use IPv4 only
  -6  --ipv6                         Use IPv6 only
  -T  --tcp                          Use TCP SYN for tracerouting (default
                                     dest-port is 80)
  -U  --udp                          Use UDP SYN for tracerouting (default
                                     dest-port is 33494)
  -F  --fast-trace                   One-Key Fast Trace to China ISPs
  -p  --port                         Set the destination port to use. With
                                     default of 80 for "tcp", 33494 for "udp"
      --icmp-mode                    Windows ONLY: Choose the method to listen
                                     for ICMP packets (1=Socket, 2=WinDivert;
                                     0=Auto)
  -q  --queries                      Latency samples per hop. Increase to 5-10
                                     on unstable paths for a steadier view.
                                     Default: 3
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
  -d  --data-provider                Choose IP Geograph Data Provider [IP.SB,
                                     IPInfo, IPInsight, IP-API.com,
                                     IPInfoLocal, CHUNZHEN, disable-geoip].
                                     Default: LeoMoeAPI
      --pow-provider                 Choose PoW Provider [api.nxtrace.org,
                                     sakura] For China mainland users, please
                                     use sakura. Default: api.nxtrace.org
  -n  --no-rdns                      Do not resolve IP addresses to their
                                     domain names
  -a  --always-rdns                  Always resolve IP addresses to their
                                     domain names
  -P  --route-path                   Print traceroute hop path by ASN and
                                     location
      --dn42                         DN42 Mode
  -o  --output                       Write trace result to FILE
                                     (RealtimePrinter only)
  -O  --output-default               Write trace result to the default log file
                                     (/tmp/trace.log)
      --table                        Output trace results as a final summary
                                     table (traceroute report mode)
      --raw                          Machine-friendly output. With MTR
                                     (--mtr/-r/-w), enables streaming raw event
                                     mode
  -j  --json                         Output trace results as JSON
  -c  --classic                      Classic Output trace results like
                                     BestTrace
  -f  --first                        Start from the first_ttl hop (instead of
                                     1). Default: 1
  -M  --map                          Disable Print Trace Map
  -e  --disable-mpls                 Disable MPLS
  -V  --version                      Print version info and exit
  -s  --source                       Use source address src_addr for outgoing
                                     packets
      --source-port                  Use source port src_port for outgoing
                                     packets
  -D  --dev                          Use the specified network device for
                                     explicit source selection. On Windows,
                                     this only chooses the source address and
                                     does not guarantee the egress interface
      --listen                       Set listen address for web console (e.g.
                                     127.0.0.1:30080)
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
                                     minimum legal size for the chosen
                                     protocol and IP family; raise for MTU or
                                     large-packet testing. Negative values
                                     randomize each probe up to abs(value).
  -Q  --tos                          Set the IP type-of-service / traffic class
                                     value [0-255]. Default: 0
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
      --show-ips                     MTR only: display both PTR hostnames and
                                     numeric IPs (PTR first, IP in parentheses)
  -y  --ipinfo                       Set initial MTR TUI host info mode (0-4).
                                     TUI only; ignored in --report/--raw.
                                     0:IP/PTR 1:ASN 2:City 3:Owner 4:Full.
                                     Default: 0
      --file                         Read IP Address or domain name from file
      TARGET                         Trace target: IPv4 address (e.g. 8.8.8.8),
                                     IPv6 address (e.g. 2001:db8::1), domain
                                     name (e.g. example.com), or URL (e.g.
                                     https://example.com)
```

## Project screenshot

![image](https://user-images.githubusercontent.com/13616352/216064486-5e0a4ad5-01d6-4b3c-85e9-2e6d2519dc5d.png)

![image](https://user-images.githubusercontent.com/59512455/218501311-1ceb9b79-79e6-4eb6-988a-9d38f626cdb8.png)

## OpenTrace

`OpenTrace` is the cross-platform `GUI` version of `NextTrace` developed by @Archeb, bringing a familiar but more powerful user experience.

This software is still in the early stages of development and may have many flaws and errors. We value your feedback.

[https://github.com/Archeb/opentrace](https://github.com/Archeb/opentrace)

## NEXTTRACE WEB API

`NextTraceWebApi` is a web-based server implementation of `NextTrace` in the `MTR` style, offering various deployment options including `Docker`.

For WebSocket continuous tracing, MTR now streams per-event payloads with `type: "mtr_raw"` (instead of periodic `mtr` snapshots).

[https://github.com/nxtrace/nexttracewebapi](https://github.com/nxtrace/nexttracewebapi)

## NextTraceroute

`NextTraceroute` is a root-free Android route tracing application that defaults to using the `NextTrace API`, developed by @surfaceocean.  
Thank you to all the test users for your enthusiastic support. This app has successfully passed the closed testing phase and is now officially available on the Google Play Store.

[https://github.com/nxtrace/NextTraceroute](https://github.com/nxtrace/NextTraceroute)  
<a href='https://play.google.com/store/apps/details?id=com.surfaceocean.nexttraceroute&pcampaignid=pcampaignidMKT-Other-global-all-co-prtnr-py-PartBadge-Mar2515-1'><img alt='Get it on Google Play' width="128" height="48" src='https://play.google.com/intl/en_us/badges/static/images/badges/en_badge_web_generic.png'/></a>

## LeoMoeAPI Credits

NextTrace focuses on Golang Traceroute implementations, and its LeoMoeAPI geolocation information is not supported by raw data, so a commercial version is not possible.

The LeoMoeAPI data is subject to copyright restrictions from multiple data sources, and is only used for the purpose of displaying the geolocation of route tracing.

1. We would like to credit samleong123 for providing nodes in Malaysia, TOHUNET Looking Glass for global nodes, and Ping.sx from Misaka, where more than 80% of reliable calibration data comes from ping/mtr reports.

2. At the same time, we would like to credit isyekong for their contribution to rDNS-based calibration ideas and data. LeoMoeAPI is accelerating the development of rDNS resolution function, and has already achieved automated geolocation resolution for some backbone networks, but there are some misjudgments. We hope that NextTrace will become a One-Man ISP-friendly traceroute tool in the future, and we are working on improving the calibration of these ASN micro-backbones as much as possible.

3. In terms of development, I would like to credit missuo and zhshch for their help with Go cross-compilation, design concepts and TCP/UDP Traceroute refactoring, and tsosunchia for their support on TraceMap.

4. I would also like to credit FFEE_CO, TheresaQWQ, stydxm and others for their help. LeoMoeAPI has received a lot of support since its first release, so I would like to credit them all!

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

[![Star History Chart](https://api.star-history.com/svg?repos=nxtrace/NTrace-core&type=Date)](https://star-history.com/#nxtrace/NTrace-core&Date)
