"""Record loopback MTR and exercise offline replay on a real PTY.

Usage: python3 scripts/test_mtr_session_pty.py /path/to/nexttrace
The same command accepts full, tiny and ntr builds. Windows requires
pywinpty==3.0.5; Linux CI runs under sudo for ICMP socket access.
"""

import json
import math
import os
import platform
import re
import select
import signal
import struct
import subprocess
import sys
import tempfile
import time

if os.name != "nt":
    import fcntl
    import pty
    import termios


class Terminal:
    def __init__(self, command, log):
        self.log = log
        self.windows = os.name == "nt"
        if self.windows:
            from winpty import PtyProcess
            self.process = PtyProcess.spawn(command, dimensions=(32, 150))
        else:
            self.master, self.slave = pty.openpty()
            self.original = termios.tcgetattr(self.slave)
            fcntl.ioctl(self.slave, termios.TIOCSWINSZ,
                        struct.pack("HHHH", 32, 150, 0, 0))
            self.process = subprocess.Popen(
                command, stdin=self.slave, stdout=self.slave, stderr=self.slave)

    def read(self, seconds=0.2):
        end = time.monotonic() + seconds
        data = bytearray()
        while time.monotonic() < end:
            source = self.process.fileobj if self.windows else self.master
            if select.select([source], [], [], max(0, end - time.monotonic()))[0]:
                try:
                    chunk = (self.process.read(65536).encode() if self.windows
                             else os.read(self.master, 65536))
                    if not chunk:
                        break
                    data.extend(chunk)
                except (OSError, EOFError):
                    break
        self.log.extend(data)
        return data.decode(errors="replace")

    def expect(self, *needles, timeout=5):
        text = ""
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            text += self.read()
            if all(needle in text for needle in needles):
                return text
        raise AssertionError(f"Missing {needles!r}: {text[-12000:]!r}")

    def send(self, keys):
        if self.windows:
            self.process.write(keys)
        else:
            os.write(self.master, keys.encode())

    def quit(self):
        self.send("q")
        self.expect("\x1b[?1049l", "\x1b[?25h")
        if self.windows:
            deadline = time.monotonic() + 3
            while self.process.isalive() and time.monotonic() < deadline:
                self.read(0.1)
            assert not self.process.isalive(), "Process did not exit"
            assert self.process.exitstatus == 0, self.process.exitstatus
        else:
            self.process.wait(timeout=3)
            assert self.process.returncode == 0, self.process.returncode
            assert termios.tcgetattr(self.slave) == self.original, "Terminal attributes not restored"

    def close(self):
        if self.windows:
            self.process.close(force=True)
        else:
            if self.process.poll() is None:
                self.process.kill()
                self.process.wait()
            os.close(self.master)
            os.close(self.slave)


def replay_time(nanoseconds):
    ms = nanoseconds // 1000000
    return f"{ms // 3600000:02d}:{ms // 60000 % 60:02d}:{ms // 1000 % 60:02d}.{ms % 1000:03d}"


def verify_recording(binary, path):
    with open(path, encoding="utf-8") as source:
        records = [json.loads(line) for line in source]
    assert records[0]["type"] == "start", records[0]
    assert records[-1]["type"] == "end", records[-1]
    kinds = [record["type"] for record in records]
    assert all(kind in kinds for kind in ("probe", "pause", "resume", "reset", "path_end")), kinds
    assert [r["seq"] for r in records] == list(range(1, len(records) + 1)), "Non-contiguous sequence"
    assert all(a["elapsed_ns"] <= b["elapsed_ns"] for a, b in zip(records, records[1:])), "Playback clock moved backwards"
    reset = next(r for r in records if r["type"] == "reset")
    probes = [r["probe"] for r in records if r["type"] == "probe" and r["generation"] == reset["generation"]]
    assert probes, "No probes recorded after reset"
    result = subprocess.run([binary, "--mtr-replay", path, "--json"],
                            capture_output=True, text=True, timeout=10, check=True)
    summary = json.loads(result.stdout)
    assert summary["complete"] is True, summary
    assert summary["generation"] == reset["generation"], summary
    assert len(summary["stats"]) == 1, summary
    stat = summary["stats"][0]
    successful = [p["rtt_ns"] / 1000000 for p in probes if p["success"]]
    assert stat["snt"] == len(probes), summary
    assert stat["received"] == len(successful), summary
    assert stat["ip"] == "127.0.0.1", summary
    if successful:
        expected = {"last_ms": successful[-1], "avg_ms": sum(successful) / len(successful),
                    "best_ms": min(successful), "wrst_ms": max(successful)}
        for key, value in expected.items():
            assert math.isclose(stat[key], value, rel_tol=1e-12, abs_tol=1e-9), (key, stat[key], value)
    return records[-1]["elapsed_ns"]


def verify_recording_signals(binary, directory, mode, log):
    # Unix delivers actual OS signals; ConPTY key input is a separate contract.
    for output_mode, flags in (("tui", mode), ("report", ["-r"]), ("raw", mode + ["--raw"])):
        for sig, expected_code in ((signal.SIGINT, 130), (signal.SIGTERM, 143)):
            path = os.path.join(directory, f"{output_mode}-{sig.name}.jsonl")
            terminal = Terminal([binary] + flags + [
                "--mtr-record", path, "-q", "1000", "-i", "40", "--timeout", "100",
                "-d", "disable-geoip", "-n", "-s", "127.0.0.1", "-m", "1", "127.0.0.1",
            ], log)
            try:
                deadline = time.monotonic() + 5
                ready = False
                while time.monotonic() < deadline:
                    terminal.read(0.05)
                    try:
                        with open(path, encoding="utf-8") as source:
                            ready = any('"type":"probe"' in line for line in source)
                    except FileNotFoundError:
                        pass
                    if ready:
                        break
                assert ready, f"{output_mode} did not record a probe before {sig.name}"
                terminal.process.send_signal(sig)
                deadline = time.monotonic() + 5
                while terminal.process.poll() is None and time.monotonic() < deadline:
                    terminal.read(0.05)
                assert terminal.process.poll() == expected_code, (output_mode, sig.name, terminal.process.poll())
                assert termios.tcgetattr(terminal.slave) == terminal.original, "Signal exit did not restore terminal"
                with open(path, encoding="utf-8") as source:
                    records = [json.loads(line) for line in source]
                assert records[-1]["type"] == "end", (output_mode, sig.name)
                assert records[-1]["end"]["end_reason"] == "interrupted", records[-1]
                assert records[-1]["end"]["signal"] == sig.name, records[-1]
            finally:
                terminal.close()


def main(binary):
    binary = os.path.abspath(binary)
    log = bytearray()
    help_result = subprocess.run([binary, "--help"], capture_output=True,
                                 text=True, timeout=5, check=True)
    for flag in ("--help", "--version"):
        result = subprocess.run([binary, "--mtr-replay", flag], capture_output=True,
                                text=True, timeout=5, check=True)
        assert result.stdout.strip(), f"Replay {flag} produced no output"
        if flag == "--help":
            assert "--mtr-replay FILE" in result.stdout
            assert "Offline only." in result.stdout
    # ntr selects MTR by default and deliberately omits the mode flag.
    explicit_mtr = bool(re.search(r"^\s+-t\s+--mtr\s", help_result.stdout, re.M))
    mode = ["--mtr"] if explicit_mtr else []
    try:
        with tempfile.TemporaryDirectory(prefix="mtr-session-pty-") as directory:
            path = os.path.join(directory, "loopback.ndjson")
            online = Terminal([binary] + mode + [
                "--mtr-record", path,
                "--mtr-columns", "loss,snt,received,avg", "-d", "disable-geoip", "-n",
                "-s", "127.0.0.1", "-m", "1", "-i", "100", "--timeout", "100",
                "--no-color", "127.0.0.1",
            ], log)
            try:
                online.expect("Rcv", "127.0.0.1")
                online.read(1.2)
                online.send("p")
                online.expect("Paused")
                online.read(0.4)  # Let probes already in flight finish.
                online.send(" ")
                online.expect("127.0.0.1")
                online.read(0.4)
                online.send("r")
                online.read(0.5)
                online.expect("127.0.0.1")
                online.quit()
            finally:
                online.close()
            duration = verify_recording(binary, path)
            assert duration >= 1000000000, "Recording too short to seek"
            offline = Terminal([binary, "--mtr-replay", path, "--no-color"], log)
            try:
                offline.expect("Replay", "Paused " + replay_time(duration), "Rcv")
                offline.send("J")
                offline.expect("Go to HH:MM:SS[.mmm]", "Time:")
                offline.send("\x1500:00:01.000\r")
                offline.expect("Paused 00:00:01.000/", "127.0.0.1")
                offline.send("d")
                offline.expect("History")
                offline.send("o")
                offline.expect("Fields:")
                offline.send("\x15la\r")
                offline.expect("Loss%", "Avg", "Paused")
                offline.send(" ")
                offline.expect("Playing")
                offline.send("p")
                offline.expect("Paused")
                offline.send("r")
                offline.expect("Paused 00:00:00.000/")
                offline.send(" ")
                offline.expect("Playing")
                offline.expect("Paused " + replay_time(duration) + "/", timeout=duration / 1000000000 + 5)
                offline.quit()
            finally:
                offline.close()
            if os.name != "nt":
                verify_recording_signals(binary, directory, mode, log)
        print(json.dumps({
            "platform": platform.system(), "binary": os.path.basename(binary), "session_pty": "PASS",
            "checks": ["replay help/version", "loopback recording", "pause/resume", "reset generation", "ordered file",
                       "offline count/RTT parity", "final snapshot", "time seek", "history", "column editor",
                       "play/pause", "rewind", "EOF pause", "terminal restore"]
                      + (["recorded SIGINT/SIGTERM"] if os.name != "nt" else []),
        }, indent=2))
    finally:
        with open(os.path.join(tempfile.gettempdir(), "mtr-session-pty.log"), "wb") as output:
            output.write(log)


if __name__ == "__main__":
    main(sys.argv[1])
