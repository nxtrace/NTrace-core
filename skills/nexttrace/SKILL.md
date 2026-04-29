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
2. Pick the narrowest tool for the job:
   - Local route: `nexttrace_traceroute`
   - Repeated loss/latency stats: `nexttrace_mtr_report`
   - Probe-level stream records: `nexttrace_mtr_raw`
   - Path MTU: `nexttrace_mtu_trace`
   - Global vantage points: `nexttrace_globalping_trace`
3. Prefer `structuredContent`; use text content only as a fallback.
4. For Globalping, read [references/globalping.md](references/globalping.md).
5. For full tool schemas and boundaries, read [references/mcp-tools.md](references/mcp-tools.md) and [references/capability-matrix.md](references/capability-matrix.md).

## References

- [MCP tools](references/mcp-tools.md): tool names, inputs, outputs, unsupported parameter families.
- [Globalping](references/globalping.md): worldwide traceroute workflow and comparison guidance.
- [Capability matrix](references/capability-matrix.md): local vs Globalping vs CLI fallback.
- [CLI fallback](references/cli-fallback.md): commands to use when MCP is unavailable.
- [Platform notes](references/platform-notes.md): OS-specific source, device, TOS, raw-socket details.
- [Validation](references/validation.md): smoke checks and expected behavior.
