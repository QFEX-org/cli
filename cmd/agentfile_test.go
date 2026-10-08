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

func TestWriteAgentFileKeepsBlankLinesAfterSection(t *testing.T) {
	// Refreshing the block must not consume the blank line separating it from
	// whatever the user keeps below, and an unchanged file must not be rewritten.
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	original := agentBlock(agentMDContent) + "\n" + userContent
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeAgentFile(path, agentMDContent); err != nil {
		t.Fatalf("writeAgentFile: %v", err)
	}

	if got := readFile(t, path); got != original {
		t.Errorf("refresh changed an up-to-date file:\n got %q\nwant %q", got, original)
	}
}

func TestWriteAgentFileUpgradesUnmarkedSectionWithCodeComments(t *testing.T) {
	// The qfex section has shell comments like "# Extract mid price" in a code
	// block. They are not headings, so the whole legacy section is replaced and
	// the user's section after it is kept.
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	notes := "# My notes\n\nKeep this.\n"
	if err := os.WriteFile(path, []byte(agentMDContent+"\n"+notes), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeAgentFile(path, agentMDContent); err != nil {
		t.Fatalf("writeAgentFile: %v", err)
	}

	got := readFile(t, path)
	if n := strings.Count(got, "# Extract mid price"); n != 1 {
		t.Errorf("the code block appears %d times, want 1", n)
	}
	if !strings.HasSuffix(got, agentBlockEnd+"\n\n"+notes) {
		t.Errorf("the user's section after the qfex section was not kept:\n%s", got)
	}
}

func TestWriteAgentFileUpgradesDriftedUnmarkedSection(t *testing.T) {
	// The unmarked section on disk was written by an earlier release, so it does
	// not match the current content verbatim. It must still be upgraded in
	// place, not left beside the new block as a stale second qfex section.
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	stale := "# qfex CLI\n\nInstructions from an earlier release.\n"
	if err := os.WriteFile(path, []byte(userContent+"\n"+stale), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeAgentFile(path, agentMDContent); err != nil {
		t.Fatalf("writeAgentFile: %v", err)
	}

	got := readFile(t, path)
	if n := strings.Count(got, "# qfex CLI"); n != 1 {
		t.Errorf("qfex section appears %d times, want 1", n)
	}
	if strings.Contains(got, "Instructions from an earlier release.") {
		t.Error("the superseded section was left behind")
	}
	if !strings.Contains(got, "Always answer in metric units.") {
		t.Error("the user's own instructions were lost")
	}
}
