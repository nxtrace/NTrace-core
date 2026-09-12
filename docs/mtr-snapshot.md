# MTR snapshot reports v1

All three builds can save a report from a live or replay MTR TUI. Press `s/S`
to open the save dialog. Tab switches between path and format; Left/Right or
Space changes the selected format. Enter saves and Esc cancels.

## Saved reports

| Format | Content |
| --- | --- |
| `txt` | Plain-text session information and cumulative statistics using the current columns and Host/MPLS display |
| `json` | One `mtr_snapshot` document with session, state, and statistics |
| `html` | Standalone report using the current columns and Host/MPLS display, viewable without a running NextTrace service |

Opening the save dialog freezes the current cumulative statistics, including
the reset generation, current display settings and available metadata. Replay reports
describe the current playback position. Reports do not contain historical probe
events and cannot be opened by `--mtr-replay`; use `--mtr-record` to retain a
session's events.

Saving creates a new file and never overwrites an existing file. It does not
pause probing/playback, reset counters, or change the current display. A save
error is displayed in the dialog, leaving the session running.

## JSON contract

The file contains one UTF-8 JSON object followed by a newline. Consumers should
check both `type` and `schema_version` and tolerate additional v1 fields.
This contract is separate from [live MTR JSON](mtr-json.md) and the
[offline replay document](mtr-session.md#offline-json-fields).

| Field | Meaning |
| --- | --- |
| `schema_version` | `1` |
| `type` | `"mtr_snapshot"` |
| `session` | Session version, target, resolved IP, protocol, start time, effective parameters, source information and current `display` settings |
| `source` | `"live"` or `"replay"` |
| `captured_at` | Time the snapshot was captured |
| `elapsed_ns` | Elapsed session position, in nanoseconds |
| `generation` | Reset generation for the captured statistics |
| `state` | `"running"` or `"paused"`; live exit summaries use `"completed"`, `"interrupted"`, or `"error"` |
| `path_end` | Current path conclusion, or `null` when no conclusion is available |
| `stats` | Current cumulative `MTRHopStat` rows; an empty result is `[]` |

`session.display` describes the captured display rather than only the settings
at session startup. Column names use the existing `--mtr-columns` CSV names,
including repeated `space` entries. Column selection controls report
presentation; JSON statistics retain all metric fields.

Replay snapshots additionally include a `replay` object:

| Field | Meaning |
| --- | --- |
| `cursor_ns` | Current playback position in nanoseconds |
| `duration_ns` | Available recording duration in nanoseconds |
| `recording_complete` | Whether the recording has a valid end record |
| `recorded_paused` | Recorded probe-pause state at the current position |

`recorded_paused` is independent of whether playback is paused. The captured
statistics describe accepted probe results through the current position;
in-flight probes are not added as losses. Reset generations, responder ordering,
missing metadata and zero RTTs retain the existing aggregation rules. Metric
units and definitions match [MTR JSON statistics](mtr-json.md#final-report).
`path_end` describes reachability separately from session completion.

## Exit summary

Live and replay TUI sessions restore the terminal and print a plain-text
summary on exit by default. The replay summary uses the current position.

```sh
nexttrace --mtr --no-mtr-summary example.com
nexttrace --mtr-replay session.jsonl --no-mtr-summary
```

`--no-mtr-summary` suppresses this summary and does not select MTR. Traditional
traceroute, MTR report, RAW, JSON, and non-TTY output keep their own formats.
Standalone modes retain their own accepted options.

## Complete help

Press `?` in a live or replay TUI to open complete help. Up/Down scroll the
help, `?` or Esc closes it, and `q` or Ctrl-C exits. Opening or scrolling help
does not pause or reset the session.
