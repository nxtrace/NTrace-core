# Validation

## Startup

Loopback tokenless default:

```bash
nexttrace --deploy --mcp
```

External authenticated:

```bash
nexttrace --deploy --mcp --listen 0.0.0.0:1080 --deploy-token "$TOKEN"
```

Expected:

- stdout prints the Web console listen URL.
- stdout prints the MCP endpoint when `--mcp` is enabled.
- external listen without manual token prints a generated token.
- manual token does not echo the token.

## Auth Smoke Checks

```bash
curl -i http://127.0.0.1:1080/api/options
curl -i -H "Authorization: Bearer $TOKEN" http://127.0.0.1:1080/api/options
curl -i -H "X-NextTrace-Token: $TOKEN" http://127.0.0.1:1080/api/options
```

Do not use query token URLs.

## MCP Smoke Checks

Use an MCP client pointed at:

```text
http://127.0.0.1:1080/mcp
```

Then call:

1. `nexttrace_capabilities`
2. `nexttrace_traceroute` with `{"target":"example.com","protocol":"icmp"}`
3. `nexttrace_globalping_limits`
4. `nexttrace_globalping_trace` with a small `locations[]` set

## Repo Tests

```bash
go build ./...
go test ./...
node --test server/web/assets/*.test.cjs
```
