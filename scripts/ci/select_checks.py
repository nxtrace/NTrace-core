#!/usr/bin/env python3
"""Select CI checks from local Git history; keep native workflow paths in sync."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
POLICY = ROOT / ".github/ci-paths.json"
TEST_CHECKS = ("go", "pty", "doctor", "dns", "install", "web", "shell")
WORKFLOWS = {
    "test": TEST_CHECKS,
    "build": ("build",),
    "golangci-lint": ("lint",),
    "tos": ("tos",),
    "fwmark": ("fwmark",),
    "regression": ("regression",),
    "performance": ("performance",),
}


def matches(path, pattern):
    # The policy uses only GitHub's *, **, and **/ globs. A bare ** includes /.
    tokens = re.split(r"(\*\*/|\*\*|\*)", pattern)
    special = {"**/": "(?:.*/)?", "**": ".*", "*": "[^/]*"}
    return re.fullmatch("".join(special.get(t, re.escape(t)) for t in tokens), path) is not None


def load_rules():
    policy = json.loads(POLICY.read_text())
    return {
        check: list(dict.fromkeys(
            policy["shared"] + [p for group in groups for p in policy["groups"][group]]
        ))
        for check, groups in policy["checks"].items()
    }


def classify(paths, rules, full=False):
    own_workflow = ".github/workflows/test.yml" in paths
    return {
        check: full or own_workflow or any(matches(p, g) for p in paths for g in rules[check])
        for check in TEST_CHECKS
    }


def platforms(selected):
    if any(selected[c] for c in ("go", "pty", "doctor")):
        return ["windows-latest", "ubuntu-latest", "macos-latest"]
    if selected["install"]:
        return ["ubuntu-latest", "macos-latest"]
    return ["ubuntu-latest"]


def git(repo, *args):
    return subprocess.check_output(["git", "-C", str(repo), *args], stderr=subprocess.PIPE)


def changed_paths(event, payload, repo=ROOT):
    if event == "workflow_dispatch":
        return [], "manual run: all Test checks enabled"
    try:
        if event == "pull_request":
            base = payload["pull_request"]["base"]["sha"]
            head = payload["pull_request"]["head"]["sha"]
        elif event == "push":
            base, head = payload["before"], payload["after"]
        else:
            raise ValueError(f"unsupported event: {event}")
        if not all(isinstance(ref, str) and re.fullmatch(r"[0-9a-f]{40}", ref)
                   and ref != "0" * 40 for ref in (base, head)):
            raise ValueError("missing or invalid comparison commit")
        if event == "pull_request":
            base = git(repo, "merge-base", base, head).decode().strip()
        # Disable rename detection so both old and new paths are classified.
        raw = git(repo, "diff", "--no-renames", "--name-only", "-z", base, head, "--")
        return [p.decode("utf-8", errors="surrogateescape") for p in raw.split(b"\0") if p], ""
    except (KeyError, TypeError, ValueError, OSError, subprocess.CalledProcessError) as exc:
        return [], f"cannot reliably read changed paths ({exc}); all Test checks enabled"


def workflow_header(name, rules):
    paths = sorted({p for c in WORKFLOWS[name] for p in rules[c]}
                   | {f".github/workflows/{name}.yml"})
    lines = ["on:", "  workflow_dispatch:", "  push:", "    branches: [main]"]
    if name == "build":
        lines.append('    tags: ["v*"]')
    for event in ("push", "pull_request"):
        if event == "pull_request":
            lines.extend(["  pull_request:", "    types: [opened, synchronize, reopened]"])
        lines.append("    paths:")
        lines.extend("      - " + json.dumps(p) for p in paths)
    return "\n".join(lines) + "\n\n"


def sync_workflows(rules, write=False):
    mismatched = []
    for name in WORKFLOWS:
        path = ROOT / f".github/workflows/{name}.yml"
        source = path.read_text()
        # Only the top-level event block is generated; jobs remain hand-written.
        updated, count = re.subn(r"(?ms)^on:\n.*?(?=^[A-Za-z][\w-]*:)",
                                 lambda _: workflow_header(name, rules), source, count=1)
        if count != 1:
            raise ValueError(f"cannot locate event block in {path}")
        if updated != source:
            mismatched.append(str(path.relative_to(ROOT)))
            if write:
                path.write_text(updated)
    return mismatched


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check-workflows", action="store_true")
    parser.add_argument("--sync-workflows", action="store_true")
    args = parser.parse_args()
    rules = load_rules()
    if args.check_workflows or args.sync_workflows:
        changed = sync_workflows(rules, write=args.sync_workflows)
        if changed:
            print("Workflow event filters: " + ", ".join(changed))
        return int(bool(changed) and not args.sync_workflows)

    try:
        payload = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
        paths, reason = changed_paths(os.environ.get("GITHUB_EVENT_NAME", ""), payload)
    except (KeyError, OSError, ValueError) as exc:
        paths, reason = [], f"cannot read event ({exc}); all Test checks enabled"
    selected = classify(paths, rules, full=bool(reason))
    output = "".join(f"{k}={str(v).lower()}\n" for k, v in selected.items())
    output += "platforms=" + json.dumps(platforms(selected)) + "\n"
    print(reason or f"Compared complete event diff: {len(paths)} changed paths")
    print(output, end="")
    with open(os.environ["GITHUB_OUTPUT"], "a") as handle:
        handle.write(output)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as handle:
            handle.write("## Test check selection\n\n" + (reason or "Complete event diff read successfully")
                         + "\n\n```text\n" + output + "```\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
