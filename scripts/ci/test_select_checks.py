import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import select_checks as ci


class SelectionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.rules = ci.load_rules()

    def selected(self, *paths):
        return {k for k, v in ci.classify(paths, self.rules).items() if v}

    def test_docs_do_not_trigger_checks(self):
        for path in ("README.md", "docs/mtr-session.md", "scripts/perf/README.md",
                     "scripts/regression/README.md"):
            self.assertEqual(self.selected(path), set())
            for workflow in ci.WORKFLOWS:
                with self.subTest(path=path, workflow=workflow):
                    self.assertFalse(any(ci.matches(path, p)
                                         for c in ci.WORKFLOWS[workflow] for p in self.rules[c]))

    def test_printer_selects_go_and_pty_without_packet_acceptance(self):
        path = "printer/mtr_screen.go"
        self.assertEqual(self.selected(path), {"go", "pty"})
        for check in ("tos", "fwmark"):
            self.assertFalse(any(ci.matches(path, p) for p in self.rules[check]))

    def test_probe_backend_selects_packet_and_integration_checks(self):
        path = "trace/internal/icmp_darwin.go"
        self.assertEqual(self.selected(path), {"go", "pty", "doctor"})
        for check in ("tos", "fwmark", "regression", "performance"):
            self.assertTrue(any(ci.matches(path, p) for p in self.rules[check]), check)

    def test_acceptance_runtime_dependencies_are_not_skipped(self):
        for path in ("cmd/mtr_json.go", "internal/mtrsession/model.go",
                     "printer/mtr_raw.go", "assets/windivert/divert.go",
                     "assets/windivert/x64/WinDivert64.sys"):
            for check in ("tos", "fwmark"):
                with self.subTest(path=path, check=check):
                    self.assertTrue(any(ci.matches(path, p) for p in self.rules[check]))
        self.assertIn("pty", self.selected("cmd/fwmark.go"))
        self.assertIn("go", self.selected("assets/windivert/x64/WinDivert.dll"))

    def test_specialized_changes(self):
        cases = {
            "internal/dnsclient/transport.go": {"go", "dns"},
            "nt_install.sh": {"install"},
            "scripts/test_nt_install.sh": {"install"},
            "server/web/assets/app.js": {"web"},
            "server/web/assets/app.test.cjs": {"web"},
            "scripts/mtr_pty_screen.py": {"pty"},
            "scripts/test_mtr_columns_pty.py": {"pty"},
            "scripts/test_mtr_session_pty.py": {"pty"},
            "scripts/test_mtr_snapshot_pty.py": {"pty"},
            "internal/doctor/route_linux.go": {"go", "doctor"},
        }
        for path, want in cases.items():
            with self.subTest(path=path):
                self.assertEqual(self.selected(path), want)

    def test_shared_inputs_and_test_workflow_enable_complete_test_suite(self):
        for path in ("go.mod", "go.sum", "default.pgo", ".cross_compile.sh",
                     ".github/ci-paths.json", "scripts/ci/select_checks.py",
                     ".github/workflows/test.yml"):
            with self.subTest(path=path):
                self.assertEqual(self.selected(path), set(ci.TEST_CHECKS))

    def test_native_filters_are_synchronized(self):
        self.assertEqual(ci.sync_workflows(self.rules), [])

    def test_own_workflow_does_not_trigger_other_workflows(self):
        for name in ci.WORKFLOWS:
            path = f".github/workflows/{name}.yml"
            for other in ci.WORKFLOWS:
                header = ci.workflow_header(other, self.rules)
                self.assertEqual(json.dumps(path) in header, name == other)

    def test_manual_and_failure_enable_all_checks(self):
        for event, payload in (("workflow_dispatch", {}), ("push", {}),
                               ("pull_request", {}), ("unknown", {})):
            with self.subTest(event=event):
                paths, reason = ci.changed_paths(event, payload)
                self.assertTrue(reason)
                self.assertTrue(all(ci.classify(paths, self.rules, full=bool(reason)).values()))

    def test_git_error_fails_open(self):
        with patch.object(ci, "git", side_effect=subprocess.CalledProcessError(128, "git")):
            paths, reason = ci.changed_paths("push", {"before": "a" * 40, "after": "b" * 40})
        self.assertEqual(paths, [])
        self.assertIn("all Test checks enabled", reason)

    def test_zero_and_invalid_refs_fail_open(self):
        for ref in ("0" * 40, "--help", "abc", None):
            self.assertTrue(ci.changed_paths("push", {"before": ref, "after": "a" * 40})[1])

    def test_globs_match_root_and_nested_files_without_crossing_single_star(self):
        self.assertTrue(ci.matches("main.go", "**/*.go"))
        self.assertTrue(ci.matches("trace/internal/a.go", "trace/**/*.go"))
        self.assertTrue(ci.matches("trace/a.go", "trace/**/*.go"))
        self.assertTrue(ci.matches("trace/testdata/case", "trace/**/testdata/**"))
        self.assertFalse(ci.matches("cmd/nested/a.go", "cmd/*.go"))
        self.assertFalse(ci.matches("trace/README.md", "trace/**/*.go"))

    def test_only_needed_platforms_start(self):
        for path, want in (("scripts/test_mtr_columns_pty.py", 3),
                           ("nt_install.sh", 2), ("internal/dnsclient/query.go", 3),
                           ("scripts/test_nt_install.sh", 2)):
            self.assertEqual(len(ci.platforms(ci.classify([path], self.rules))), want)

    def test_cli_writes_flags_and_fallback_reason(self):
        with tempfile.TemporaryDirectory() as directory:
            event = Path(directory) / "event.json"
            output = Path(directory) / "output"
            summary = Path(directory) / "summary"
            event.write_text("invalid json")
            environment = {"GITHUB_EVENT_PATH": str(event), "GITHUB_OUTPUT": str(output),
                           "GITHUB_STEP_SUMMARY": str(summary), "GITHUB_EVENT_NAME": "push"}
            with patch.dict(os.environ, environment), patch("sys.argv", ["select_checks.py"]):
                self.assertEqual(ci.main(), 0)
            self.assertTrue(all(f"{check}=true\n" in output.read_text() for check in ci.TEST_CHECKS))
            self.assertIn("all Test checks enabled", summary.read_text())

    def test_shared_rules_trigger_every_native_workflow(self):
        for path in ("go.mod", "go.sum", ".cross_compile.sh", ".github/ci-paths.json",
                     "scripts/ci/select_checks.py"):
            for workflow, checks in ci.WORKFLOWS.items():
                with self.subTest(path=path, workflow=workflow):
                    self.assertTrue(any(ci.matches(path, p) for c in checks for p in self.rules[c]))


class GitDiffTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.run_git("init", "-b", "main")
        self.run_git("config", "user.email", "ci-test@example.invalid")
        self.run_git("config", "user.name", "CI test")
        self.write("README.md", "base")
        self.base = self.commit()

    def run_git(self, *args):
        return ci.git(self.repo, *args).decode().strip()

    def write(self, path, text):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)

    def commit(self):
        self.run_git("add", "-A")
        self.run_git("commit", "-m", "test fixture")
        return self.run_git("rev-parse", "HEAD")

    def test_complete_pr_diff_includes_earlier_commits_not_base_only_changes(self):
        self.run_git("checkout", "-b", "feature")
        self.write("scripts/mtr_pty_screen.py", "first commit")
        self.commit()
        self.write("docs/note.md", "second commit")
        head = self.commit()
        self.run_git("checkout", "main")
        self.write("internal/dnsclient/base.go", "base only")
        advanced_base = self.commit()
        payload = {"pull_request": {"base": {"sha": advanced_base}, "head": {"sha": head}}}
        paths, reason = ci.changed_paths("pull_request", payload, self.repo)
        self.assertEqual(set(paths), {"scripts/mtr_pty_screen.py", "docs/note.md"})
        self.assertEqual(reason, "")

    def test_push_includes_added_deleted_and_both_rename_paths(self):
        self.write("printer/old.go", "old\n")
        self.write("trace/deleted.go", "delete\n")
        before = self.commit()
        self.run_git("mv", "printer/old.go", "printer/new.go")
        (self.repo / "trace/deleted.go").unlink()
        self.write("server/web/assets/new file.js", "added\n")
        after = self.commit()
        paths, reason = ci.changed_paths("push", {"before": before, "after": after}, self.repo)
        self.assertEqual(set(paths), {"printer/old.go", "printer/new.go", "trace/deleted.go",
                                     "server/web/assets/new file.js"})
        self.assertEqual(reason, "")


if __name__ == "__main__":
    unittest.main()
