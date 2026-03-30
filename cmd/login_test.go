package cmd

import (
	"io"
	"strings"
	"testing"

	"github.com/qfex/cli/internal/config"
	"github.com/spf13/cobra"
)

func TestHandlePostLoginSubaccountSelectionWithNoSubaccountsClearsSelection(t *testing.T) {
	oldIsRunning := daemonIsRunning
	oldRestart := daemonRestart
	t.Cleanup(func() {
		daemonIsRunning = oldIsRunning
		daemonRestart = oldRestart
	})
	daemonIsRunning = func() bool { return false }
	daemonRestart = func(cmd *cobra.Command, args []string) error { return nil }

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg = &config.Config{SelectedSubaccount: "old-sub"}

	if err := handlePostLoginSubaccountSelection(&cobra.Command{}, strings.NewReader(""), io.Discard, nil); err != nil {
		t.Fatalf("handlePostLoginSubaccountSelection() error = %v", err)
	}

	if got := cfg.SelectedSubaccount; got != "" {
		t.Fatalf("SelectedSubaccount = %q, want empty", got)
	}
}

func TestHandlePostLoginSubaccountSelectionSetsSelectedSubaccountWhenUserChoosesOne(t *testing.T) {
	oldIsRunning := daemonIsRunning
	oldRestart := daemonRestart
	t.Cleanup(func() {
		daemonIsRunning = oldIsRunning
		daemonRestart = oldRestart
	})
	daemonIsRunning = func() bool { return false }
	daemonRestart = func(cmd *cobra.Command, args []string) error { return nil }

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg = &config.Config{}

	if err := handlePostLoginSubaccountSelection(&cobra.Command{}, strings.NewReader("2\n"), io.Discard, []string{"sub-1", "sub-2"}); err != nil {
		t.Fatalf("handlePostLoginSubaccountSelection() error = %v", err)
	}

	if got := cfg.SelectedSubaccount; got != "sub-2" {
		t.Fatalf("SelectedSubaccount = %q, want %q", got, "sub-2")
	}
}

func TestHandlePostLoginSubaccountSelectionLeavesSelectionUnsetWhenUserKeepsPrimary(t *testing.T) {
	oldIsRunning := daemonIsRunning
	oldRestart := daemonRestart
	t.Cleanup(func() {
		daemonIsRunning = oldIsRunning
		daemonRestart = oldRestart
	})
	daemonIsRunning = func() bool { return false }
	daemonRestart = func(cmd *cobra.Command, args []string) error { return nil }

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg = &config.Config{}

	if err := handlePostLoginSubaccountSelection(&cobra.Command{}, strings.NewReader("0\n"), io.Discard, []string{"sub-1", "sub-2"}); err != nil {
		t.Fatalf("handlePostLoginSubaccountSelection() error = %v", err)
	}

	if got := cfg.SelectedSubaccount; got != "" {
		t.Fatalf("SelectedSubaccount = %q, want empty", got)
	}
}
