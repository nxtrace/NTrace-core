# NextTrace MCP Tools

MCP endpoint: `/mcp` under `nexttrace --deploy --mcp`.

All tools return structured JSON under `structuredContent`.

For every tool, respect the returned or documented `parameters.supported`, `parameters.not_applicable`, and `parameters.not_yet_supported` boundaries. Do not pass unsupported families just because another NextTrace tool accepts them.

## Tools

### `nexttrace_capabilities`

Lists tools and parameter boundaries. Call this first when the server is reachable.

Respect the capability output when choosing tools. Do not infer that a parameter supported by one tool is available in another.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_capabilities).

### `nexttrace_traceroute`

Runs local NextTrace traceroute.

Supported:

```json
{
  "target": "example.com",
  "protocol": "icmp|tcp|udp",
  "port": 443,
  "queries": 3,
  "max_hops": 30,
  "timeout_ms": 1000,
  "packet_size": 64,
  "tos": 0,
  "parallel_requests": 18,
  "begin_hop": 1,
  "ipv4_only": false,
  "ipv6_only": false,
  "data_provider": "LeoMoeAPI|IP.SB|IPInfo|IPInfoLocal|IPInsight|ip-api.com|chunzhen|DN42|disable-geoip|ipdb.one",
  "pow_provider": "api.nxtrace.org|sakura",
  "dot_server": "dnssb|aliyun|dnspod|google|cloudflare",
  "disable_rdns": false,
  "always_rdns": false,
  "disable_maptrace": true,
  "disable_mpls": false,
  "language": "cn|en",
  "source_address": "192.0.2.10",
  "source_port": 0,
  "source_device": "eth0",
  "icmp_mode": 0,
  "packet_interval": 50,
  "ttl_interval": 300,
  "max_attempts": 0
}
```

Output includes `target`, `resolved_ip`, `protocol`, `data_provider`, `language`, `hops[]`, and `duration_ms`.

Respect its parameter boundaries. Do not switch from ICMP to TCP/UDP because some hops drop packets; ask or report the limitation first. Keep explicit TCP/UDP ports, and remember omitted ports default to TCP `80` and UDP `33494`.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_traceroute).

### `nexttrace_mtr_report`

Runs bounded MTR and returns aggregated `stats[]`.

Adds:

- `hop_interval_ms`
- `max_per_hop`

Use this for loss, jitter, and repeated RTT comparison.

Respect its parameter boundaries. Use this for repeated local statistics, not for worldwide probe selection. Do not summarize a lossy intermediate hop as destination failure without checking later hops and final-hop stats.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_mtr_report).

### `nexttrace_mtr_raw`

Runs bounded MTR raw mode and returns probe records.

Adds:

- `hop_interval_ms`
- `max_per_hop`
- `duration_ms`

If neither `max_per_hop` nor `duration_ms` is set, NextTrace bounds output with a small default and reports a warning.

Respect its parameter boundaries. Raw output is probe-level records, not a final path table. Accept the bounded default or set `max_per_hop` / `duration_ms`; do not request unbounded raw streams through MCP.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_mtr_raw).

### `nexttrace_mtu_trace`

Runs UDP path-MTU discovery.

Supported:

- `target`
- `port`
- `queries`
- `max_hops`
- `begin_hop`
- `timeout_ms`
- `ttl_interval_ms`
- `ipv4_only`
- `ipv6_only`
- `data_provider`
- `dot_server`
- `disable_rdns`
- `always_rdns`
- `language`
- `source_address`
- `source_port`
- `source_device`

Not applicable:

- `protocol`
- `packet_size`
- `tos`

Respect these boundaries. MTU is UDP-only; do not pass `protocol`, `packet_size`, or `tos`. MTU failure indicates path-MTU discovery could not complete, not that normal traceroute or the destination is necessarily down.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_mtu_trace).

### `nexttrace_speed_test`

Runs a conservative local speed test.

Supported:

- `provider`: `apple` or `cloudflare`
- `max`
- `timeout_ms`
- `threads`
- `latency_count`
- `endpoint_ip`
- `no_metadata`
- `language`
- `dot_server`
- `source_address`
- `source_device`

Respect its parameter boundaries. Use speed test only for bandwidth/latency-to-test-endpoint questions. It is local HTTP transfer testing, not route diagnostics and not Globalping.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_speed_test).

### `nexttrace_annotate_ips`

Annotates IPv4/IPv6 literals in text.

Supported:

- `text`
- `data_provider`
- `timeout_ms`
- `language`
- `ipv4_only`
- `ipv6_only`

Respect its parameter boundaries. This tool annotates IP literals already present in text; it does not resolve domains, run traceroute, or validate reachability.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_annotate_ips).

### `nexttrace_geo_lookup`

Looks up metadata for one IP address.

Supported:

- `query`
- `data_provider`
- `language`

Respect its parameter boundaries. `query` must be an IP address. For a domain, first use a trace or resolver path that returns an IP, then call this tool if a separate GeoIP lookup is still needed.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_geo_lookup).

### `nexttrace_globalping_trace`

Runs a Globalping multi-probe measurement. See [globalping.md](globalping.md).

Supported:

- `target`
- `locations[]`
- `limit`
- `protocol`
- `port`
- `packets`
- `ip_version`

Not applicable:

- local `source_address`
- local `source_device`
- `dot_server`
- `packet_size`
- `tos`
- `ttl_interval`

Respect these boundaries. Summarize per `results[].probe` and verify requested ASN/location constraints against returned probe metadata. Do not use Globalping for local `source_address`, `source_device`, `dot_server`, `packet_size`, `tos`, or TTL-interval experiments.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_globalping_trace).

### `nexttrace_globalping_limits`

Returns current Globalping rate/credit limits. Call this before large multi-location jobs.

Respect its parameter boundaries. This tool takes no target; do not use it as a reachability probe.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_globalping_limits).

### `nexttrace_globalping_get_measurement`

Fetches an existing measurement:

```json
{"measurement_id": "..." }
```

Respect its parameter boundaries. Use it only with a `measurement_id` returned by `nexttrace_globalping_trace`; do not change the original target/location/protocol while polling.

Final answer shape: use [output-templates.md](output-templates.md#nexttrace_globalping_trace).
