---
name: nexttrace
description: Use NextTrace through its deploy MCP endpoint for traceroute, MTR, MTU discovery, speed tests, IP annotation, GeoIP lookup, and Globalping multi-location traceroute. Trigger when an agent needs network path diagnostics or needs to call NextTrace MCP tools.
---

# NextTrace MCP

Use this skill when the user wants route diagnostics through NextTrace or asks how an Agent should call NextTrace.

## Start MCP

NextTrace exposes MCP only as a deploy submode over HTTP. There is no stdio MCP mode.

```bash
nexttrace --deploy --mcp
nexttrace --deploy --mcp --listen 0.0.0.0:1080 --deploy-token "$TOKEN"
```

Endpoint:

```text
http://127.0.0.1:1080/mcp
```

Auth:

- Loopback listen addresses are tokenless by default unless a token is set.
- External listen addresses require a token. If none is provided, NextTrace prints a generated token to stdout.
- MCP/API clients authenticate with `Authorization: Bearer <token>` or `X-NextTrace-Token: <token>`.
- Browser WebUI uses `/auth/login` and an HttpOnly cookie.
- Do not put tokens in URL query strings.

## Workflow

1. Call `nexttrace_capabilities` first when available.
2. Pick the narrowest common tool for the job:
   - Local route: `nexttrace_traceroute`
   - Repeated loss/latency stats: `nexttrace_mtr_report`
   - Probe-level stream records: `nexttrace_mtr_raw`
   - Path MTU: `nexttrace_mtu_trace`
   - Global vantage points: `nexttrace_globalping_trace`
   - Other tools: `nexttrace_speed_test`, `nexttrace_annotate_ips`, `nexttrace_geo_lookup`, `nexttrace_globalping_limits`, `nexttrace_globalping_get_measurement`
3. Prefer `structuredContent`; use text content only as a fallback.
4. Preserve explicit user inputs: `target`, `protocol`, `port`, `source_address`, `source_device`, ASN, location, and `ip_version`. Do not substitute them unless the user asks for a fallback.
5. On errors or missing results, report the exact failure and suggested next step. Do not automatically switch protocol, port, location, ASN, tool, or local/Globalping mode.
6. For full tool schemas and boundaries, read [references/mcp-tools.md](references/mcp-tools.md) and [references/capability-matrix.md](references/capability-matrix.md).
7. For Globalping, read [references/globalping.md](references/globalping.md). For local source/device/TOS behavior, read [references/platform-notes.md](references/platform-notes.md). Use [references/cli-fallback.md](references/cli-fallback.md) only when MCP is unavailable, unsupported, or the user asks for CLI output.
8. Keep this skill and its references synced with `server/mcp.go` whenever MCP tools or parameters change.

## References

- [MCP tools](references/mcp-tools.md): tool names, inputs, outputs, unsupported parameter families.
- [Globalping](references/globalping.md): worldwide traceroute workflow and comparison guidance.
- [Capability matrix](references/capability-matrix.md): local vs Globalping vs CLI fallback.
- [CLI fallback](references/cli-fallback.md): commands to use when MCP is unavailable.
- [Platform notes](references/platform-notes.md): OS-specific source, device, TOS, raw-socket details.
- [Validation](references/validation.md): smoke checks and expected behavior.
