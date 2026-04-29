# Globalping Through NextTrace MCP

Use Globalping when the user needs traceroute/MTR from outside the local host, especially multiple countries, ASNs, ISPs, or cloud regions.

## Workflow

1. Call `nexttrace_globalping_limits`.
2. Choose a small `locations[]` set first.
3. Call `nexttrace_globalping_trace`.
4. Store `measurement_id`.
5. If more detail is needed later, call `nexttrace_globalping_get_measurement`.
6. Compare per probe: country, ASN, network, tags, raw output, and hop path.

## Example

```json
{
  "target": "example.com",
  "locations": ["Japan", "Germany", "AS13335", "aws-us-east-1"],
  "limit": 4,
  "protocol": "ICMP",
  "packets": 3,
  "ip_version": 4
}
```

## Location Strings

This integration currently uses Globalping magic strings. There is no MCP location search/list tool yet.

Useful magic string forms:

- Continent or country: `Europe`, `Japan`, `United States`
- Region or city: `Tokyo`, `Frankfurt`, `California`
- ASN: `AS13335`
- ISP or network name: `Cloudflare`, `Akamai`
- Cloud region: provider-specific strings such as `aws-us-east-1` when supported by Globalping

If a location is rejected, simplify it to country, city, or ASN.

## Output

`nexttrace_globalping_trace` returns:

- `measurement_id`
- `status`
- `probes_count`
- `results[]`

Each `results[]` item includes:

- `probe`: location, ASN, network, tags
- `status`
- `resolved_address`
- `resolved_hostname`
- `hops[]`
- `raw_output`

Use `raw_output` when Globalping has formatting or platform-specific details that are not represented in decoded hops.

## Boundaries

Globalping does not use local network stack controls:

- no `source_address`
- no `source_device`
- no `dot_server`
- no `packet_size`
- no `tos`
- no `ttl_interval`

For local source/device/TOS experiments, use `nexttrace_traceroute`, `nexttrace_mtr_report`, or CLI fallback.
