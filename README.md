<div align="center">

<img src="asset/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

<h1 align="center">
  <br>NextTrace<br>
</h1>

<h4 align="center">An open source visual routing tool that pursues light weight, developed using Golang.</h4>

---------------------------------------

<h6 align="center">HomePage: www.nxtrace.org</h6>

<p align="center">
  <a href="https://github.com/nxtrace/Ntrace-V1/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/nxtrace/Ntrace-V1/build.yml?branch=main&style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/nxtrace/Ntrace-V1">
    <img src="https://goreportcard.com/badge/github.com/nxtrace/Ntrace-V1?style=flat-square">
  </a>
  <a href="https://www.nxtrace.org/downloads">
    <img src="https://img.shields.io/github/release/nxtrace/Ntrace-V1/all.svg?style=flat-square">
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
      curl nxtrace.org/nt |bash
      ```
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
    * Deepin installation command

      ```shell
      apt install nexttrace
      ```
    * Termux installation command

      ```shell
      pkg install nexttrace-enhanced
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
    * Windows Scoop installation command
        * Scoop-extras version

             ```powershell
             scoop bucket add extras && scoop install extras/nexttrace
             ```

        * Scoop-extra is maintained by soenggam

* X-CMD
    * [x-cmd](https://x-cmd.com) is a lightweight cross-platform package manager implemented in posix shell. Quickly download and execute `nexttrace` with a single command: [`x nexttrace`](https://x-cmd.com/pkg/nexttrace)
        * You can also install `nexttrace` in the user level without requiring root privileges.
             ```shell
             x env use nexttrace
             ```

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

# Disable Path Visualization With the -M parameter
nexttrace koreacentral.blob.core.windows.net
# MapTrace URL: https://api.nxtrace.org/tracemap/html/c14e439e-3250-5310-8965-42a1e3545266.html

# Disable MPLS display using the --disable-mpls / -e parameter or the NEXTTRACE_DISABLEMPLS environment variable
nexttrace --disable-mpls example.com
export NEXTTRACE_DISABLEMPLS=1
```

PS: The routing visualization drawing module was written by [@tsosunchia](https://github.com/tsosunchia), and the specific code can be viewed at [tsosunchia/traceMap](https://github.com/tsosunchia/traceMap).

Note that in LeoMoeAPI 2.0, due to the addition of geographical location data, **we have deprecated the online query part of the OpenStreetMap API in the traceMap plugin and are using location information from our own database**.

The routing visualization function requires the geographical coordinates of each Hop, but third-party APIs generally do not provide this information, so this function is currently only supported when used with LeoMoeAPI.

`NextTrace` now supports quick testing, and friends who have a one-time backhaul routing test requirement can use it

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

`NextTrace` already supports route tracing for specified Network Devices

```bash
# Use eth0 network interface
nexttrace --dev eth0 2606:4700:4700::1111

# Use eth0 network interface's IP
# When using the network interface's IP for route tracing, note that the IP type to be traced should be the same as network interface's IP type (e.g. both IPv4)
nexttrace --source 204.98.134.56 9.9.9.9
```

`NextTrace` can also use `TCP` and `UDP` protocols to perform `Traceroute` requests, but `UDP` protocols only supports `IPv4` now

```bash
# TCP SYN Trace
nexttrace --tcp www.bing.com

# You can specify the port by yourself [here is 443], the default port is 80
nexttrace --tcp --port 443 2001:4860:4860::8888

# UDP Trace
nexttrace --udp 1.0.0.1

nexttrace --udp --port 53 1.0.0.1
```

`NextTrace` also supports some advanced functions, such as ttl control, concurrent probe packet count control, mode switching, etc.

```bash
# Send 2 probe packets per hop
nexttrace --queries 2 www.hkix.net

# No concurrent probe packets, only one probe packet is sent at a time
nexttrace --parallel-requests 1 www.hkix.net

# Start Trace with TTL of 5, end at TTL of 10
nexttrace --first 5 --max-hops 10 www.decix.net
# In addition, an ENV is provided to set whether to hide the destination IP
export NEXTTRACE_ENABLEHIDDENDSTIP=1

# Turn off the IP reverse parsing function
nexttrace --no-rdns www.bbix.net

# Set the payload size to 1024 bytes
nexttrace --psize 1024 example.com

# Set the payload size and DF flag for TCP Trace
nexttrace --psize 1024 --dont-fragment --tcp example.com

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
nexttrace --nocolor 1.1.1.1
# or use ENV
export NO_COLOR=1
```

`NextTrace` supports users to select their own IP API (currently supports: `LeoMoeAPI`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`, `Ip2region`, `IPInfoLocal`, `CHUNZHEN`)

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
##      For the offline database Ip2region, you can download it manually and rename it to ip2region.db, or let NextTrace download it automatically
## Please be aware: Due to the serious abuse of IP.SB, you will often be not able to query IP data from this source
## IP-API.com has a stricter restiction on API calls, if you can't query IP data from this source, please try again in a few minutes

# The Pure-FTPd IP database defaults to using http://127.0.0.1:2060 as the query interface. To customize it, please use environment variables
export NEXTTRACE_CHUNZHENURL=http://127.0.0.1:2060
## You can use https://github.com/freshcn/qqwry to build your own Pure-FTPd IP database service

# You can also specify the default IP database by setting an environment variable
export NEXTTRACE_DATAPROVIDER=ipinfo
```

`NextTrace` supports mixed parameters and shortened parameters

```bash
Example:
nexttrace --data-provider IPAPI.com --max-hops 20 --tcp --port 443 --queries 5 --no-rdns 1.1.1.1
nexttrace -tcp --queries 2 --parallel-requests 1 --table --route-path 2001:4860:4860::8888

Equivalent to:
nexttrace -d ip-api.com -m 20 -T -p 443 -q 5 -n 1.1.1.1
nexttrace -T -q 2 --parallel-requests 1 -t -P 2001:4860:4860::8888
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
Usage: nexttrace [-h|--help] [-4|--ipv4] [-6|--ipv6] [-T|--tcp] [-U|--udp]
                 [-F|--fast-trace] [-p|--port <integer>] [-q|--queries
                 <integer>] [--parallel-requests <integer>] [-m|--max-hops
                 <integer>] [-d|--data-provider
                 (Ip2region|ip2region|IP.SB|ip.sb|IPInfo|ipinfo|IPInsight|ipinsight|IPAPI.com|ip-api.com|IPInfoLocal|ipinfolocal|chunzhen|LeoMoeAPI|leomoeapi|disable-geoip)]
                 [--pow-provider (api.nxtrace.org|sakura)] [-n|--no-rdns]
                 [-a|--always-rdns] [-P|--route-path] [-r|--report] [--dn42]
                 [-o|--output] [-t|--table] [--raw] [-j|--json] [-c|--classic]
                 [-f|--first <integer>] [-M|--map] [-e|--disable-mpls]
                 [-v|--version] [-s|--source "<value>"] [-D|--dev "<value>"]
                 [-z|--send-time <integer>] [-i|--ttl-time <integer>]
                 [--timeout <integer>] [--psize <integer>]
                 [_positionalArg_nexttrace_32 "<value>"] [--dot-server
                 (dnssb|aliyun|dnspod|google|cloudflare)] [-g|--language
                 (en|cn)] [--file "<value>"] [-C|--nocolor]

Arguments:

  -h  --help                         Print help information
  -4  --ipv4                         Use IPv4 only
  -6  --ipv6                         Use IPv6 only
  -T  --tcp                          Use TCP SYN for tracerouting (default port
                                     is 80)
  -U  --udp                          Use UDP SYN for tracerouting (default port
                                     is 33494)
  -F  --fast-trace                   One-Key Fast Trace to China ISPs
  -p  --port                         Set the destination port to use. With
                                     default of 80 for "tcp", 33494 for "udp"
  -q  --queries                      Set the number of probes per each hop.
                                     Default: 3
      --parallel-requests            Set ParallelRequests number. It should be
                                     1 when there is a multi-routing. Default:
                                     18
  -m  --max-hops                     Set the max number of hops (max TTL to be
                                     reached). Default: 30
  -d  --data-provider                Choose IP Geograph Data Provider [IP.SB,
                                     IPInfo, IPInsight, IP-API.com, Ip2region,
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
  -f  --first                        Start from the first_ttl hop (instead from
                                     1). Default: 1
  -M  --map                          Disable Print Trace Map
  -e  --disable-mpls                 Disable MPLS
  -v  --version                      Print version info and exit
  -s  --source                       Use source src_addr for outgoing packets
  -D  --dev                          Use the following Network Devices as the
                                     source address in outgoing packets
  -z  --send-time                    Set how many [milliseconds] between
                                     sending each packet.. Useful when some
                                     routers use rate-limit for ICMP messages.
                                     Default: 50
  -i  --ttl-time                     Set how many [milliseconds] between
                                     sending packets groups by TTL. Useful when
                                     some routers use rate-limit for ICMP
                                     messages. Default: 50
      --timeout                      The number of [milliseconds] to keep probe
                                     sockets open before giving up on the
                                     connection.. Default: 1000
      --psize                        Set the payload size. Default: 52
      --_positionalArg_nexttrace_32  IP Address or domain name
      --dot-server                   Use DoT Server for DNS Parse [dnssb,
                                     aliyun, dnspod, google, cloudflare]
  -g  --language                     Choose the language for displaying [en,
                                     cn]. Default: cn
      --file                         Read IP Address or domain name from file
  -C  --nocolor                      Disable Colorful Output
      --dont-fragment                Set the Don't Fragment bit (IPv4 TCP
                                     only). Default: false
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


## JetBrain Support

This Project uses [JetBrain Open-Source Project License](https://jb.gg/OpenSourceSupport). We Proudly Develop By `Goland`.

<img src="https://resources.jetbrains.com/storage/products/company/brand/logos/GoLand.png" title="" alt="GoLand logo" width="331">

## Credits

[Gubo](https://www.gubo.org) Reliable Host Recommendation Website

[IPInfo](https://ipinfo.io) Provided most of the data support for this project free of charge

[BGP.TOOLS](https://bgp.tools) Provided some data support for this project free of charge

[PeeringDB](https://www.peeringdb.com) Provided some data support for this project free of charge

[sjlleo](https://github.com/sjlleo) The perpetual leader, founder, and core contributors

[tsosunchia](https://github.com/tsosunchia) The project chair, infra maintainer, and core contributors

[Vincent Young](https://github.com/missuo)

[zhshch2002](https://github.com/zhshch2002)

[Sam Sam](https://github.com/samleong123)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p)

[bobo liu](https://github.com/fakeboboliu)

[YekongTAT](https://github.com/isyekong)

### Others

Although other third-party APIs are integrated in this project, please refer to the official website of the third-party APIs for specific TOS and AUP. If you encounter IP data errors, please contact them directly to correct them.

For feedback related to corrections about IP information, we currently have two channels available:
>- [IP 错误报告汇总帖](https://github.com/orgs/nxtrace/discussions/222) in the GITHUB ISSUES section of this project (Recommended)
>- This project's dedicated correction email: `correction@nxtrace.org` (Please note that this email is only for correcting IP-related information. For other feedback, please submit an ISSUE)

How to obtain the freshly baked binary executable of the latest commit?
> Please go to the most recent [Build & Release](https://github.com/nxtrace/Ntrace-V1/actions/workflows/build.yml) workflow in GitHub Actions.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=nxtrace/NTrace-core&type=Date)](https://star-history.com/#nxtrace/NTrace-core&Date)
