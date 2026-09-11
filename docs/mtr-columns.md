# MTR column metrics

Full, tiny and ntr support the same columns in live text output, reports and
offline replay. The default remains `LSNABWV` (Loss%, Snt, Last, Avg, Best, Wrst,
StDev). Column selection does not enable MTR or apply to RAW/JSON.

| Code | CLI name | Heading | JSON statistic | Definition |
| --- | --- | --- | --- | --- |
| D | dropped | Drop | dropped | Completed probes without a reply: snt minus received |
| G | gmean | Gmean | gmean_ms | Geometric mean of successful RTTs |
| J | jitter | Jttr | jitter_ms | Absolute difference of successive successful RTTs |
| M | javg | Javg | jitter_avg_ms | Mean jitter, including the first zero sample |
| X | jmax | Jmax | jitter_max_ms | Maximum jitter |
| I | jint | Jint | jitter_interarrival_ms | mtr-scale accumulator: I = 15/16 I + J |
| space | space | extra space | — | One additional display space |

RTT and jitter units are milliseconds. These definitions follow
[mtr's statistics](https://github.com/traviscross/mtr/blob/v0.94/ui/net.c), while
retaining NextTrace floating-point precision rather than mtr's integer microsecond
rounding. Jint deliberately uses mtr's accumulator scale, approximately 16 times
the divided RFC interarrival estimate. It is not measured packet-arrival spacing.

Statistics retain NextTrace's existing responder-per-row accounting. Successful
samples are consumed in aggregator/event order, independently for each TTL and
responder. Timeouts neither become zero RTTs nor break the successful RTT chain.
The first successful sample has zero jitter. A successful zero RTT participates
normally and makes the geometric mean zero. Rows without successful replies have
zero RTT-derived statistics in JSON; waiting text rows keep metric cells blank.

For successful RTTs 10, 20 and 40 ms, Gmean is 20, Jttr 20, Javg 10, Jmax 20 and
Jint 29.375 ms. A timeout between these replies changes Drop, not those metrics.
The mean is evaluated in log space, without retaining the full sample history.

Reset clears these statistics. Metadata updates do not add samples. The retained
`MigrateStats` API joins the destination successful sequence followed by the
source sequence, including their boundary jitter. Its count cap preserves derived
statistics by rescaling cumulative means; it does not reconstruct a truncated
sample prefix. Live destination handling clears old rows rather than migrating
them.

## Selection and spacing

```sh
nexttrace -w --mtr-columns loss,snt,space,dropped,gmean,jitter,javg,jmax,jint example.com
nexttrace --mtr-replay session.jsonl --mtr-columns loss,space,space,jitter,jint
```

CSV names ignore case and surrounding whitespace. Use explicit `space` entries
for spacing; an empty CSV entry remains invalid. Spaces may repeat, including at
the beginning and end, but at least one metric is required. Metrics cannot repeat.

In the `o/O` Fields editor, literal spaces insert the same extra spacing. The page
lists each code with its description. Enter applies, Esc cancels, Backspace deletes,
Ctrl-U clears, and Ctrl-C exits. Bracketed paste maps newlines and tabs to spaces
without submitting. Editing does not pause or resume probing. The selection
survives reopening the editor but is not persisted as a user preference.

## Structured output and recordings

All six numeric fields are included in CLI JSON reports, replay JSON and
`nexttrace_mtr_report` service/MCP statistics, even when zero. JSON schema version
remains 1; consumers must tolerate additive fields. MCP output schema describes
the fields and units. RAW, NDJSON and WebSocket probe events remain unchanged;
they do not gain aggregate snapshots.

Existing recordings retain raw RTTs and can be replayed with the new metrics.
Recording schema and event order are unchanged. See [session compatibility](mtr-session.md#extended-column-statistics)
for replaying a recording with new column names in an older binary.
