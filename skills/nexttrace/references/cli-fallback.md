# CLI Fallback

Use CLI fallback only when MCP is unreachable, the user wants terminal output, or MCP returns an explicit unsupported-capability error.

Fallback must preserve explicit user inputs: `target`, `protocol`, `port`, `source_address`, `source_device`, ASN, location, and IP family. Do not change those values unless the user agrees.

## Local Traceroute

```bash
nexttrace example.com
nexttrace --tcp -p 443 example.com
nexttrace --udp -p 33494 example.com
nexttrace --json example.com
```

## MTR

```bash
nexttrace --report example.com
nexttrace --mtr --raw -q 5 example.com
```

## MTU

`--mtu` is available in the `nexttrace` and `nexttrace-tiny` flavors. It is not supported by `ntr`.

```bash
nexttrace --mtu example.com
nexttrace --mtu --json example.com
```

## Globalping

```bash
nexttrace --from "Japan" example.com
nexttrace --from "AS13335" --tcp -p 443 example.com
```

Globalping CLI mode is single-location oriented. For Agent multi-location work, prefer MCP `nexttrace_globalping_trace.locations[]`.

## Deploy MCP

```bash
nexttrace --deploy --mcp
nexttrace --deploy --mcp --listen 0.0.0.0:1080 --deploy-token "$TOKEN"
```
