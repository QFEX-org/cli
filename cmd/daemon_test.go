package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestRestartDaemonIfRunningStopsAndStartsWhenRunning(t *testing.T) {
	oldIsRunning := daemonIsRunning
	oldRestart := daemonRestart
	t.Cleanup(func() {
		daemonIsRunning = oldIsRunning
		daemonRestart = oldRestart
	})

	called := false
	daemonIsRunning = func() bool { return true }
	daemonRestart = func(cmd *cobra.Command, args []string) error {
		called = true
		return nil
	}

	if err := restartDaemonIfRunning(&cobra.Command{}); err != nil {
		t.Fatalf("restartDaemonIfRunning() error = %v", err)
	}

	if !called {
		t.Fatalf("daemon restart was not called")
	}
}

func TestRestartDaemonIfRunningNoopsWhenDaemonNotRunning(t *testing.T) {
	oldIsRunning := daemonIsRunning
	oldRestart := daemonRestart
	t.Cleanup(func() {
		daemonIsRunning = oldIsRunning
		daemonRestart = oldRestart
	})

	daemonIsRunning = func() bool { return false }
	daemonRestart = func(cmd *cobra.Command, args []string) error {
		return errors.New("should not be called")
	}

	if err := restartDaemonIfRunning(&cobra.Command{}); err != nil {
		t.Fatalf("restartDaemonIfRunning() error = %v", err)
	}
}
