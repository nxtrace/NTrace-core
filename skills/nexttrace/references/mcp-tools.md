# NextTrace MCP Tools

MCP endpoint: `/mcp` under `nexttrace --deploy --mcp`.

All tools return structured JSON under `structuredContent`.

## Tools

### `nexttrace_capabilities`

Lists tools and parameter boundaries. Call this first when the server is reachable.

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

### `nexttrace_mtr_report`

Runs bounded MTR and returns aggregated `stats[]`.

Adds:

- `hop_interval_ms`
- `max_per_hop`

Use this for loss, jitter, and repeated RTT comparison.

### `nexttrace_mtr_raw`

Runs bounded MTR raw mode and returns probe records.

Adds:

- `hop_interval_ms`
- `max_per_hop`
- `duration_ms`

If neither `max_per_hop` nor `duration_ms` is set, NextTrace bounds output with a small default and reports a warning.

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

### `nexttrace_annotate_ips`

Annotates IPv4/IPv6 literals in text.

Supported:

- `text`
- `data_provider`
- `timeout_ms`
- `language`
- `ipv4_only`
- `ipv6_only`

### `nexttrace_geo_lookup`

Looks up metadata for one IP address.

Supported:

- `query`
- `data_provider`
- `language`

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

### `nexttrace_globalping_limits`

Returns current Globalping rate/credit limits. Call this before large multi-location jobs.

### `nexttrace_globalping_get_measurement`

Fetches an existing measurement:

```json
{"measurement_id": "..." }
```
