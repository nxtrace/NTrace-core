# Output Templates

Use these templates when turning NextTrace MCP `structuredContent` into a user-facing answer.

## Rules

- Use Markdown by default.
- Put conclusion first, then evidence.
- Preserve explicit user inputs. Do not change `target`, `protocol`, `port`, `source_address`, `source_device`, ASN, location, or `ip_version`.
- Use only fields present in `structuredContent`. Omit unknown values instead of guessing.
- Keep raw data short. Link or quote only the key lines from `raw_output` when it materially changes the conclusion.
- Choose the Chinese or English template based on the user's language. Do not output both languages unless requested.

## `nexttrace_capabilities`

中文模板:

```markdown
**可用能力**

| 工具 | 用途 | 关键参数 |
| --- | --- | --- |
| `<tool.name>` | `<tool.description>` | `<tool.parameters.supported>` |

**边界**
- 不适用: `<parameters.not_applicable>`
- 尚未支持: `<parameters.not_yet_supported>`
```

English template:

```markdown
**Available Capabilities**

| Tool | Purpose | Key Parameters |
| --- | --- | --- |
| `<tool.name>` | `<tool.description>` | `<tool.parameters.supported>` |

**Boundaries**
- Not applicable: `<parameters.not_applicable>`
- Not yet supported: `<parameters.not_yet_supported>`
```

## `nexttrace_traceroute`

中文模板:

```markdown
**结论**
<一句话说明是否到达目标、主要出口/骨干/异常点。>

**概览**

| 项目 | 值 |
| --- | --- |
| 目标 | `<target>` |
| 解析 IP | `<resolved_ip>` |
| 协议 | `<protocol>` |
| Geo 数据源 | `<data_provider>` |
| 耗时 | `<duration_ms> ms` |

**关键路径**

| TTL | IP / Host | ASN / 地理 | RTT | 说明 |
| --- | --- | --- | --- | --- |
| `<hop.ttl>` | `<attempt.ip>` / `<attempt.hostname>` | `<attempt.geo.asnumber>` `<attempt.geo.country/prov/city>` | `<attempt.rtt_ms> ms` | `<正常/超时/可能限速/MPLS>` |

**需要注意**
- `<只列异常 hop、连续超时、ASN 切换、MPLS、最终跳未达等事实。>`
- `<不要把中间 hop 丢包直接写成目标不可达。>`
```

English template:

```markdown
**Conclusion**
<One sentence on reachability, main transit path, or the likely anomaly.>

**Overview**

| Field | Value |
| --- | --- |
| Target | `<target>` |
| Resolved IP | `<resolved_ip>` |
| Protocol | `<protocol>` |
| Geo source | `<data_provider>` |
| Duration | `<duration_ms> ms` |

**Key Path**

| TTL | IP / Host | ASN / Location | RTT | Note |
| --- | --- | --- | --- | --- |
| `<hop.ttl>` | `<attempt.ip>` / `<attempt.hostname>` | `<attempt.geo.asnumber>` `<attempt.geo.country/prov/city>` | `<attempt.rtt_ms> ms` | `<normal/timeout/rate-limited/MPLS>` |

**Notes**
- `<Only list observed anomalies, consecutive timeouts, ASN changes, MPLS, or final-hop gaps.>`
- `<Do not treat intermediate-hop loss as destination failure by itself.>`
```

## `nexttrace_mtr_report`

中文模板:

```markdown
**结论**
<一句话说明最终 hop 的 loss/avg/stdev 是否可接受；如果只是中间 hop 丢包，明确说明。>

**MTR 概览**

| 项目 | 值 |
| --- | --- |
| 目标 | `<target>` |
| 解析 IP | `<resolved_ip>` |
| 协议 | `<protocol>` |
| 采样耗时 | `<duration_ms> ms` |

**最终 hop**

| TTL | Host / IP | Loss | Snt/Rcv | Last | Avg | Best/Wrst | StDev |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `<stat.ttl>` | `<stat.host>` / `<stat.ip>` | `<stat.loss_percent>%` | `<stat.snt>/<stat.received>` | `<stat.last_ms>` | `<stat.avg_ms>` | `<stat.best_ms>/<stat.wrst_ms>` | `<stat.stdev_ms>` |

**最高风险 hop**

| TTL | Host / IP | Loss | Avg | StDev | 判断 |
| --- | --- | --- | --- | --- | --- |
| `<stat.ttl>` | `<stat.host>` / `<stat.ip>` | `<stat.loss_percent>%` | `<stat.avg_ms>` | `<stat.stdev_ms>` | `<中间限速/疑似拥塞/最终丢包>` |
```

English template:

```markdown
**Conclusion**
<One sentence on final-hop loss/latency/jitter. Say clearly if loss is only on intermediate hops.>

**MTR Overview**

| Field | Value |
| --- | --- |
| Target | `<target>` |
| Resolved IP | `<resolved_ip>` |
| Protocol | `<protocol>` |
| Duration | `<duration_ms> ms` |

**Final Hop**

| TTL | Host / IP | Loss | Snt/Rcv | Last | Avg | Best/Wrst | StDev |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `<stat.ttl>` | `<stat.host>` / `<stat.ip>` | `<stat.loss_percent>%` | `<stat.snt>/<stat.received>` | `<stat.last_ms>` | `<stat.avg_ms>` | `<stat.best_ms>/<stat.wrst_ms>` | `<stat.stdev_ms>` |

**Highest-Risk Hops**

| TTL | Host / IP | Loss | Avg | StDev | Assessment |
| --- | --- | --- | --- | --- | --- |
| `<stat.ttl>` | `<stat.host>` / `<stat.ip>` | `<stat.loss_percent>%` | `<stat.avg_ms>` | `<stat.stdev_ms>` | `<intermediate rate-limit/congestion/final-hop loss>` |
```

## `nexttrace_mtr_raw`

中文模板:

```markdown
**结论**
<基于 records 的简短判断；说明这是 probe-level 采样，不是最终汇总表。>

**采样边界**

| 项目 | 值 |
| --- | --- |
| 目标 | `<target>` |
| 解析 IP | `<resolved_ip>` |
| 协议 | `<protocol>` |
| 记录数 | `<len(records)>` |
| 耗时 | `<duration_ms> ms` |
| Warning | `<warnings[]>` |

**关键记录**

| Iter | TTL | IP / Host | RTT | ASN / 地理 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `<record.iteration>` | `<record.ttl>` | `<record.ip>` / `<record.host>` | `<record.rtt_ms> ms` | `<record.asn>` `<record.country/prov/city>` | `<record.success>` |
```

English template:

```markdown
**Conclusion**
<Short interpretation from records. State that this is probe-level sampling, not a final summary table.>

**Sampling Bounds**

| Field | Value |
| --- | --- |
| Target | `<target>` |
| Resolved IP | `<resolved_ip>` |
| Protocol | `<protocol>` |
| Records | `<len(records)>` |
| Duration | `<duration_ms> ms` |
| Warnings | `<warnings[]>` |

**Key Records**

| Iter | TTL | IP / Host | RTT | ASN / Location | Status |
| --- | --- | --- | --- | --- | --- |
| `<record.iteration>` | `<record.ttl>` | `<record.ip>` / `<record.host>` | `<record.rtt_ms> ms` | `<record.asn>` `<record.country/prov/city>` | `<record.success>` |
```

## `nexttrace_mtu_trace`

中文模板:

```markdown
**结论**
<一句话说明 path_mtu；若未发现，说明 MTU 探测未完成，不等于目标不可达。>

**Path MTU**

| 项目 | 值 |
| --- | --- |
| 目标 | `<target>` |
| 解析 IP | `<resolved_ip>` |
| IP 版本 | `<ip_version>` |
| 协议 | `<protocol>` |
| 起始 MTU | `<start_mtu>` |
| 探测包大小 | `<probe_size>` |
| Path MTU | `<path_mtu>` |

**关键 hop**

| TTL | Event | IP / Host | RTT | PMTU | 说明 |
| --- | --- | --- | --- | --- | --- |
| `<hop.ttl>` | `<hop.event>` | `<hop.ip>` / `<hop.hostname>` | `<hop.rtt_ms> ms` | `<hop.pmtu>` | `<packet_too_big/frag_needed/destination/timeout>` |
```

English template:

```markdown
**Conclusion**
<One sentence with path_mtu. If unavailable, say PMTU discovery did not complete; do not claim normal reachability failed.>

**Path MTU**

| Field | Value |
| --- | --- |
| Target | `<target>` |
| Resolved IP | `<resolved_ip>` |
| IP version | `<ip_version>` |
| Protocol | `<protocol>` |
| Start MTU | `<start_mtu>` |
| Probe size | `<probe_size>` |
| Path MTU | `<path_mtu>` |

**Key Hops**

| TTL | Event | IP / Host | RTT | PMTU | Note |
| --- | --- | --- | --- | --- | --- |
| `<hop.ttl>` | `<hop.event>` | `<hop.ip>` / `<hop.hostname>` | `<hop.rtt_ms> ms` | `<hop.pmtu>` | `<packet_too_big/frag_needed/destination/timeout>` |
```

## `nexttrace_speed_test`

中文模板:

```markdown
**结论**
<一句话总结下载/上传、空载/负载延迟、是否 degraded。>

**测速环境**

| 项目 | 值 |
| --- | --- |
| Provider | `<result.config.provider>` |
| Endpoint | `<result.selected_endpoint.ip>` `<result.selected_endpoint.description>` |
| Client | `<result.connection_info.client.ip>` `<result.connection_info.client.isp/asn/location>` |
| Server | `<result.connection_info.server.ip>` `<result.connection_info.server.isp/asn/location>` |
| Degraded | `<result.degraded>` |
| 耗时 | `<result.duration_ms> ms` |

**结果**

| 轮次 | 方向 | 线程 | Mbps | Loaded latency | Faults |
| --- | --- | --- | --- | --- | --- |
| `<round.name>` | `<round.direction>` | `<round.threads>` | `<round.mbps>` | `<round.loaded_latency.median_ms> ms / jitter <round.loaded_latency.jitter_ms> ms` | `<round.fault_count>` |

**延迟**
- Idle latency: `<result.idle_latency.median_ms> ms`, jitter `<result.idle_latency.jitter_ms> ms`, samples `<result.idle_latency.samples>`
- Warnings: `<result.warnings[]>`
```

English template:

```markdown
**Conclusion**
<One sentence summarizing download/upload, idle/loaded latency, and degraded state.>

**Test Context**

| Field | Value |
| --- | --- |
| Provider | `<result.config.provider>` |
| Endpoint | `<result.selected_endpoint.ip>` `<result.selected_endpoint.description>` |
| Client | `<result.connection_info.client.ip>` `<result.connection_info.client.isp/asn/location>` |
| Server | `<result.connection_info.server.ip>` `<result.connection_info.server.isp/asn/location>` |
| Degraded | `<result.degraded>` |
| Duration | `<result.duration_ms> ms` |

**Rounds**

| Round | Direction | Threads | Mbps | Loaded latency | Faults |
| --- | --- | --- | --- | --- | --- |
| `<round.name>` | `<round.direction>` | `<round.threads>` | `<round.mbps>` | `<round.loaded_latency.median_ms> ms / jitter <round.loaded_latency.jitter_ms> ms` | `<round.fault_count>` |

**Latency**
- Idle latency: `<result.idle_latency.median_ms> ms`, jitter `<result.idle_latency.jitter_ms> ms`, samples `<result.idle_latency.samples>`
- Warnings: `<result.warnings[]>`
```

## `nexttrace_annotate_ips`

中文模板:

````markdown
**结论**
<一句话说明识别到多少个 IP，是否只覆盖 IPv4/IPv6。>

**标注结果**

```text
<text>
```

**已识别 IP**

| IP | ASN / 地理 | 说明 |
| --- | --- | --- |
| `<从 text 中提取的 IP>` | `<从标注文本中可见的 ASN/国家/城市>` | `<只写标注中可见的信息>` |
````

English template:

````markdown
**Conclusion**
<One sentence with how many IP literals were annotated and whether IPv4/IPv6 filtering applied.>

**Annotated Text**

```text
<text>
```

**Recognized IPs**

| IP | ASN / Location | Note |
| --- | --- | --- |
| `<IP extracted from text>` | `<ASN/country/city visible in annotated text>` | `<Only include visible annotation facts>` |
````

## `nexttrace_geo_lookup`

中文模板:

```markdown
**IP 信息**

| 项目 | 值 |
| --- | --- |
| IP | `<query>` |
| ASN | `<geo.asnumber>` |
| 国家/地区 | `<geo.country>` / `<geo.country_en>` |
| 省市 | `<geo.prov>` `<geo.city>` `<geo.district>` |
| Owner / ISP | `<geo.owner>` / `<geo.isp>` |
| Prefix / Domain | `<geo.prefix>` / `<geo.domain>` |
| Source | `<geo.source>` |
```

English template:

```markdown
**IP Information**

| Field | Value |
| --- | --- |
| IP | `<query>` |
| ASN | `<geo.asnumber>` |
| Country / Region | `<geo.country>` / `<geo.country_en>` |
| Province / City | `<geo.prov>` `<geo.city>` `<geo.district>` |
| Owner / ISP | `<geo.owner>` / `<geo.isp>` |
| Prefix / Domain | `<geo.prefix>` / `<geo.domain>` |
| Source | `<geo.source>` |
```

## `nexttrace_globalping_trace`

中文模板:

```markdown
**结论**
<一句话总结多 probe 的共同路径、差异、是否满足请求的 ASN/location。>

**Measurement**

| 项目 | 值 |
| --- | --- |
| ID | `<measurement_id>` |
| 状态 | `<status>` |
| 类型 | `<type>` |
| 目标 | `<target>` |
| 探针数 | `<probes_count>` |

**Probe 对比**

| Probe | ASN / Network | 解析目标 | 状态 | 关键路径 |
| --- | --- | --- | --- | --- |
| `<probe.city>, <probe.country>` | `AS<probe.asn>` `<probe.network>` | `<resolved_address>` `<resolved_hostname>` | `<status>` | `<主要 ASN/出口/终点>` |

**路径差异**
- `<按 probe 维度说明国家、ASN、云区域、出口、终点解析差异。>`
- `<如果 probe.asn 不符合用户指定 ASN/location，明确标出，不要混入结论。>`
- `<必要时引用 raw_output 的关键行。>`
```

English template:

```markdown
**Conclusion**
<One sentence summarizing common path, differences, and whether requested ASN/location constraints were met.>

**Measurement**

| Field | Value |
| --- | --- |
| ID | `<measurement_id>` |
| Status | `<status>` |
| Type | `<type>` |
| Target | `<target>` |
| Probes | `<probes_count>` |

**Probe Comparison**

| Probe | ASN / Network | Resolved Target | Status | Key Path |
| --- | --- | --- | --- | --- |
| `<probe.city>, <probe.country>` | `AS<probe.asn>` `<probe.network>` | `<resolved_address>` `<resolved_hostname>` | `<status>` | `<main ASNs/egress/destination>` |

**Path Differences**
- `<Compare country, ASN, cloud region, egress, and target resolution per probe.>`
- `<If probe.asn does not match the requested ASN/location, mark it clearly and exclude it from the main conclusion.>`
- `<Quote key raw_output lines only when needed.>`
```

## `nexttrace_globalping_limits`

中文模板:

```markdown
**Globalping 配额**

| 项目 | 值 |
| --- | --- |
| Create 类型 | `<measurements.create.type>` |
| Create 限额 | `<measurements.create.limit>` |
| Create 剩余 | `<measurements.create.remaining>` |
| Reset | `<measurements.create.reset>` |
| Credits 剩余 | `<credits.remaining>` |

**建议**
<根据 remaining/credits.remaining 说明是否适合继续大范围多测点任务。>
```

English template:

```markdown
**Globalping Limits**

| Field | Value |
| --- | --- |
| Create type | `<measurements.create.type>` |
| Create limit | `<measurements.create.limit>` |
| Create remaining | `<measurements.create.remaining>` |
| Reset | `<measurements.create.reset>` |
| Credits remaining | `<credits.remaining>` |

**Recommendation**
<Say whether a wide multi-location job is reasonable based on remaining/credits.remaining.>
```

## Error Or Fallback

中文模板:

```markdown
**未完成**
<一句话说明哪个 tool 失败。>

**原因**
- Tool: `<tool_name>`
- Error: `<error>`
- 保留的用户参数: `<target/protocol/port/source_device/ASN/location/ip_version>`

**下一步**
<说明需要用户确认的 fallback，例如换协议、换端口、换测点、改成本地 trace、或改用 CLI。不要自行执行。>
```

English template:

```markdown
**Not Completed**
<One sentence naming the failed tool.>

**Reason**
- Tool: `<tool_name>`
- Error: `<error>`
- Preserved user inputs: `<target/protocol/port/source_device/ASN/location/ip_version>`

**Next Step**
<State the fallback that needs user approval, such as changing protocol, port, location, ASN, local/Globalping mode, or CLI fallback. Do not run it silently.>
```
