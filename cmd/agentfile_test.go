package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const userContent = `# My global instructions

Always answer in metric units.
Never force-push to main.
`

func TestWriteAgentFilePreservesExistingContent(t *testing.T) {
	// ~/.claude/CLAUDE.md holds the user's own global Claude Code instructions.
	// Adding the qfex section must not cost them that file.
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(path, []byte(userContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeAgentFile(path, agentMDContent); err != nil {
		t.Fatalf("writeAgentFile: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, "Always answer in metric units.") {
		t.Error("the user's own instructions were lost")
	}
	if !strings.Contains(got, "Never force-push to main.") {
		t.Error("the user's own instructions were truncated")
	}
	if !strings.Contains(got, "# qfex CLI") {
		t.Error("the qfex section was not added")
	}
}

func TestWriteAgentFileIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(path, []byte(userContent), 0644); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if err := writeAgentFile(path, agentMDContent); err != nil {
			t.Fatalf("writeAgentFile: %v", err)
		}
	}

	got := readFile(t, path)
	if n := strings.Count(got, agentBlockBegin); n != 1 {
		t.Errorf("qfex section appears %d times, want 1", n)
	}
	if n := strings.Count(got, "Always answer in metric units."); n != 1 {
		t.Errorf("user content appears %d times, want 1", n)
	}
}

func TestWriteAgentFileUpgradesUnmarkedSection(t *testing.T) {
	// A file written by an earlier qfex version holds the bare content with no
	// markers. It should be wrapped in place, not duplicated.
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(path, []byte(userContent+"\n"+agentMDContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeAgentFile(path, agentMDContent); err != nil {
		t.Fatalf("writeAgentFile: %v", err)
	}

	got := readFile(t, path)
	if n := strings.Count(got, "# qfex CLI"); n != 1 {
		t.Errorf("qfex section appears %d times, want 1", n)
	}
	if !strings.Contains(got, "Always answer in metric units.") {
		t.Error("the user's own instructions were lost")
	}
	if !strings.Contains(got, agentBlockBegin) {
		t.Error("the legacy section was not wrapped in markers")
	}
}

func TestWriteAgentFileCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := writeAgentFile(path, agentMDContent); err != nil {
		t.Fatalf("writeAgentFile: %v", err)
	}
	if got := readFile(t, path); !strings.Contains(got, "# qfex CLI") {
		t.Error("the qfex section was not written")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
