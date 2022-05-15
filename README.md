# NextTrace

可视化路由跟踪工具

## How To Use

```shell
NextTrace v0.1.0 Alpha 
xgadget-lab zhshch (xzhsh.ch) & leo (leo.moe)
Usage of nexttrace:
  -T    Use TCP SYN for tracerouting (default port is 80 in TCP, 53 in UDP)
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
        Auto-Generate a Route-Path Report by Traceroute
```

