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

## Supported vs Not Applicable vs Not Yet Supported

Each tool reports parameter boundaries in output:

- `supported`: accepted by this tool.
- `not_applicable`: meaningful for another tool or local stack, but not this tool.
- `not_yet_supported`: planned or useful, but not implemented as MCP input.

Current important gaps:

- No Globalping location search/list MCP tool.
- MCP transport is HTTP deploy mode only, not stdio.
