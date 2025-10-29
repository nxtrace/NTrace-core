<div align="center">

<img src="assets/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

<h1 align="center">
  <br>NextTrace<br>
</h1>

<h4 align="center">An open source visual routing tool that pursues light weight, developed using Golang.</h4>

---------------------------------------

<h6 align="center">HomePage: www.nxtrace.org</h6>

<p align="center">
  <a href="https://github.com/nxtrace/NTrace-V1/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/nxtrace/NTrace-V1/build.yml?branch=main&style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/nxtrace/NTrace-V1">
    <img src="https://goreportcard.com/badge/github.com/nxtrace/NTrace-V1?style=flat-square">
  </a>
  <a href="https://github.com/nxtrace/NTrace-V1/releases">
    <img src="https://img.shields.io/github/release/nxtrace/NTrace-V1/all.svg?style=flat-square">
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

⚠️ Please note: We welcome PR submissions from the community, but please submit your PRs to the [NTrace-V1](https://github.com/nxtrace/NTrace-V1) repository instead of [NTrace-core](https://github.com/nxtrace/NTrace-core) repository.<br>
Regarding the NTrace-V1 and NTrace-core repositories:<br>
Both will largely remain consistent with each other. All development work is done within the NTrace-V1 repository. The NTrace-V1 repository releases new versions first. After running stably for an undetermined period, we will synchronize that version to NTrace-core. This means that the NTrace-V1 repository serves as a "beta" or "testing" version.<br>
Please note, there are exceptions to this synchronization. If a version of NTrace-V1 encounters a serious bug, NTrace-core will skip that flawed version and synchronize directly to the next version that resolves the issue.

### Automated Install

* Linux
    * One-click installation script
      ```shell
      curl -sL nxtrace.org/nt |bash
      ```

    * Install nxtrace from the APT repository
        * Supports AMD64/ARM64 architectures
          ```shell
          curl -fsSL https://github.com/nxtrace/nexttrace-debs/releases/latest/download/nexttrace-archive-keyring.gpg | sudo tee /etc/apt/keyrings/nexttrace.gpg >/dev/null
          echo "Types: deb
          URIs: https://github.com/nxtrace/nexttrace-debs/releases/latest/download/
          Suites: ./
          Signed-By: /etc/apt/keyrings/nexttrace.gpg" | sudo tee /etc/apt/sources.list.d/nexttrace.sources >/dev/null
          sudo apt update
          sudo apt install nexttrace
          ```
        * APT repository maintained by wcbing and nxtrace

    * Arch Linux AUR installation command
        * Directly download bin package (only supports amd64)
          ```shell
          yay -S nexttrace-bin
          ```
        * Build from source (only supports amd64)
          ```shell
          yay -S nexttrace
          ```
        * The AUR builds are maintained by ouuan, huyz

    * Linuxbrew's installation command  

      Same as the macOS Homebrew's installation method (homebrew-core version only supports amd64)

    * deepin installation command
      ```shell
      apt install nexttrace
      ```
    
    * [x-cmd](https://www.x-cmd.com/pkg/nexttrace) installation command
      ```shell
      x env use nexttrace
      ```

    * Termux installation command
      ```shell
      pkg install root-repo
      pkg install nexttrace
      ```
    
    * ImmortalWrt installation command
      ```shell
      opkg install nexttrace
      ```

* macOS
    * macOS Homebrew's installation command
        * Homebrew-core version
          ```shell
          brew install nexttrace
          ```
        * This repository's ACTIONS automatically built version (updates faster)
          ```shell
          brew tap nxtrace/nexttrace && brew install nxtrace/nexttrace/nexttrace
          ```
        * The homebrew-core build is maintained by chenrui333, please note that this version's updates may lag behind the repository Action automatically version

* Windows
    * Windows WinGet installation command
        * WinGet version
          ```powershell
          winget install nexttrace
          ```
        * WinGet build maintained by Dragon1573

    * Windows Scoop installation command
        * Scoop-extras version
          ```powershell
          scoop bucket add extras && scoop install extras/nexttrace
          ```
        * Scoop-extra is maintained by soenggam

Please note, the repositories for all of the above installation methods are maintained by open source enthusiasts. Availability and timely updates are not guaranteed. If you encounter problems, please contact the repository maintainer to solve them, or use the binary packages provided by the official build of this project.

### Manual Install
* Download the precompiled executable

    For users not covered by the above methods, please go directly to [Release](https://www.nxtrace.org/downloads) to download the compiled binary executable.

    * `Release` provides compiled binary executables for many systems and different architectures. If none are available, you can compile it yourself.
    * Some essential dependencies of this project are not fully implemented on `Windows` by `Golang`, so currently, `NextTrace` is in an experimental support phase on the `Windows` platform.

### Get Started

`NextTrace` uses the `ICMP` protocol to perform TraceRoute requests by default, which supports both `IPv4` and `IPv6`

```bash
# IPv4 ICMP Trace
nexttrace 1.0.0.1
# URL
nexttrace http://example.com:8080/index.html?q=1

# Form printing
nexttrace --table 1.0.0.1

# An Output Easy to Parse
nexttrace --raw 1.0.0.1
nexttrace --json 1.0.0.1

# IPv4/IPv6 Resolve Only, and automatically select the first IP when there are multiple IPs
nexttrace --ipv4 g.co
nexttrace --ipv6 g.co

# IPv6 ICMP Trace
nexttrace 2606:4700:4700::1111

# Developer mode: set the ENV variable NEXTTRACE_DEVMODE=1 to make fatal errors panic with a stack trace.
export NEXTTRACE_DEVMODE=1

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
  **TCP/UDP mode** requires additional installation of `npcap` and `WinDivert`.  
  **ICMP mode** requires allowing `ICMP/ICMPv6` in the firewall if `npcap` and `WinDivert` are not installed.  
  You can download and install `npcap` from the official website ([https://npcap.com/#download](https://npcap.com/#download)), and `WinDivert` can be automatically configured using the `--init` parameter.

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

#### `NextTrace` also supports some advanced functions, such as ttl control, concurrent probe packet count control, mode switching, etc.

```bash
# Send 2 probe packets per hop
nexttrace --queries 2 www.hkix.net

# Set the maximum attempts per TTL
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

# Set the payload size to 1024 bytes
nexttrace --psize 1024 example.com

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
nexttrace -T -q 2 --parallel-requests 1 -t -P 2001:4860:4860::8888
```

### Globalping

[Globalping](https://globalping.io/) provides access to thousands of community hosted probes to run network tests and measurements.

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

### For full usage list, please refer to the usage menu

```shell
Usage: nexttrace [-h|--help] [--init] [-4|--ipv4] [-6|--ipv6] [-T|--tcp]
                 [-U|--udp] [-F|--fast-trace] [-p|--port <integer>]
                 [--icmp-mode <integer>] [-q|--queries <integer>]
                 [--parallel-requests <integer>] [-m|--max-hops <integer>]
                 [--max-attempts <integer>] [-d|--data-provider
                 (IP.SB|ip.sb|IPInfo|ipinfo|IPInsight|ipinsight|IPAPI.com|ip-api.com|IPInfoLocal|ipinfolocal|chunzhen|LeoMoeAPI|leomoeapi|ipdb.one|disable-geoip)]
                 [--pow-provider (api.nxtrace.org|sakura)] [-n|--no-rdns]
                 [-a|--always-rdns] [-P|--route-path] [-r|--report] [--dn42]
                 [-o|--output] [-t|--table] [--raw] [-j|--json] [-c|--classic]
                 [-f|--first <integer>] [-M|--map] [-e|--disable-mpls]
                 [-V|--version] [-s|--source "<value>"] [--source-port
                 <integer>] [-D|--dev "<value>"] [--listen "<value>"]
                 [--deploy] [-z|--send-time <integer>] [-i|--ttl-time
                 <integer>] [--timeout <integer>] [--psize <integer>]
                 [_positionalArg_nexttrace_38 "<value>"] [--dot-server
                 (dnssb|aliyun|dnspod|google|cloudflare)] [-g|--language
                 (en|cn)] [--file "<value>"] [-C|--no-color] [--from "<value>"]

Arguments:

  -h  --help                         Print help information
      --init                         Windows ONLY: Extract WinDivert runtime to
                                     current directory
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
                                     for ICMP packets (1=Socket, 2=PCAP;
                                     0=Auto)
  -q  --queries                      Set the number of probes per each hop.
                                     Default: 3
      --parallel-requests            Set ParallelRequests number. It should be
                                     1 when there is a multi-routing. Default:
                                     18
  -m  --max-hops                     Set the max number of hops (max TTL to be
                                     reached). Default: 30
      --max-attempts                 Set the max number of attempts per TTL
                                     (instead of a fixed auto value)
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
  -r  --report                       output using report mode
      --dn42                         DN42 Mode
  -o  --output                       Write trace result to file
                                     (RealTimePrinter ONLY)
  -t  --table                        Output trace results as table
      --raw                          An Output Easy to Parse
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
  -D  --dev                          Use the following Network Devices as the
                                     source address in outgoing packets
      --listen                       Set listen address for web console (e.g.
                                     127.0.0.1:30080)
      --deploy                       Start the Gin powered web console
  -z  --send-time                    Set how many [milliseconds] between
                                     sending each packet. Useful when some
                                     routers use rate-limit for ICMP messages.
                                     Default: 50
  -i  --ttl-time                     Set how many [milliseconds] between
                                     sending packets groups by TTL. Useful when
                                     some routers use rate-limit for ICMP
                                     messages. Default: 50
      --timeout                      The number of [milliseconds] to keep probe
                                     sockets open before giving up on the
                                     connection. Default: 1000
      --psize                        Set the payload size. Default: 52
      --_positionalArg_nexttrace_38  IP Address or domain name
      --dot-server                   Use DoT Server for DNS Parse [dnssb,
                                     aliyun, dnspod, google, cloudflare]
  -g  --language                     Choose the language for displaying [en,
                                     cn]. Default: cn
      --file                         Read IP Address or domain name from file
  -C  --no-color                     Disable Colorful Output
      --from                         Run traceroute via Globalping
                                     (https://globalping.io/network) from a
                                     specified location. The location field
                                     accepts continents, countries, regions,
                                     cities, ASNs, ISPs, or cloud regions.
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

[Globalping](https://globalping.io) An open source and free project that provides global access to run network tests like traceroute

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
    >- [IP 错误报告汇总帖](https://github.com/orgs/nxtrace/discussions/222) in the GITHUB ISSUES section of this project (Recommended)
    >- This project's dedicated correction email: `correct#nxtrace.org` (Please note that this email is only for correcting IP-related information. For other feedback, please submit an ISSUE)


- How to obtain the freshly baked binary executable of the latest commit?
    > Please go to the most recent [Build & Release](https://github.com/nxtrace/NTrace-V1/actions/workflows/build.yml) workflow in GitHub Actions.

- Known Issues
    + On Windows, ICMP mode requires manual firewall allowance for ICMP/ICMPv6
    + On Windows, TCP modes are currently unavailable
    + On macOS, only ICMP mode does not require elevated privileges
    + In some cases, running multiple instances of NextTrace simultaneously may interfere with each other’s results (observed so far only in TCP mode)

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=nxtrace/NTrace-core&type=Date)](https://star-history.com/#nxtrace/NTrace-core&Date)
