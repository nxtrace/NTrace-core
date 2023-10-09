<div align="center">

<img src="asset/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

<h1 align="center">
  <br>NextTrace<br>
</h1>

<h4 align="center">An open source visual routing tool that pursues light weight, developed using Golang.</h4>

<p align="center">
  <a href="https://github.com/nxtrace/Ntrace-V1/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/nxtrace/Ntrace-V1/build.yml?branch=main&style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/nxtrace/Ntrace-V1">
    <img src="https://goreportcard.com/badge/github.com/nxtrace/Ntrace-V1?style=flat-square">
  </a>
  <a href="https://github.com/nxtrace/Ntrace-V1/releases">
    <img src="https://img.shields.io/github/release/nxtrace/Ntrace-V1/all.svg?style=flat-square">
  </a>
  <a href="https://telegram.dog/sjprojects">
    <img src="https://img.shields.io/endpoint?color=neon&style=flat-square&url=https%3A%2F%2Ftg.sumanjay.workers.dev%2Fnexttrace">
  </a>
</p>

## IAAS Sponsor

<div style="text-align: center;">
    <a href="https://dmit.io">
        <img src="https://www.dmit.io/templates/dmit_theme_2020/dmit/assets/images/dmit_logo_with_text_blue.svg" width="170.7" height="62.9">
    </a>
    &nbsp;&nbsp;&nbsp;&nbsp;
    <a href="https://misaka.io" >
        <img src="https://www.jsdelivr.com/assets/8997e39e1f9d776502ab4d7cdff9d1608aa67aaf/img/globalping/sponsors/misaka.svg" width="170.7" height="62.9">
    </a>
    &nbsp;&nbsp;&nbsp;&nbsp;
    <a href="https://skywolf.cloud" >
        <img src="https://hk.skywolf.cloud/assets/img/skywolf.svg" width="170.7" height="62.9">
    </a>
</div>

We are extremely grateful to [DMIT](https://dmit.io) and [Misaka](https://misaka.io) and [Skywolf](https://skywolf.cloud) for providing the network infrastructure that powers this project.

## How To Use

Document Language: English | [简体中文](README_zh_CN.md)

Regarding the NTrace-V1 and NTrace-core repositories:<br>
Both will largely remain consistent with each other. All development work is done within the NTrace-V1 repository. The NTrace-V1 repository releases new versions first. After running stably for an undetermined period, we will synchronize that version to NTrace-core. This means that the NTrace-V1 repository serves as a "beta" or "testing" version.<br>
Please note, there are exceptions to this synchronization. If a version of NTrace-V1 encounters a serious bug, NTrace-core will skip that flawed version and synchronize directly to the next version that resolves the issue.

### Automated Install

* Linux
    * One-click installation script

      ```shell
      bash -c "$(curl http://nexttrace-io-leomoe-api-a0.shop/nt_install_v1.sh)"
      ```
    * Arch Linux AUR installation command
        * Directly download bin package (only supports amd64)

             ```shell
             yay -S nexttrace-bin
             ```
        * The AUR builds are maintained by ouuan
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

    Please note, the repositories for all of the above installation methods are maintained by open source enthusiasts. Availability and timely updates are not guaranteed. If you encounter problems, please contact the repository maintainer to solve them, or use the binary packages provided by the official build of this project.

### Manual Install
* Download the precompiled executable

    For users not covered by the above methods, please go directly to [Release](https://github.com/nxtrace/Ntrace-V1/releases/latest) to download the compiled binary executable.

    * `Release` provides compiled binary executables for many systems and different architectures. If none are available, you can compile it yourself.
    * Some essential dependencies of this project are not fully implemented on `Windows` by `Golang`, so currently, `NextTrace` is in an experimental support phase on the `Windows` platform.

* Install from source

    After installing Go >= 1.20 yourself, you can use the following command to install

    ```shell
    go install github.com/nxtrace/Ntrace-V1@latest
    ```
    After installation, the executable is in the `$GOPATH/bin` directory. If you have not set `GOPATH`, it is in the `$HOME/go/bin` directory.


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

# IPv4/IPv6 Resolve Only
nexttrace --ipv4 g.co
nexttrace --ipv6 g.co

# IPv6 ICMP Trace
nexttrace 2606:4700:4700::1111

# Disable Path Visualization With the -M parameter
nexttrace koreacentral.blob.core.windows.net
# MapTrace URL: https://api.leo.moe/tracemap/html/c14e439e-3250-5310-8965-42a1e3545266.html

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
```

`NextTrace` supports users to select their own IP API (currently supports: `LeoMoeAPI`, `IP.SB`, `IPInfo`, `IPInsight`, `IPAPI.com`, `Ip2region`, `IPInfoLocal`, `CHUNZHEN`)

```bash
# You can specify the IP database by yourself [IP-API.com here], if not specified, LeoMoeAPI will be used
nexttrace --data-provider ip-api.com
## Note There are frequency limits for free queries of the ipinfo and IPInsight APIs. You can purchase services from these providers to remove the limits
##      If necessary, you can clone this project, add the token provided by ipinfo or IPInsight and compile it yourself
## Note For the offline database IPInfoLocal, please download it manually and rename it to ipinfoLocal.mmdb. (You can download it from here: https://ipinfo.io/signup?ref=free-database-downloads)
##      For the offline database Ip2region, you can download it manually and rename it to ip2region.db, or let NextTrace download it automatically
## Fill the token to: ipgeo/tokens.go
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
nexttrace -d IPAPI.com -m 20 -T -p 443 -q 5 -n 1.1.1.1
nexttrace -T -q 2 --parallel-requests 1 -t -R 2001:4860:4860::8888
```

### IP Database

#### We use [bgp.tools](https://bgp.tools) as a data provider for routing tables.

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
                 [-n|--no-rdns] [-a|--always-rdns] [-P|--route-path]
                 [-r|--report] [--dn42] [-o|--output] [-t|--table] [--raw]
                 [-j|--json] [-c|--classic] [-f|--first <integer>] [-M|--map]
                 [-v|--version] [-s|--source "<value>"] [-D|--dev "<value>"]
                 [-R|--route] [-z|--send-time <integer>] [-i|--ttl-time
                 <integer>] [--timeout <integer>] [--psize <integer>]
                 [_positionalArg_nexttrace_31 "<value>"] [--dot-server
                 (dnssb|aliyun|dnspod|google|cloudflare)] [-g|--language
                 (en|cn)]

Arguments:

  -h  --help                         Print help information
  -4  --ipv4                         Use IPv4 only
  -6  --ipv6                         Use IPv6 only
  -T  --tcp                          Use TCP SYN for tracerouting (default port
                                     is 80)
  -U  --udp                          Use UDP SYN for tracerouting (default port
                                     is 53)
  -F  --fast-trace                   One-Key Fast Trace to China ISPs
  -p  --port                         Set the destination port to use. It is
                                     either initial udp port value for
                                     "default"method (incremented by each
                                     probe, default is 33434), or initial seq
                                     for "icmp" (incremented as well, default
                                     from 1), or some constantdestination port
                                     for other methods (with default of 80 for
                                     "tcp", 53 for "udp", etc.)
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
      --pow-provider                 Choose PoW Provider [api.leo.moe, sakura]
                                     For China mainland users, please use
                                     sakura. Default: api.leo.moe
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
  -R  --route                        Show Routing Table [Provided By BGP.Tools]
  -z  --send-time                    Set how many [milliseconds] between
                                     sending each packet.. Useful when some
                                     routers use rate-limit for ICMP messages.
                                     Default: 100
  -i  --ttl-time                     Set how many [milliseconds] between
                                     sending packets groups by TTL. Useful when
                                     some routers use rate-limit for ICMP
                                     messages. Default: 500
      --timeout                      The number of [milliseconds] to keep probe
                                     sockets open before giving up on the
                                     connection.. Default: 1000
      --psize                        Set the packet size (payload size).
                                     Default: 52
      --_positionalArg_nexttrace_31  IP Address or domain name
      --dot-server                   Use DoT Server for DNS Parse [dnssb,
                                     aliyun, dnspod, google, cloudflare]
  -g  --language                     Choose the language for displaying [en,
                                     cn]. Default: cn
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

## LeoMoeAPI Credit

NextTrace focuses on Golang Traceroute implementations, and its LeoMoeAPI geolocation information is not supported by raw data, so a commercial version is not possible.

The LeoMoeAPI data is subject to copyright restrictions from multiple data sources, and is only used for the purpose of displaying the geolocation of route tracing.

1. We would like to credit samleong123 for providing nodes in Malaysia, TOHUNET Looking Glass for global nodes, and Ping.sx from Misaka, where more than 80% of reliable calibration data comes from ping/mtr reports.

2. At the same time, we would like to credit isyekong for their contribution to rDNS-based calibration ideas and data. LeoMoeAPI is accelerating the development of rDNS resolution function, and has already achieved automated geolocation resolution for some backbone networks, but there are some misjudgments. We hope that NextTrace will become a One-Man ISP-friendly traceroute tool in the future, and we are working on improving the calibration of these ASN micro-backbones as much as possible.

3. In terms of development, I would like to credit missuo and zhshch for their help with Go cross-compilation, design concepts and TCP/UDP Traceroute refactoring, and tsosunchia for their support on TraceMap.

4. I would also like to credit FFEE_CO, TheresaQWQ, stydxm and others for their help. leoMoeAPI has received a lot of support since its first release, so I would like to credit them all!

We hope you can give us as much feedback as possible on IP geolocation errors (see issue) so that it can be calibrated in the first place and others can benefit from it.


## JetBrain Support

#### This Project uses [JetBrain Open-Source Project License](https://jb.gg/OpenSourceSupport). We Proudly Develop By Goland.

<img src="https://resources.jetbrains.com/storage/products/company/brand/logos/GoLand.png" title="" alt="GoLand logo" width="331">

## Credits

[sjlleo](https://github.com/sjlleo) The perpetual leader, founder, and core contributors of the NextTrace Project

[BGP.TOOLS](https://bgp.tools) provided some data support for this project. And we would like to express our sincere gratitude.

[Vincent Young](https://github.com/missuo) (i@yyt.moe)

[Sam Sam](https://github.com/samleong123) (samsam123@samsam123.name.my)

[tsosunchia](https://github.com/tsosunchia)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p)

### Others

Although other third-party APIs are integrated in this project, please refer to the official website of the third-party APIs for specific TOS and AUP. If you encounter IP data errors, please contact them directly to correct them.

For feedback related to corrections about IP information, we currently have two channels available:
>- [IP 错误报告汇总帖](https://github.com/nxtrace/NTrace-core/issues/41) in the GITHUB ISSUES section of this project (Recommended)
>- This project's dedicated correction email: `correction@moeqing.com` (Please note that this email is only for correcting IP-related information. For other feedback, please submit an ISSUE)

How to obtain the freshly baked binary executable of the latest commit?
> Please go to the most recent [Build & Release](https://github.com/nxtrace/Ntrace-V1/actions/workflows/build.yml) workflow in GitHub Actions.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=nxtrace/NTrace-core&type=Date)](https://star-history.com/#nxtrace/NTrace-core&Date)
