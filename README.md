<div align="center">

<img src="asset/logo.png" height="200px"/>

</div>

# NextTrace

可视化路由跟踪工具。

## How To Use

```shell
Usage of nexttrace:
  -T    Use TCP SYN for tracerouting (default port is 80)
  -U    Use UDP Package for tracerouting (default port is 53 in UDP)
  -d string
        Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight] (default "LeoMoeAPI")
  -displayMode string
        Choose The Display Mode [table, classic] (default "table")
  -m int
        Set the max number of hops (max TTL to be reached). (default 30)
  -p int
        Set SYN Traceroute Port (default 80)
  -q int
        Set the number of probes per each hop. (default 3)
  -r int
        Set ParallelRequests number. It should be 1 when there is a multi-routing. (default 18)
  -rdns
        Set whether rDNS will be display
  -report
        Route Path
```

![](asset/screenshot.png)

## Thanks

Vincent Young (i@yyt.moe)

