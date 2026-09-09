# CLI Fallback

Use CLI fallback only when MCP is unreachable, the user wants terminal output, or MCP returns an explicit unsupported-capability error.

Fallback must preserve explicit user inputs: `target`, `protocol`, `port`, `source_address`, `source_device`, ASN, location, and IP family. Do not change those values unless the user agrees.

## Local Traceroute

```bash
nexttrace --traceroute example.com
nexttrace --traceroute --tcp -p 443 example.com
nexttrace --traceroute --udp -p 33494 example.com
nexttrace --traceroute --json example.com
```

Full and tiny builds support `-k/--traceroute` and optional MTR. Their default remains traditional traceroute in this release; the default and bare `--raw` will switch to MTR no earlier than 2027, in a separately announced release. Use `--traceroute --raw` to pin traditional RAW (12-column success rows, historical 8-column timeout rows). Detect `--traceroute` in `--help` before using it with older binaries; do not retry failed probes with different mode flags. Explicit traditional mode conflicts with MTR/report/wide and standalone DNS/MTU/speed/nali/deploy modes.

JSON includes optional `StopReason` with lowercase nested fields `hop`, `reason`, `responses`, and `markers`. `responses` contains human-readable descriptions; `markers` contains machine-readable codes. Use `reason` as the reachability conclusion; do not compare the last hop IP with the target.

Normal traceroute output precedence is `--json` > `--table` > `--classic` > `--raw` > `--output` > realtime. A higher-priority mode that overrides an explicit output file emits a warning on stderr.

## MTR

```bash
nexttrace --report example.com
nexttrace --mtr --raw -q 5 example.com
```

When `-q` is omitted, `--mtr --raw` runs continuously, while `--report/--wide` (including with `--raw`) defaults to 10 probes per hop. Set an explicit `-q` for bounded automation. MTR raw stdout remains a fixed 12-column stream. Semantic unreachable diagnostics are written to stderr so stdout parsers remain compatible. In unbounded MTR, an unreachable edge is provisional and later transit evidence can reopen the path; use the final `path_end` from bounded structured output as the authoritative boundary.

### JSON, recording and replay

```sh
nexttrace --mtr --json -q 10 example.com
nexttrace -r --json -q 10 example.com
nexttrace --mtr --tcp --source-port -1 --mtr-record session.jsonl -q 10 example.com
nexttrace --mtr-replay session.jsonl --json
nexttrace -r --mtr-columns loss,received,avg example.com
```

All three builds support these CLI capabilities; full/tiny require explicit MTR
for live runs, while `ntr --json` defaults to streaming MTR. JSON streams and
reports have different count/error contracts. Do not use live NDJSON as a replay
file: only `--mtr-record` creates that format. Replay accepts presentation flags
only and never probes or queries metadata; it can run without network access or
probe privileges. Incomplete files return nonzero with the valid prefix marked
incomplete. Text columns do not change RAW/JSON schemas and cannot accompany
those output modes. See [JSON](../../../docs/mtr-json.md) and
[recording/replay](../../../docs/mtr-session.md) for schemas and lifecycle rules.

### Environment check and Linux marks

```sh
nexttrace --doctor --tcp --port 443 --language en example.com
nexttrace -r --json --fwmark 0x100 -q 10 example.com
```

Doctor is plain text and performs local setup checks only; DNS/DoT may access
the network. It does not send probes or verify delivery. Exit codes are 0 for
required checks passed, 1 for failure, 2 for invalid arguments, 3 for unconfirmed
required checks, and 130/143 for handled signals. On Windows, WinDivert checks
use NO_INSTALL. Doctor rejects `--fwmark` and random source port `-1`.

`--fwmark` is Linux-only local traceroute/MTR CLI input, not an MCP parameter.
Do not silently drop it or send an unmarked MCP request. It supports decimal or
hex uint32 values, including explicit zero; source/device constraints remain in
force. Mark setup errors stop probing. It does not mark DNS/Geo/API traffic.

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

### MTR column metrics

Use `--mtr-columns loss,snt,space,dropped,gmean,jitter,javg,jmax,jint` in MTR text
mode or offline replay. `space` inserts one extra display space; it may repeat.
`o/O` opens the `Fields:` editor, where literal spaces have the same meaning.
Codes `D/G/J/M/X/I` select Drop/Gmean/Jttr/Javg/Jmax/Jint. The default columns stay
unchanged. `--mtr-columns` remains unavailable with RAW or JSON.

Jitter compares successive successful RTTs per responder row, across timeouts.
The first jitter is zero and participates in Javg. Jint is the mtr-scale
accumulator (`I = 15/16 I + jitter`), not the divided RFC estimate. Gmean includes
successful zero RTTs, which make it zero. Drop is `snt - received`.
