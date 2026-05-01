# Globalping Through NextTrace MCP

Use Globalping when the user needs traceroute/MTR from outside the local host, especially multiple countries, ASNs, ISPs, or cloud regions.

## Workflow

1. Call `nexttrace_globalping_limits`.
2. Choose a small `locations[]` set first.
3. Call `nexttrace_globalping_trace`.
4. Store `measurement_id`.
5. If more detail is needed later, call `nexttrace_globalping_get_measurement`.
6. Compare per probe: country, ASN, network, tags, raw output, and hop path.

Do not change the requested ASN, target, protocol, or IP family by yourself. If the requested ASN has no online probes or Globalping rejects it, report that exact limitation and ask before trying another ASN, country, city, protocol, or port. Switching from ICMP to TCP/UDP does not make an offline or unavailable ASN probe become available.

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

ASN-specific example:

```json
{
  "target": "w135.gubo.org",
  "locations": ["AS4837"],
  "limit": 3,
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

For ASN requests:

- Use the exact uppercase form, for example `AS4837`.
- Do not replace `AS4837` with a nearby ASN such as `AS9929`.
- Verify each returned `results[].probe.asn` matches the requested ASN before summarizing.
- If Globalping returns fewer probes than `limit`, state the actual `probes_count`; do not treat it as a protocol failure.
- If `results[].status` is `in-progress`, use `nexttrace_globalping_get_measurement` with `measurement_id` before giving a final route conclusion.

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

For final answers, prefer the probe-by-probe template in [output-templates.md](output-templates.md#nexttrace_globalping_trace).

## Boundaries

Globalping does not use local network stack controls:

- no `source_address`
- no `source_device`
- no `dot_server`
- no `packet_size`
- no `tos`
- no `ttl_interval`

For local source/device/TOS experiments, use `nexttrace_traceroute`, `nexttrace_mtr_report`, or CLI fallback.
