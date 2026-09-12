package printer

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestMTRDialogsFitNarrowAndShortTerminals(t *testing.T) {
	for _, width := range []int{1, 8, 24, 40, 80, 120} {
		for _, height := range []int{2, 5, 10, 24} {
			for _, save := range []bool{false, true} {
				header := MTRTUIHeader{}
				if save {
					header.SaveDialog = MTRSaveDialog{Active: true, Format: "json", Path: strings.Repeat("中文目录/", 200) + "报告.json", Error: "write failed\x1b[31m", CapturedAt: time.Now()}
				} else {
					header.HelpDialog = MTRHelpDialog{Active: true, Offset: 1}
					header.Replay = &MTRReplayStatus{}
				}
				var frame strings.Builder
				mtrTUIRenderWithSize(&frame, header, nil, width, height)
				text := strings.TrimPrefix(frame.String(), "\x1b[H\x1b[2J")
				if !utf8.ValidString(text) || strings.Contains(text, "\x1b") {
					t.Fatalf("invalid or unsafe dialog: %q", text)
				}
				lines := strings.Split(strings.TrimSuffix(text, "\r\n"), "\r\n")
				if len(lines) > height-1 {
					t.Fatalf("height %d: %d lines", height, len(lines))
				}
				for _, line := range lines {
					if displayWidth(line) > width {
						t.Fatalf("width %d: %q", width, line)
					}
				}
			}
		}
	}
}

func TestMTRHelpContainsModeSpecificKeysAndScrolls(t *testing.T) {
	for _, replay := range []bool{false, true} {
		all := strings.Join(mtrHelpLines(replay, 120), "\n")
		for _, key := range []string{"q/Q", "Ctrl-C", "s/S", "?", "p/P", "Space", "r/R", "y/Y", "n/N", "e/E", "d/D", "g/G", "o/O", "Tab", "Enter", "Esc", "Backspace", "Ctrl-U"} {
			if !strings.Contains(all, key) {
				t.Fatalf("missing %s", key)
			}
		}
		if strings.Contains(all, "j/J") != replay || strings.Contains(all, "pause probing") == replay {
			t.Fatal("incorrect help mode", all)
		}
		var start, end strings.Builder
		renderMTRHelpDialog(&start, MTRHelpDialog{Active: true}, replay, 40, 8)
		renderMTRHelpDialog(&end, MTRHelpDialog{Active: true, Offset: MTRHelpMaxOffset(replay, 40, 8)}, replay, 40, 8)
		if start.String() == end.String() || !strings.Contains(end.String(), "Ctrl-C quits the session") {
			t.Fatal("cannot scroll to final help entry", end.String())
		}
	}
}

func TestMTRHelpHintSurvivesNarrowTerminals(t *testing.T) {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, width := range []int{1, 8, 24, 40, 80, 120, 200} {
		for _, replay := range []bool{false, true} {
			header := MTRTUIHeader{}
			if replay {
				header.Replay = &MTRReplayStatus{}
			}
			line := ansi.ReplaceAllString(buildMTRTUIControlsLine(header, width), "")
			if !strings.Contains(line, "?") || displayWidth(line) > width {
				t.Fatalf("width %d replay %v: %q", width, replay, line)
			}
		}
	}
}

func TestMTRSaveErrorCauseRemainsVisibleAt40Columns(t *testing.T) {
	for _, cause := range []string{"file already exists", "permission denied"} {
		var frame strings.Builder
		renderMTRSaveDialog(&frame, MTRSaveDialog{
			Active: true, Format: "txt", Path: strings.Repeat("long-directory/", 10) + "report.txt",
			Error: "save report: " + cause,
		}, 40, 10)
		if !strings.Contains(frame.String(), "save report: "+cause+"\r\n") {
			t.Fatalf("error cause was truncated: %q", frame.String())
		}
	}
}
