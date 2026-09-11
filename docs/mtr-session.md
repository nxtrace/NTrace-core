# MTR session recording v1

`--mtr-record FILE` explicitly records the current CLI MTR session in all three
builds. TUI, non-TTY tables, report/wide, RAW and MTR JSON can be recorded. The
file is independent of stdout and of the existing [MTR JSON v1](mtr-json.md)
contract. Existing NDJSON streams and final JSON reports are not replay files.

## Recording

```sh
nexttrace --mtr --mtr-record session.jsonl example.com
nexttrace -w -q 10 --mtr-record report-session.jsonl example.com
nexttrace --mtr --json -q 10 --mtr-record stream-session.jsonl example.com
ntr --mtr-record session.jsonl example.com
```

Full and tiny require an explicit MTR mode; ntr defaults to MTR. Traditional
traceroute, standalone modes and replay cannot be combined with recording.
The file is exclusively created with Unix mode `0600`: existing files and
symlinks are never overwritten or appended to. The parent directory must exist.
Opening the recording precedes network initialization and probing.

Each accepted event is written as a complete UTF-8 JSON line. A failed encode or
write cancels probing. The file is synchronized and closed at the end, and
sync/close errors are reported as failures. The first session error and its
stage take precedence over later recording start or finish errors. When file
writes already failed, no complete error end record is guaranteed. No events
are silently dropped.
There is no rotation, compression, append mode, or periodic durable checkpoint.
Crash recovery is limited to complete records that remain readable from disk.

## Event contract

Records identify `format: "nexttrace-mtr-session"` and `schema_version: 1`.
The maximum encoded record size is 1 MiB including its terminating newline.
Every record includes `type`, `seq`, `generation`, `elapsed_ns`, and `timestamp`.

- `seq` starts at 1 and increases by exactly one, including control events.
- `generation` starts at 0; each reset increases it by one.
- `elapsed_ns` is the nondecreasing playback clock from session start. Events
  with equal elapsed time retain sequence order.
- `timestamp` records event application time. Probe completion time is separate:
  concurrent results must never be reordered by their completion timestamps.
- Probe records include optional `probe_age_ns`, computed from the monotonic clock before serialization; replay uses this nonnegative completion-to-application age for history positioning, falling back to the wall timestamps only for older records without this field.
- `start` is first and `end` is last in a complete recording. Additional fields
  may be introduced within v1; unsupported versions and event types are errors.

| Type | Payload |
| --- | --- |
| `start` | `session`: software version, target, resolved IP, protocol, start time, effective parameters, source display information and initial display settings |
| `probe` | `probe`: counted TTL, IP/Host, success, integer `rtt_ns`, `completed_at`, complete available Geo/MPLS and response details; `iteration` retains scheduler meaning |
| `metadata` | `metadata`: IP and applied Host/Geo updates, without counting another probe |
| `pause`, `resume` | Changes actually observed by the scheduler; pending probe results may still follow pause |
| `reset` | New generation; clears current statistics, path evidence and history |
| `path_end` | `path_end`: destination/unreachable/max-hops conclusion, or explicit `null` for reopening |
| `end` | `end`: end time, reason, final path conclusion and optional error/signal |

The end time must equal its record timestamp, and the final path must match the
reconstructed state. `completed` has no error or signal; `interrupted` has no
error and optionally `SIGINT` or `SIGTERM`; `error` requires a nonempty error
stage/message and no signal. Metadata events must actually update an existing
responder. Probe TTLs stay within the configured range, and bounded sessions
may not exceed `max_per_hop` in any generation. A `max_hops` event requires
all configured TTLs to have completed that count. Reset clears these counts.
Contradictory or unapplied events are rejected.

When present, a probe's `response.kind` must be `transit`, `destination`, or
`unreachable`; empty or unknown kinds are invalid records. A response also
requires `success: true` and a valid responder IP; timeouts cannot carry path
evidence.

Replay verifies destination, unreachable and reopening events against the
accumulated probe evidence before applying a path boundary. A contradictory
event is an error, not an instruction to discard statistics.

Offline JSON always encodes `stats` as an array, including `[]` for empty
sessions, initialization failures and the state immediately after a reset.

The session header only includes explicitly selected safe fields, not the
runtime configuration or provider credentials. Source selection describes the
recorded configuration/display, not proof of an operating-system route.
Initialization failure can produce a complete empty session with undetermined
parameters and an error end record.

### Statistics and ordering

Only probes actually accepted into the existing aggregator are recorded.
The probe payload contains the final Hop used for aggregation, including
synchronous metadata. Later metadata patches update existing rows without
changing Snt, RTT statistics or responder order.

Response identity, integer RTT precision, unknown-row merging and synthetic
timeouts retain the live aggregator's semantics. Individual local send retries,
unsent time during pause and in-flight probes discarded by cancellation are
not converted into additional timeout records.

The triggering probe precedes its `path_end` event. A confirmed destination
clears statistics above its TTL. A provisional unreachable edge hides those
rows; reopening can reveal their earlier statistics. Reset discards late results
from the previous generation. It does not remove earlier events from the file.

## Offline replay

```sh
nexttrace --mtr-replay session.jsonl
nexttrace --mtr-replay session.jsonl -w
nexttrace --mtr-replay session.jsonl -r --json
```

`--mtr-replay --help` and `--mtr-replay --version` need no file. For a file named
`--help` or `--version`, use `--mtr-replay=--help` or a path such as `./--help`.

Replay accepts presentation options, not a target or probe configuration.

| Option | Behavior |
| --- | --- |
| `-r`, `--report` | Print the final compact text report |
| `-w`, `--wide` | Print the final text report without host truncation |
| `-j`, `--json` | Print the offline JSON document, taking precedence over report/wide |
| `--mtr-columns` | Select text columns; incompatible with JSON |
| `-y`, `--ipinfo` | Initial TUI host mode `0..4` |
| `--show-ips` | Display PTR and IP together |
| `-g`, `--language` | Display `cn` or `en` using recorded metadata |
| `-C`, `--no-color` | Disable TUI colors; text reports already have no ANSI |

Omitted host mode, PTR/IP display, columns and language inherit the recording's
initial settings; MPLS display also starts from the recorded setting. Live column
edits are session-local and are not recorded as replay control events. Explicit
`-r` chooses compact output; without `-r/-w`, non-TTY output inherits the
recorded wide setting. Both source and responder host names are truncated in
compact output. No target, `--raw`, probe options, `--mtr-record`, stdin (`-`),
pipe or non-regular input file is accepted. It
does not initialize probing, resolve addresses, query metadata, or discover a
source interface. Recorded missing metadata stays missing. It can run without
raw-socket privileges and without network connectivity.

TTY opens paused at the last valid position. Space plays at original speed;
at EOF it starts from the beginning. `p` pauses playback, `r` rewinds and pauses,
`q` quits. Recorded probe pause and current playback pause are separate states.
Host, MPLS, columns and the existing three history charts remain available.

`j/J` opens elapsed-time input, prefilled with the current position, and pauses.
Use `HH:MM:SS[.mmm]`, with hours allowed beyond 23. Enter seeks and stays paused;
Esc cancels and stays paused; Backspace deletes and Ctrl-U clears. Invalid or
out-of-range input leaves the current position unchanged. Bracketed-paste
newlines cannot submit the input.

The history chart shows the three minutes ending at the playback position.
The whole file remains available for seeking. Initial loading and seeking scan
records sequentially, using the existing aggregator; no full event array or
checkpoint index is retained. Seeking is O(N) in the events before the requested
position and can be canceled. Memory still depends on distinct responder state
and probes inside the current history window. This version does not tail a file
that continues growing after it is opened.

Non-TTY or explicit `-r/-w` produces one final report. `--json` produces a
separate offline document with session information, statistics, path state,
playback position and recording completeness, rather than emitting live NDJSON.

### Offline JSON fields

The output is one object plus a newline, with `schema_version: 1` and
`type: "mtr_replay"`. It contains:

| Field | Meaning |
| --- | --- |
| `session` | Original session header and initial display/effective parameters |
| `complete` | Whether a valid end record was read |
| `cursor_ns`, `duration_ns` | Final valid playback position and duration, in nanoseconds |
| `generation`, `recorded_paused` | Final reducer generation and recorded probe-pause state |
| `path_end` | Reconstructed conclusion, or `null` |
| `stats` | Final MTR statistics; always an array, including `[]` |
| `end` | Original end payload, omitted when absent |

Replay text/TUI and diagnostics replace Unicode control (`Cc`) and format
(`Cf`, including bidi overrides and isolates) characters with spaces. Only
display copies are sanitized; recorded data and offline JSON retain original
values. JSON consumers must apply their own display escaping.

## Incomplete and invalid files

A missing end record or an unfinished final JSON line recovers only the complete
valid prefix. Text/TUI explicitly identify the recording as incomplete; offline
JSON exposes the same state. Incomplete replay exits nonzero, even when some
statistics could be recovered. An unsupported version, invalid middle record,
oversized record or invalid sequence is an error; records are never skipped to
fabricate a complete session.

Normal replay of a complete file exits 0. A complete recording may describe an
original session that ended with an error or signal; that historical outcome is
retained separately from the current file-read result. Invalid arguments exit
2; file/recording errors exit 1; handled SIGINT/SIGTERM exit 130/143. Diagnostics
go to stderr. A completed probe session is not itself a reachability verdict.

Recordings contain target, source and responder addresses and available
metadata. Redact those fields before attaching files to a public issue when
they identify private infrastructure.

## Extended column statistics

Replay recomputes dropped packets, geometric mean and jitter from the recorded
successful RTTs in event sequence order, not completion-timestamp order. Existing
recordings need no conversion. See [column metrics](mtr-columns.md).

The recording schema stays unchanged. A new recording may name additional
columns (including `space`) in `display.columns`. Older binaries need an explicit
supported `--mtr-columns` selection for text replay of such a recording; JSON
replay does not parse display columns. Interactive column edits remain local to
the display and do not rewrite the recording's initial column selection.
