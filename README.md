<div align="center">

<img src="asset/logo.png" height="200px" alt="NextTrace Logo"/>

</div>

## NextTrace

An open source visual routing tool that pursues light weight, developed using Golang.

NextTrace is part of the [OwO Network](https://github.com/OwO-Network) project. The project is a joint initiative of Leo and Vincent. 

If you like this project, [Donate us](https://afdian.net/a/sjlleo/plan) to help us to provide long-lasting API cost expenses.

## How To Use

Document Language: English | [简体中文](README_zh_CN.md)

### Automated Installation

```bash
# Linux one-click install script
bash -c "$(curl -Ls https://raw.githubusercontent.com/sjlleo/nexttrace/main/nt_install.sh)"

# macOS brew install command
brew tap xgadget-lab/nexttrace && brew install nexttrace

# GHProxy Mirror (For China Mainland User)
bash -c "$(curl -Ls https://ghproxy.com/https://raw.githubusercontent.com/sjlleo/nexttrace/main/nt_install.sh)"
```

Windows users please go to [Release Page](https://github.com/sjlleo/nexttrace/releases/latest) directly and download exe file.

- `Release` provides compiled executables for many systems and architectures, if not, you can compile it yourself.
- Some of the necessary dependencies of this project are not fully implemented in `Golang` on `Windows`, so currently `NextTrace` is experimental on `Windows` platform.

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

All NextTrace IP geolocation `API DEMO` can refer to [here](https://github.com/xgadget-lab/nexttrace/blob/main/ipgeo/)

### For full usage list, please refer to the usage menu

```shell
Usage: nexttrace [-h|--help] [-T|--tcp] [-U|--udp] [-F|--fast-trace] [-p|--port
                 <integer>] [-q|--queries <integer>] [--parallel-requests
                 <integer>] [-m|--max-hops <integer>] [-d|--data-provider
                 (Ip2region|ip2region|IP.SB|ip.sb|IPInfo|ipinfo|IPInsight|ipinsight|IPAPI.com|ip-api.com|IPInfoLocal|ipinfolocal|chunzhen|LeoMoeAPI|leomoeapi|disable-geoip)]
                 [-n|--no-rdns] [-a|--always-rdns] [-P|--route-path]
                 [-r|--report] [--dn42] [-o|--output] [-t|--table] [--raw]
                 [-j|--json] [-c|--classic] [-f|--first <integer>] [-M|--map]
                 [-v|--version] [-s|--source "<value>"] [-D|--dev "<value>"]
                 [-R|--route] [-z|--send-time <integer>] [-i|--ttl-time
                 <integer>] [_positionalArg_nexttrace_27 "<value>"]
                 [--dot-server (dnssb|aliyun|dnspod|google|cloudflare)]
                 [-g|--language (en|cn)]

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
  -v  --version                      Print version info and exit
  -s  --source                       Use source src_addr for outgoing packets
  -D  --dev                          Use the following Network Devices as the
                                     source address in outgoing packets
  -R  --route                        Show Routing Table [Provided By BGP.Tools]
  -z  --send-time                    Set the time interval for sending every
                                     packet. Useful when some routers use
                                     rate-limit for ICMP messages. Default: 100
  -i  --ttl-time                     Set the time interval for sending packets
                                     groups by TTL. Useful when some routers
                                     use rate-limit for ICMP messages. Default:
                                     500
      --_positionalArg_nexttrace_25  IP Address or domain name
      --dot-server                   Use DoT Server for DNS Parse [dnssb,
                                     aliyun, dnspod, google, cloudflare]
  -g  --language                     Choose the language for displaying [en,
                                     cn]. Default: cn
```

## Project screenshot

![image](https://user-images.githubusercontent.com/13616352/216064486-5e0a4ad5-01d6-4b3c-85e9-2e6d2519dc5d.png)

![image](https://user-images.githubusercontent.com/59512455/218501311-1ceb9b79-79e6-4eb6-988a-9d38f626cdb8.png)

## NextTrace Enhanced

`NextTrace Enhanced` is an enhanced version for enthusiasts, `Enhanced` provides trace route calls in the form of Web API and a simple Looking Glass webpage with built-in visualization.

Please Notice that `NextTrace Enhanced` is currently not supported in English.

https://github.com/OwO-Network/nexttrace-enhanced

## LeoMoeAPI Credit

NextTrace focuses on Golang Traceroute implementations, and its LeoMoeAPI geolocation information is not supported by raw data, so a commercial version is not possible.

The LeoMoeAPI data is subject to copyright restrictions from multiple data sources, and is only used for the purpose of displaying the geolocation of route tracing.

1. We would like to credit samleong123 for providing nodes in Malaysia, TOHUNET Looking Glass for global nodes, and Ping.sx from Misaka, where more than 80% of reliable calibration data comes from ping/mtr reports.

2. At the same time, we would like to credit isyekong for their contribution on rDNS-based calibration ideas and data. LeoMoeAPI is accelerating the development of rDNS resolution function, and has already achieved automated geolocation resolution for some backbone networks, but there are some misjudgments. We hope that NextTrace will become a One-Man ISP-friendly traceroute tool in the future, and we are working on improving the calibration of these ASN micro-backbones as much as possible.

3. In terms of development, I would like to credit missuo and zhshch for their help with Go cross-compilation, design concepts and TCP/UDP Traceroute refactoring, and tsosunchia for their support on TraceMap.

4. I would also like to credit FFEE_CO, TheresaQWQ, stydxm and others for their help. leoMoeAPI has received a lot of support since its first release, so I would like to credit them all!

We hope you can give us as much feedback as possible on IP geolocation errors (see issue) so that it can be calibrated in the first place and others can benefit from it.


## JetBrain Support

#### This Project uses [JetBrain Open-Source Project License](https://jb.gg/OpenSourceSupport). We Proudly Develop By Goland.

<img src="https://resources.jetbrains.com/storage/products/company/brand/logos/GoLand.png" title="" alt="GoLand logo" width="331">

## Credits

BGP.TOOLS provided some data support for this project and we would like to express our sincere gratitude.

[Vincent Young](https://github.com/missuo) (i@yyt.moe)

[Sam Sam](https://github.com/samleong123) (samsam123@samsam123.name.my)

[tsosunchia](https://github.com/tsosunchia)

[waiting4new](https://github.com/waiting4new)

[FFEE_CO](https://github.com/fkx4-p) 

### Others

Although other third-party APIs are integrated in this project, please refer to the official website of the third-party APIs for specific TOS and AUP. If you encounter IP data errors, please contact them directly to correct them.

For feedback related to corrections about IP information, we currently have two channels available:
>- [IP 错误报告汇总帖](https://github.com/sjlleo/nexttrace/issues/41) in the GITHUB ISSUES section of this project (Recommended)
>- This project's dedicated correction email: `correction@moeqing.com` (Please note that this email is only for correcting IP-related information. For other feedback, please submit an ISSUE)

How to obtain the freshly baked binary executable of the latest commit?
> Please go to the most recent [Build & Release](https://github.com/sjlleo/nexttrace/actions/workflows/build.yml) workflow in GitHub Actions.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=sjlleo/nexttrace&type=Date)](https://star-history.com/#sjlleo/nexttrace&Date)
