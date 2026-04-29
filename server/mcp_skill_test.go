package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextTraceSkillFilesAndToolNames(t *testing.T) {
	root := filepath.Join("..", "skills", "nexttrace")
	files := []string{
		"SKILL.md",
		filepath.Join("references", "mcp-tools.md"),
		filepath.Join("references", "globalping.md"),
		filepath.Join("references", "capability-matrix.md"),
		filepath.Join("references", "cli-fallback.md"),
		filepath.Join("references", "platform-notes.md"),
		filepath.Join("references", "validation.md"),
	}
	for _, file := range files {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Fatalf("skill file %s missing: %v", file, err)
		}
	}

	toolsDoc, err := os.ReadFile(filepath.Join(root, "references", "mcp-tools.md"))
	if err != nil {
		t.Fatalf("read mcp-tools.md: %v", err)
	}
	skillDoc, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	matrixDoc, err := os.ReadFile(filepath.Join(root, "references", "capability-matrix.md"))
	if err != nil {
		t.Fatalf("read capability-matrix.md: %v", err)
	}
	toolNames := []string{
		"nexttrace_capabilities",
		"nexttrace_traceroute",
		"nexttrace_mtr_report",
		"nexttrace_mtr_raw",
		"nexttrace_mtu_trace",
		"nexttrace_speed_test",
		"nexttrace_annotate_ips",
		"nexttrace_geo_lookup",
		"nexttrace_globalping_trace",
		"nexttrace_globalping_limits",
		"nexttrace_globalping_get_measurement",
	}
	for _, name := range toolNames {
		if !strings.Contains(string(toolsDoc), name) {
			t.Fatalf("mcp-tools.md missing tool name %s", name)
		}
		if !strings.Contains(string(skillDoc), name) {
			t.Fatalf("SKILL.md missing tool name %s", name)
		}
		if !strings.Contains(string(matrixDoc), name) {
			t.Fatalf("capability-matrix.md missing tool name %s", name)
		}
	}
	if !strings.Contains(string(skillDoc), "server/mcp.go") {
		t.Fatal("SKILL.md missing server/mcp.go sync reminder")
	}
}
