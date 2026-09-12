package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/trace"
)

// advance checks cancellation between records, so this accepts one pending
// record and then cancels without depending on a scheduler timing window.
type replayCancelAfterRecord struct {
	context.Context
	checks int
}

func (c *replayCancelAfterRecord) Err() error {
	c.checks++
	if c.checks > 1 {
		return context.Canceled
	}
	return nil
}

type replayExitFrameWriter struct {
	bytes.Buffer
	afterFrame func() error
}

func (w *replayExitFrameWriter) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data)
	if w.afterFrame != nil && bytes.Contains(data, []byte("\x1b[H\x1b[2J")) {
		callback := w.afterFrame
		w.afterFrame = nil
		return n, callback()
	}
	return n, err
}

func TestMTRReplayExitSummaryCommitsCancelledAdvance(t *testing.T) {
	for _, invalidRecord := range []bool{false, true} {
		name := "cancelled_after_probe"
		if invalidRecord {
			name = "failed_record_keeps_published_position"
		}
		t.Run(name, func(t *testing.T) {
			r, err := mtrsession.OpenReader(replayFixture(t, time.Now(), true))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.Close() }()
			current, err := readMTRReplay(t.Context(), r, 0)
			if err != nil {
				t.Fatal(err)
			}
			header, err := mtrReplayHeader(current.session, mtrReplayOptions{})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var advanceErr error
			output := &replayExitFrameWriter{}
			output.afterFrame = func() error {
				// The first frame already published cursor zero. Simulate an
				// advance that stops before a second frame can be published.
				if invalidRecord {
					current.pending = &mtrsession.Record{ElapsedNS: int64(time.Second),
						MTRSessionEvent: trace.MTRSessionEvent{Type: trace.MTRSessionMetadataEvent,
							Metadata: &trace.MTRSessionMetadata{IP: "192.0.2.99", Host: "unseen.invalid"}}}
				}
				advanceErr = current.advance(&replayCancelAfterRecord{Context: ctx}, r, 7*time.Second)
				cancel()
				if invalidRecord {
					return advanceErr
				}
				return nil
			}
			runErr := runMTRReplayTUI(ctx, r, current, header, 7*time.Second, true, false, output)
			if invalidRecord {
				if advanceErr == nil || errors.Is(advanceErr, context.Canceled) || !errors.Is(runErr, advanceErr) {
					t.Fatalf("advance=%v run=%v, want the original invalid-record error", advanceErr, runErr)
				}
			} else if !errors.Is(advanceErr, context.Canceled) || runErr != nil {
				t.Fatalf("advance=%v run=%v, want cancellation without a new error", advanceErr, runErr)
			}
			if current.cursor != time.Second {
				t.Fatalf("advance did not reach the intermediate cursor: %s", current.cursor)
			}
			_, summary, found := strings.Cut(output.String(), "NextTrace MTR snapshot\n")
			if !found {
				t.Fatalf("exit summary missing: %q", output.String())
			}
			wantCursor := "Replay: 00:00:01.000 / 00:00:07.000"
			if invalidRecord {
				wantCursor = "Replay: 00:00:00.000 / 00:00:07.000"
			}
			if !strings.Contains(summary, wantCursor) {
				t.Fatalf("wrong exit position; want %q:\n%s", wantCursor, summary)
			}
			if invalidRecord != strings.Contains(summary, "No probe statistics.") {
				t.Fatalf("statistics do not match the exit position:\n%s", summary)
			}
			if !invalidRecord && !strings.Contains(summary, "before.invalid") {
				t.Fatalf("accepted probe missing from exit summary:\n%s", summary)
			}
		})
	}
}
