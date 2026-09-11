"""Exercise the real MTR TUI on a PTY using loopback probes only.

Usage: python3 scripts/test_mtr_columns_pty.py /path/to/nexttrace
Windows requires pywinpty==3.0.5 for ConPTY.
Linux needs permission to open ICMP sockets (CI runs this under sudo).
"""

import json
import os
import platform
import re
import select
import struct
import subprocess
import sys
import tempfile
import time

if os.name != "nt":
    import fcntl
    import pty
    import termios


def main(binary):
    log = bytearray()
    windows = os.name == "nt"
    if not windows:
        master, slave = pty.openpty()
        original = termios.tcgetattr(slave)

    def resize(width, height=32):
        if windows:
            process.setwinsize(height, width)
        else:
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))

    def read(seconds=0.3):
        end = time.monotonic() + seconds
        data = bytearray()
        while time.monotonic() < end:
            source = process.fileobj if windows else master
            if select.select([source], [], [], max(0, end - time.monotonic()))[0]:
                try:
                    chunk = process.read(65536).encode() if windows else os.read(master, 65536)
                    if not chunk:
                        break
                    data.extend(chunk)
                except (OSError, EOFError):
                    break
        log.extend(data)
        return data.decode(errors="replace")

    def expect(*needles):
        text = ""
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            text += read()
            if all(needle in text for needle in needles):
                return text
        raise AssertionError(f"Missing {needles!r}: {text!r}")

    def send(keys):
        if windows:
            process.write(keys)
        else:
            os.write(master, keys.encode())

    def counters(text):
        rows = re.findall(r"^ 1\. 127\.0\.0\.1\s+(.+)$", text, re.M)
        assert rows, text
        return rows[-1].split()

    help_result = subprocess.run([binary, "--help"], capture_output=True, text=True,
                                 check=True, timeout=10)
    # ntr defaults to MTR and does not register --mtr.
    mode = ["--mtr"] if re.search(r"^\s+-t\s+--mtr\s", help_result.stdout, re.M) else []
    command = [os.path.abspath(binary)] + mode + [
        "--mtr-columns", "loss,snt,received,avg",
        "-d", "disable-geoip", "-n", "-s", "127.0.0.1", "-m", "1",
        "-i", "100", "--timeout", "100", "--no-color", "127.0.0.1",
    ]
    if windows:
        from winpty import PtyProcess
        process = PtyProcess.spawn(command, dimensions=(32, 120))
    else:
        resize(120)
        process = subprocess.Popen(command, stdin=slave, stdout=slave, stderr=slave)

    try:
        expect("Rcv", " 1. 127.0.0.1")
        send("p")
        paused = expect("Paused") + read(0.6)  # Drain any in-flight probes.
        send("o")
        expect("Fields: LSRA_", "\x1b[?2004h")
        send("\x15\x1b[200~r\nl s n a b w v\x1b[201~")
        expect("Fields: r l s n a b w v_")
        send("\r")
        applied = expect("Rcv", "Last", "Paused", "\x1b[?2004l")
        old, new = counters(paused), counters(applied)
        assert old[1:3] == [new[2], new[0]], "Editing changed paused counters"

        resize(24)
        expect("Select fewer columns")
        send("o")
        expect("Fields: R L S N A B W V_", "R: Received", "N: Newest")
        send("\x15l\x1b")
        expect("Select fewer columns")  # Bare Esc needs no subsequent read.
        resize(120)
        expect("Rcv", "Paused")
        send("d")
        expect("History")
        send("o\x15ar\r")
        applied = expect("Avg", "Rcv", "Paused")
        assert "History" not in applied.split("\x1b[H\x1b[2J")[-1], applied
        resize(160, 24)
        send("o")
        expect("Fields:", "G: Geometric Mean", "I: Interarrival Jitter")
        send("\x15 DRSGJMXI \r")
        expect("Drop", "Gmean", "Jttr", "Javg", "Jmax", "Jint", "Paused")
        send("o")
        expect("Fields:  DRSGJMXI _")
        resize(80, 10)
        expect("Fields:  DRSGJMXI _", "Enlarge terminal", "Enter: apply")
        send("\x15GG\r")
        expect("duplicate MTR column gmean")
        send("\x1b")
        expect("Drop", "Jint", "Paused")
        send("q")
        expect("\x1b[?1049l", "\x1b[?25h")
        if windows:
            deadline = time.monotonic() + 3
            while process.isalive() and time.monotonic() < deadline:
                read(0.1)
            assert not process.isalive(), "Process did not exit"
            assert process.exitstatus == 0, process.exitstatus
        else:
            process.wait(timeout=3)
            assert process.returncode == 0, process.returncode
            assert termios.tcgetattr(slave) == original, "Terminal attributes not restored"
        print(json.dumps({
            "platform": platform.system(), "pty": "PASS",
            "checks": ["loopback probes", "pause", "prefill", "paste newline",
                       "apply", "unchanged paused counters", "narrow notice",
                       "narrow editor", "bare Esc", "resize while paused",
                       "history apply", "expanded metrics", "space roundtrip",
                       "short editor", "duplicate metric", "terminal restore"],
        }, indent=2))
    finally:
        if windows:
            process.close(force=True)
        elif process.poll() is None:
            process.kill()
            process.wait()
        with open(os.path.join(tempfile.gettempdir(), "mtr-columns-pty.log"), "wb") as output:
            output.write(log)
        if not windows:
            os.close(master)
            os.close(slave)


if __name__ == "__main__":
    main(sys.argv[1])
