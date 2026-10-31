"""Minimal VT screen model for MTR PTY assertions across incremental updates."""

import re
import unicodedata


class Screen:
    def __init__(self, width=150, height=32):
        self.pending = ""
        self.resize(width, height)

    def resize(self, width, height):
        self.width, self.height = width, height
        self.rows = [[" "] * width for _ in range(height)]
        self.x = self.y = 0

    def text(self):
        return "\n".join("".join(row).rstrip() for row in self.rows)

    def newline(self):
        self.y += 1
        if self.y >= self.height:
            self.rows.pop(0)
            self.rows.append([" "] * self.width)
            self.y = self.height - 1

    def feed(self, text):
        text = self.pending + text
        self.pending = ""
        i = 0
        while i < len(text):
            char = text[i]
            if char == "\x1b":
                if text.startswith("\x1b]", i):
                    end = re.search(r"\x07|\x1b\\", text[i + 2:])
                    if not end:
                        self.pending = text[i:]
                        break
                    i += 2 + end.end()
                    continue
                match = re.match(r"\x1b\[([0-?]*)([ -/]*)([@-~])", text[i:])
                if match:
                    self.csi(match[1], match[3])
                    i += len(match[0])
                    continue
                if text[i:] == "\x1b" or text.startswith("\x1b[", i):
                    self.pending = text[i:]
                    break
                i += 2
                continue
            if char == "\r":
                self.x = 0
            elif char == "\n":
                self.newline()
            elif char == "\b":
                self.x = max(0, self.x - 1)
            elif char >= " ":
                if unicodedata.combining(char):
                    if self.x > 0:
                        self.rows[self.y][min(self.x, self.width) - 1] += char
                    i += 1
                    continue
                width = 2 if unicodedata.east_asian_width(char) in "WF" else 1
                if self.x + width > self.width:
                    self.x = 0
                    self.newline()
                self.rows[self.y][self.x] = char
                if width == 2:
                    self.rows[self.y][self.x + 1] = ""
                self.x += width
            i += 1

    def csi(self, parameters, command):
        if parameters.startswith("?"):
            if command == "h" and "1049" in parameters[1:].split(";"):
                self.resize(self.width, self.height)
            return
        values = [int(value) if value else 0 for value in parameters.split(";")]
        value = values[0]
        amount = value or 1
        if command in "Hf":
            self.y = min(self.height - 1, max(0, amount - 1))
            self.x = min(self.width - 1, max(0, (values[1] if len(values) > 1 else 1) - 1))
        elif command == "K":
            start = 0 if value in (1, 2) else min(self.x, self.width - 1)
            end = min(self.x + 1, self.width) if value == 1 else self.width
            self.rows[self.y][start:end] = [" "] * (end - start)
        elif command == "J":
            if value in (2, 3):
                self.rows = [[" "] * self.width for _ in range(self.height)]
            elif value == 0:
                self.rows[self.y][self.x:] = [" "] * (self.width - self.x)
                for row in range(self.y + 1, self.height):
                    self.rows[row] = [" "] * self.width
            elif value == 1:
                for row in range(self.y):
                    self.rows[row] = [" "] * self.width
                end = min(self.x + 1, self.width)
                self.rows[self.y][:end] = [" "] * end
        elif command == "A":
            self.y = max(0, self.y - amount)
        elif command == "B":
            self.y = min(self.height - 1, self.y + amount)
        elif command == "C":
            self.x = min(self.width - 1, self.x + amount)
        elif command == "D":
            self.x = max(0, self.x - amount)
        elif command == "G":
            self.x = min(self.width - 1, amount - 1)
        elif command == "d":
            self.y = min(self.height - 1, amount - 1)
