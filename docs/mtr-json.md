# MTR JSON v1

`nexttrace` and `nexttrace-tiny` select this contract with `--mtr --json`,
`-r --json`, or `-w --json`. `ntr --json` selects it by default.
Bare full/tiny `--json` retains the traditional traceroute contract.

## Modes

| Options | Output | Default count per hop |
| --- | --- | --- |
| `--mtr --json` | NDJSON stream | Unlimited |
| `--mtr --json -q 10` | NDJSON stream | 10 |
| `-r --json` or `-w --json` | One final JSON document | 10 |

`-q` never changes the output mode. Nonpositive counts mean unlimited for
streams and are invalid for JSON reports. The existing non-TCP count cap of
255 applies. The default per-hop interval is 1000ms.

Both modes use FULL metadata and ignore `-y`, `--show-ips`, and presentation
settings. FULL means all available information, not guaranteed Geo/PTR data.
Provider selection, language, `--no-rdns`, `--always-rdns`, `--dot-server`, and
`--disable-mpls` still apply. JSON reports use wide collection rules even with
`-r`; `--raw` cannot be combined with MTR `--json`.

## NDJSON events

Each UTF-8 line is a complete JSON object written immediately, without ANSI,
banners, or diagnostic lines. Every event has `schema_version: 1`, `type`, and
`seq`, starting at 1 and incrementing by 1. Consumers should tolerate additional
fields within schema version 1.

| Type | Fields |
| --- | --- |
| `start` | `version`, `target`, `resolved_ip`, `protocol`, `started_at`, `effective_parameters` |
| `probe` | `record`: the existing `MTRRawRecord` |
| `path_end` | `path_end`: path conclusion or `null` when the path reopens |
| `end` | `ended_at`, `duration_ms`, `end_reason`, `path_end`, optional `error` and `signal` |

`start` occurs exactly once and first. `end` occurs exactly once and last on
completion, handled interruption, or session failure, provided stdout remains
writable. Initialization failure still produces `start` and `end`; undetermined
session fields are `null`. Write/encoding failures cancel probing and stop
output, with no retry of the final event/report.

Each RAW callback maps to one `probe` in the same order. `iteration` keeps its
RAW meaning and is not a unique event identifier. A probe precedes the path
change it triggers; final max-hops conclusions precede `end`. When recording
failure prevents a RAW probe callback, its pending destination/unreachable or
reopening change is discarded from stdout and the final stream path state. A
max-hops conclusion can still be emitted after the last callback. There are no separate Geo/PTR
update events, no retained event history, and no final stream statistics.

Example finite stream with metadata queries disabled (version/times illustrative):

```jsonl
{"schema_version":1,"type":"start","seq":1,"version":"v0.0.0.alpha","target":"127.0.0.1","resolved_ip":"127.0.0.1","protocol":"icmp","started_at":"2026-09-07T00:00:00Z","effective_parameters":{"max_per_hop":1,"hop_interval_ms":1000,"timeout_ms":1000,"begin_hop":1,"max_hops":1,"parallel_requests":18,"source_address":"127.0.0.1","packet_size":28,"random_packet_size":false,"tos":0,"data_provider":"disable-geoip","language":"cn","rdns":false,"always_wait_rdns":false,"disable_mpls":false,"dn42":false}}
{"schema_version":1,"type":"probe","seq":2,"record":{"iteration":1,"ttl":1,"success":true,"ip":"127.0.0.1","rtt_ms":0,"lat":0,"lng":0}}
{"schema_version":1,"type":"path_end","seq":3,"path_end":{"hop":1,"reason":"destination_reached"}}
{"schema_version":1,"type":"end","seq":4,"ended_at":"2026-09-07T00:00:00.2Z","duration_ms":200,"end_reason":"completed","path_end":{"hop":1,"reason":"destination_reached"}}
```

`record` preserves the [MTRRawRecord fields](../trace/mtr_raw.go):
`iteration`, `ttl`, `success`, `ip`, `host`, `rtt_ms`, `asn`, `country`, `prov`,
`city`, `district`, `owner`, `lat`, `lng`, `mpls`, and `response`.
Optional metadata fields can be absent. A successful zero RTT is valid;
`success` distinguishes replies from timeouts.

## Final report

Stdout contains exactly one JSON object, followed by a newline. The first JSON
decode succeeds and the second returns EOF. Fields:

- `schema_version`, `version`, `target`, `resolved_ip`, `protocol`.
- `started_at`, `ended_at`: UTC RFC3339Nano timestamps; `duration_ms` includes
  target resolution, initialization, probing, and cleanup.
- `effective_parameters`: normalized configuration, or `null` when preparation
  did not complete.
- `stats`: the runner's final `MTRHopStat` snapshot; empty is `[]`.
- `end_reason`, `path_end`, optional `error` and `signal`.

Example validation failure:

```json
{"schema_version":1,"version":"v0.0.0.alpha","target":"127.0.0.1","resolved_ip":null,"protocol":"icmp","started_at":"2026-09-07T00:00:00Z","effective_parameters":null,"ended_at":"2026-09-07T00:00:00.001Z","duration_ms":1,"end_reason":"error","path_end":null,"error":{"stage":"validation","message":"MTR JSON report requires --queries greater than zero"},"stats":[]}
```

Statistics are shared with text wide reports, without reaggregation or row
collapsing. Each [MTRHopStat](../trace/mtr_stats.go) has `ttl`, optional `host`
and `ip`, `loss_percent`, `snt`, `last_ms`, `avg_ms`, `best_ms`, `wrst_ms`,
`stdev_ms`, `received`, `dropped`, `gmean_ms`, `jitter_ms`, `jitter_avg_ms`,
`jitter_max_ms`, `jitter_interarrival_ms`, and optional `geo`, `mpls`, `response`.
The additional statistics are always present, including zero values, within
schema version 1. See [column metrics](mtr-columns.md) for definitions and the
mtr-scale interarrival accumulator (not the divided RFC estimate). Multiple responders
and unknown rows retain the aggregator's existing identity and ordering rules.
`snt` counts completed events already entered into the aggregator. Interruptions
do not synthesize timeout events for in-flight probes. RTT units are milliseconds.

## Effective parameters

| Fields | Meaning |
| --- | --- |
| `max_per_hop`, `hop_interval_ms`, `timeout_ms` | Normalized count (0 unlimited), interval and timeout |
| `begin_hop`, `max_hops`, `parallel_requests` | TTL range and concurrency |
| `source_address`, optional `source_device` | Selected source address and applicable device binding |
| `source_port`, `port` | TCP/UDP only; source `0` selects one automatic port, `-1` selects a random port per probe, `1..65535` selects a fixed port; destination `1..65535` |
| `packet_size`, `random_packet_size` | Total IP+protocol+payload size; negative size means randomized up to its absolute value |
| `tos`, optional `icmp_mode` | Traffic class; ICMP listener setting where supported |
| `data_provider`, `language`, optional `dot_server` | Effective metadata provider, language and DNS override |
| `rdns`, `always_wait_rdns`, `disable_mpls`, `dn42` | Query/metadata configuration |

Ignored options such as `-y`, `-z`, and display width are not recorded as effective
parameters. Source selection describes configured behavior, not proof that the
OS routed packets through a particular interface.

## Completion and errors

| Outcome | `end_reason` | Exit code |
| --- | --- | --- |
| Completed, including no replies | `completed` | 0 |
| DNS, permission, initialization, unrecoverable execution or output failure | `error` | 1 |
| Session validation failure | `error` | 2 |
| SIGINT / SIGTERM | `interrupted` | 130 / 143 |

`completed` is not a reachability verdict. `path_end` independently describes
`destination_reached`, `unreachable`, or `max_hops`, with `hop` and optional `responses`
and `markers`. It is `null` without a current conclusion. A stream may emit
`path_end: null` after an earlier unreachable conclusion and later emit a new
conclusion.

Session errors preserve partial statistics and have `error.stage` and
`error.message`. Probe-resource initialization and replacement failures use
`stage: "initialize"`, including TOS/Traffic Class setup failures in TCP/UDP
per-hop fallback probes. These failures terminate probing without adding a
timeout or loss sample. Other runner failures use `stage: "probe"`.
Signals use `signal: "SIGINT"` or `"SIGTERM"`. Internal
cancellation retains its error cause and is not reported as user interruption.
Recording failures use `record` when no earlier session error exists. A later
recording start, sync or close failure does not replace the original error or
its stage. A recording failure may leave only a recoverable file prefix; a
writable stdout still follows the JSON error lifecycle.
Syntax and conflicting-mode errors occur before a session: stdout is empty,
stderr carries the diagnostic, and exit code is 2. Help/version retain their
existing output behavior. All session diagnostics go to stderr.

### Linux socket mark

For supported Linux traceroute/MTR CLI sessions, an explicit `--fwmark` is recorded in MTR `effective_parameters.fwmark` as an optional uint32 JSON number. Decimal and hexadecimal CLI spellings produce the same value. Explicit zero is included; omission leaves the field absent. Schema version remains 1. This records the requested socket setting, not proof of the observed egress route. Mark/source initialization errors use the existing `initialize` error stage and nonzero exit status; no human-readable lines are inserted into JSON or NDJSON.

## Combined examples

```sh
# Random TCP source ports, finite NDJSON and an independent replay file
nexttrace --mtr --json --tcp --source-port -1 -q 10 --mtr-record tcp-session.jsonl example.com
# Linux: marked wide JSON report with DSCP 46 / ECN 0
nexttrace -w --json --fwmark 0x100 --tos 184 -q 10 example.com
```

The TCP/UDP source-port value is preserved in effective parameters; it is not a
list of the randomly selected ports. `--mtr-columns` is rejected with JSON,
including offline JSON replay. CLI JSON, session recordings, offline replay
JSON and MCP structured output are distinct contracts.
