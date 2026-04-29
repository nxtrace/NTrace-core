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
	for _, name := range []string{
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
	} {
		if !strings.Contains(string(toolsDoc), name) {
			t.Fatalf("mcp-tools.md missing tool name %s", name)
		}
	}
}
