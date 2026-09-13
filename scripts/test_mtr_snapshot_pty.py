"""Exercise MTR snapshot saving, help and exit summaries on PTY/ConPTY.

Usage: python3 scripts/test_mtr_snapshot_pty.py /path/to/nexttrace
Accepts full, tiny and ntr builds. Only live loopback probes are sent; replay
uses the recording created by this run. Windows needs pywinpty==3.0.5.
Linux CI runs under sudo for ICMP socket access.
"""

import json
import os
import platform
import re
import struct
import subprocess
import sys
import tempfile
import time

from test_mtr_session_pty import Terminal, replay_time

if os.name != "nt":
    import fcntl
    import termios


SUMMARY = "NextTrace MTR snapshot"


def resize(terminal, width, height):
    if terminal.windows:
        terminal.process.setwinsize(height, width)
    else:
        fcntl.ioctl(terminal.slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", height, width, 0, 0))


def recording_probes(path):
    with open(path, "rb") as source:
        lines = source.readlines()
    # The writer publishes complete lines; a concurrent read may see its tail.
    records = [json.loads(line.decode("utf-8"))
               for line in lines if line.endswith(b"\n")]
    return sum(record["type"] == "probe" for record in records)


def check_recording_tail(directory):
    path = os.path.join(directory, "reader-tail.jsonl")
    prefix = b'{"type":"probe"}\n'
    tail = (json.dumps({"type": "probe", "host": "中文😀"},
                       ensure_ascii=False) + "\n").encode("utf-8")
    # Exercise every byte boundary, including cuts inside multibyte characters.
    for end in range(len(tail) + 1):
        with open(path, "wb") as output:
            output.write(prefix + tail[:end])
        assert recording_probes(path) == 1 + int(end == len(tail)), end
    with open(path, "wb") as output:
        output.write(prefix + b'{"type":"probe","host":"\xff"}\n')
    try:
        recording_probes(path)
    except UnicodeDecodeError:
        pass
    else:
        raise AssertionError("Invalid UTF-8 in a complete record was ignored")


def check_help(terminal, recording=None):
    resize(terminal, 100, 10)
    terminal.send("?")
    terminal.expect("Keyboard shortcuts")
    terminal.send("\x1b[B" * 8)
    terminal.expect("Up/Down (9/")
    terminal.send("\x1b[A" * 8)
    terminal.expect("Up/Down (1/")
    if recording:
        before = recording_probes(recording)
        terminal.read(0.6)
        assert recording_probes(recording) > before, "Help paused live probing"
    terminal.send("\x1b")
    terminal.expect("Rcv")
    terminal.send("?")
    terminal.expect("Keyboard shortcuts")
    terminal.send("?")
    terminal.expect("Rcv")
    resize(terminal, 150, 32)


def save_report(terminal, path, format_name, recording=None, exists=False):
    terminal.send("s")
    terminal.expect("Save report")
    before = None
    if recording:
        before = recording_probes(recording)
        terminal.read(0.6)
        assert recording_probes(recording) > before, "Save dialog paused live probing"
    terminal.send({"txt": "", "json": "\x1b[C", "html": "\x1b[D"}[format_name])
    terminal.send("\t\x15" + path + "\r")
    if exists:
        terminal.expect("save report:")
    else:
        terminal.expect("Saved:")
        assert os.path.isfile(path), f"Report was not created: {path}"
    terminal.send("\x1b")
    terminal.expect("Rcv")
    return before


def check_snapshot(path, source):
    with open(path, encoding="utf-8") as report:
        snapshot = json.load(report)
    assert snapshot["type"] == "mtr_snapshot", snapshot
    assert snapshot["schema_version"] == 1, snapshot
    assert snapshot["source"] == source, snapshot
    assert snapshot["session"]["display"]["columns"] == "avg,received", snapshot
    assert snapshot["state"] in ("running", "paused"), snapshot
    assert snapshot["captured_at"] and snapshot["elapsed_ns"] >= 0, snapshot
    assert snapshot["generation"] == 0, snapshot
    assert len(snapshot["stats"]) == 1, snapshot
    stat = snapshot["stats"][0]
    assert stat["ip"] == "127.0.0.1" and stat["snt"] > 0, snapshot
    # A selected two-column table still exports every JSON statistic.
    for metric in ("loss_percent", "snt", "received", "avg_ms", "last_ms",
                   "stdev_ms", "dropped", "jitter_interarrival_ms"):
        assert metric in stat, (metric, snapshot)
    return snapshot


def check_files(paths):
    with open(paths["txt"], encoding="utf-8") as source:
        text = source.read()
    assert text.startswith(SUMMARY + "\n"), text
    assert "Avg" in text and "Rcv" in text and "127.0.0.1" in text, text
    assert "Loss%" not in text, "Text report ignored current columns"
    assert "\x1b" not in text, "Text report contains terminal escapes"
    with open(paths["html"], encoding="utf-8") as source:
        html = source.read()
    assert html.lower().startswith("<!doctype html>"), html
    assert "Avg" in html and "Rcv" in html and "127.0.0.1" in html, html
    assert "<script" not in html.lower(), "Standalone report unexpectedly contains a script"
    assert "\x1b" not in html, "HTML report contains terminal escapes"


def quit_with_summary(terminal, enabled, expected_code=0):
    start = len(terminal.log)
    terminal.send("q")
    terminal.expect("\x1b[?1049l", "\x1b[?25h")
    if terminal.windows:
        deadline = time.monotonic() + 3
        while terminal.process.isalive() and time.monotonic() < deadline:
            terminal.read(0.1)
        assert not terminal.process.isalive(), "Process did not exit"
        assert terminal.process.exitstatus == expected_code, terminal.process.exitstatus
    else:
        terminal.process.wait(timeout=3)
        assert terminal.process.returncode == expected_code, terminal.process.returncode
        assert termios.tcgetattr(terminal.slave) == terminal.original, "Terminal attributes not restored"
    terminal.read(0.2)
    output = terminal.log[start:].decode(errors="replace")
    restored = output.index("\x1b[?1049l")
    assert SUMMARY not in output[:restored], "Summary printed before terminal restoration"
    tail = output[restored + len("\x1b[?1049l"):]
    assert tail.count(SUMMARY) == int(enabled), tail
    if enabled:
        assert "127.0.0.1" in tail and "\x1b" not in tail, tail


def exercise_session(terminal, directory, source, recording=None):
    terminal.expect("Rcv", "127.0.0.1")
    check_help(terminal, recording)
    terminal.send("o")
    terminal.expect("Fields:")
    terminal.send("\x15AR\r")
    terminal.expect("Avg", "Rcv")
    paths = {fmt: os.path.join(directory, f"诊断报告 {source}.{fmt}")
             for fmt in ("txt", "json", "html")}
    frozen_count = None
    for fmt, path in paths.items():
        count = save_report(terminal, path, fmt, recording if fmt == "json" else None)
        if count is not None:
            frozen_count = count
    snapshot = check_snapshot(paths["json"], source)
    check_files(paths)
    with open(paths["json"], "rb") as report:
        original = report.read()
    save_report(terminal, paths["json"], "json", exists=True)
    with open(paths["json"], "rb") as report:
        assert report.read() == original, "Existing report was overwritten"
    if recording:
        assert snapshot["stats"][0]["snt"] <= frozen_count, "Saved report changed while the dialog was open"
        terminal.send("p")
        terminal.expect("Paused")
        terminal.read(0.4)
        paused_path = os.path.join(directory, "暂停报告.json")
        save_report(terminal, paused_path, "json")
        paused = check_snapshot(paused_path, "live")
        assert paused["state"] == "paused", paused
    return snapshot


def read_replay_json(binary, path, expected_code):
    result = subprocess.run([binary, "--mtr-replay", path, "--json"],
                            capture_output=True, text=True, timeout=10)
    assert result.returncode == expected_code, (result.returncode, result.stderr)
    return json.loads(result.stdout)


def write_record_prefix(path, records):
    with open(path, "x", encoding="utf-8") as output:
        for record in records:
            output.write(json.dumps(record, ensure_ascii=False) + "\n")


def check_middle_position(binary, terminal, directory, records, final):
    at = final["duration_ns"] // 2 // 1000000 * 1000000
    assert 0 < at < final["duration_ns"], final
    prefix = [record for record in records
              if record["type"] != "end" and record["elapsed_ns"] <= at]
    prefix_path = os.path.join(directory, "middle-prefix.jsonl")
    write_record_prefix(prefix_path, prefix)
    expected = read_replay_json(binary, prefix_path, 1)
    assert expected["complete"] is False, expected
    # There is no --mtr-replay-at flag. The valid event prefix supplies an
    # independent CLI aggregation oracle; its final event can precede the
    # requested millisecond cursor, which is asserted separately below.
    assert expected["cursor_ns"] == prefix[-1]["elapsed_ns"] <= at, expected
    terminal.send("j")
    terminal.expect("Go to HH:MM:SS[.mmm]", "Time:")
    terminal.send("\x15" + replay_time(at) + "\r")
    terminal.expect("Paused " + replay_time(at) + "/")
    path = os.path.join(directory, "中间位置.json")
    save_report(terminal, path, "json")
    snapshot = check_snapshot(path, "replay")
    assert snapshot["stats"] == expected["stats"], (snapshot, expected)
    assert snapshot["stats"][0]["snt"] < final["stats"][0]["snt"], (snapshot, final)
    assert snapshot["path_end"] == expected["path_end"], (snapshot, expected)
    assert snapshot["elapsed_ns"] == at, snapshot
    assert snapshot["replay"] == {
        "cursor_ns": at, "duration_ns": final["duration_ns"],
        "recording_complete": True, "recorded_paused": expected["recorded_paused"],
    }, snapshot


def check_incomplete_recording(binary, directory, records, final, log):
    path = os.path.join(directory, "missing-end.jsonl")
    assert records[-1]["type"] == "end", records[-1]
    write_record_prefix(path, records[:-1])
    expected = read_replay_json(binary, path, 1)
    assert expected["complete"] is False, expected
    assert expected["stats"] == final["stats"], (expected, final)
    terminal = Terminal([binary, "--mtr-replay", path, "--mtr-columns", "avg,received", "--no-color"], log)
    try:
        terminal.expect("Rcv", "127.0.0.1", "Incomplete")
        report_path = os.path.join(directory, "残缺录制.json")
        save_report(terminal, report_path, "json")
        snapshot = check_snapshot(report_path, "replay")
        assert snapshot["stats"] == expected["stats"], (snapshot, expected)
        assert snapshot["elapsed_ns"] == expected["cursor_ns"], (snapshot, expected)
        assert snapshot["replay"] == {
            "cursor_ns": expected["cursor_ns"], "duration_ns": expected["duration_ns"],
            "recording_complete": False, "recorded_paused": expected["recorded_paused"],
        }, snapshot
        quit_with_summary(terminal, True, expected_code=1)
    finally:
        terminal.close()


def check_non_tty_output(binary, mode, probe_flags):
    # --mtr-columns belongs to text output, so omit that option for the shared
    # report/RAW/JSON setup. Captured pipes deliberately select non-TTY output.
    common = ["-q", "2"] + probe_flags[2:]
    for output_mode, flags in (("report", ["-r"]), ("raw", ["-r", "--raw"]),
                               ("json_report", ["-r", "--json"]),
                               ("json_stream", mode + ["--json"])):
        for suppress in (False, True):
            extra = ["--no-mtr-summary"] if suppress else []
            result = subprocess.run([binary] + flags + extra + common,
                                    capture_output=True, text=True, timeout=10)
            assert result.returncode == 0, (output_mode, result.returncode, result.stderr)
            assert SUMMARY not in result.stdout + result.stderr, (output_mode, result.stdout, result.stderr)
            assert "\x1b[?1049" not in result.stdout, (output_mode, result.stdout)
            if output_mode == "report":
                assert "127.0.0.1" in result.stdout and "Snt" in result.stdout, result.stdout
            elif output_mode == "raw":
                rows = [line for line in result.stdout.splitlines() if "|" in line]
                assert len(rows) == 2 and all(len(line.split("|")) == 12 for line in rows), result.stdout
            elif output_mode == "json_report":
                report = json.loads(result.stdout)
                assert report["stats"][0]["snt"] == 2, report
            else:
                events = [json.loads(line) for line in result.stdout.splitlines()]
                assert events[0]["type"] == "start" and events[-1]["type"] == "end", events
                assert sum(event["type"] == "probe" for event in events) == 2, events


def main(binary):
    binary = os.path.abspath(binary)
    log = bytearray()
    help_result = subprocess.run([binary, "--help"], capture_output=True,
                                 text=True, timeout=10, check=True)
    assert "--no-mtr-summary" in help_result.stdout, help_result.stdout
    mode = ["--mtr"] if re.search(r"^\s+-t\s+--mtr\s", help_result.stdout, re.M) else []
    probe_flags = ["--mtr-columns", "loss,snt,received,avg", "-d", "disable-geoip",
                   "-n", "-s", "127.0.0.1", "-m", "1", "-i", "60",
                   "--timeout", "100", "--no-color", "--language", "en", "127.0.0.1"]
    try:
        with tempfile.TemporaryDirectory(prefix="mtr-snapshot-pty-") as directory:
            check_recording_tail(directory)
            recording = os.path.join(directory, "session.jsonl")
            online = Terminal([binary] + mode + ["--mtr-record", recording] + probe_flags, log)
            try:
                exercise_session(online, directory, "live", recording)
                quit_with_summary(online, True)
            finally:
                online.close()

            with open(recording, encoding="utf-8") as source:
                records = [json.loads(line) for line in source]
            final = read_replay_json(binary, recording, 0)
            assert final["complete"] is True, final
            offline = Terminal([binary, "--mtr-replay", recording, "--no-color"], log)
            try:
                snapshot = exercise_session(offline, directory, "replay")
                assert snapshot["stats"] == final["stats"], (snapshot, final)
                assert snapshot["elapsed_ns"] == final["cursor_ns"], (snapshot, final)
                assert snapshot["replay"] == {
                    "cursor_ns": final["cursor_ns"], "duration_ns": final["duration_ns"],
                    "recording_complete": True, "recorded_paused": final["recorded_paused"],
                }, snapshot
                check_middle_position(binary, offline, directory, records, final)
                quit_with_summary(offline, True)
            finally:
                offline.close()

            for command in ([binary] + mode + ["--no-mtr-summary"] + probe_flags,
                            [binary, "--mtr-replay", recording, "--no-mtr-summary", "--no-color"]):
                terminal = Terminal(command, log)
                try:
                    terminal.expect("Rcv", "127.0.0.1")
                    quit_with_summary(terminal, False)
                finally:
                    terminal.close()
            check_incomplete_recording(binary, directory, records, final, log)
            check_non_tty_output(binary, mode, probe_flags)
        print(json.dumps({
            "platform": platform.system(), "binary": os.path.basename(binary), "snapshot_pty": "PASS",
            "checks": ["live/replay help", "help scrolling", "Esc and ? close help",
                       "live probes continue in dialogs", "current display columns", "UTF-8 paths",
                       "txt/json/html reports", "JSON full statistics", "exclusive file creation",
                       "frozen snapshot", "paused snapshot", "replay statistics/position parity",
                       "middle seek snapshot/event-prefix parity", "incomplete snapshot and nonzero exit",
                       "summary after terminal restore", "no-mtr-summary live/replay",
                       "non-TTY report/RAW/JSON contracts", "terminal restore"],
        }, indent=2))
    finally:
        with open(os.path.join(tempfile.gettempdir(), "mtr-snapshot-pty.log"), "wb") as output:
            output.write(log)


if __name__ == "__main__":
    main(sys.argv[1])
