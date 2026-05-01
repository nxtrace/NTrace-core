# Capability Matrix

| Need | MCP Tool | Notes |
| --- | --- | --- |
| List tools and parameter support | `nexttrace_capabilities` | Call first to discover available tools |
| One local path trace | `nexttrace_traceroute` | ICMP/TCP/UDP, GeoIP, RDNS, MPLS, source controls |
| Repeated local loss/latency stats | `nexttrace_mtr_report` | Bounded MTR report, structured stats |
| Probe-level local stream records | `nexttrace_mtr_raw` | Bound with `max_per_hop` or `duration_ms` |
| Local path MTU | `nexttrace_mtu_trace` | UDP only; no `packet_size` or `tos` |
| Local speed test | `nexttrace_speed_test` | Conservative defaults for Agent usage |
| Annotate text containing IPs | `nexttrace_annotate_ips` | Preserves original text with metadata annotations |
| Single IP GeoIP | `nexttrace_geo_lookup` | IP literals only |
| Worldwide route comparison | `nexttrace_globalping_trace` | Globalping probes by magic location strings |
| Existing Globalping result | `nexttrace_globalping_get_measurement` | Requires `measurement_id` |
| Globalping rate budget | `nexttrace_globalping_limits` | Call before wide jobs |

## Common Wrong Tool Choices

| User intent | Use | Do not use |
| --- | --- | --- |
| Trace from worldwide countries, cities, ASNs, ISPs, or cloud regions | `nexttrace_globalping_trace` | Local `source_device` / `source_address` parameters |
| Trace from this machine, specific source IP, or specific network device | `nexttrace_traceroute`, `nexttrace_mtr_report`, `nexttrace_mtr_raw`, or `nexttrace_mtu_trace` | Globalping |
| Annotate pasted logs or text containing IPs | `nexttrace_annotate_ips` | `nexttrace_geo_lookup` per token unless single-IP lookup is requested |
| Look up one known IP address | `nexttrace_geo_lookup` | Traceroute, MTR, or Globalping |
| Measure CDN/download throughput | `nexttrace_speed_test` | Traceroute or Globalping |

Do not translate local source/device/TOS requirements into Globalping requests. Globalping probes are remote machines selected by location magic strings.

## Supported vs Not Applicable vs Not Yet Supported

Each tool reports parameter boundaries in output:

- `supported`: accepted by this tool.
- `not_applicable`: meaningful for another tool or local stack, but not this tool.
- `not_yet_supported`: planned or useful, but not implemented as MCP input.

Current important gaps:

- No Globalping location search/list MCP tool.
- MCP transport is HTTP deploy mode only, not stdio.
