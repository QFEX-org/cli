package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/config"
	"github.com/qfex/cli/internal/daemon"
	"github.com/qfex/cli/internal/protocol"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the qfex daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the qfex daemon",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the qfex daemon",
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE:  runDaemonStatus,
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the qfex daemon",
	RunE:  runDaemonRestart,
}

// daemonRunCmd is the hidden command that actually runs the daemon process.
var daemonRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run the daemon in the foreground (internal use)",
	Hidden: true,
	RunE:   runDaemonForeground,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonRestartCmd)
	daemonCmd.AddCommand(daemonRunCmd)
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	if cli.IsRunning() {
		fmt.Println(`{"status": "already_running"}`)
		return nil
	}

	// Ensure data dir exists
	if err := os.MkdirAll(config.DataDir(), 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	logFile, err := os.OpenFile(config.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer logFile.Close()

	proc := exec.Command(exe, "daemon", "run")
	proc.Stdin = nil
	proc.Stdout = logFile
	proc.Stderr = logFile
	proc.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := proc.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	// Wait for the daemon socket to be ready
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if cli.IsRunning() {
			break
		}
	}
	if !cli.IsRunning() {
		return fmt.Errorf("daemon did not start in time; check %s", config.LogPath())
	}

	// If credentials are configured, wait for trade authentication
	if cfg.HasCredentials() {
		authDeadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(authDeadline) {
			resp, err := cli.Send(cmd.Context(), protocol.CmdStatus, nil)
			if err == nil && resp.OK {
				var status struct {
					TradeAuthed bool `json:"trade_authed"`
				}
				if json.Unmarshal(resp.Data, &status) == nil && status.TradeAuthed {
					break
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	fmt.Printf(`{"status": "started", "pid": %d, "log": %q}`+"\n", proc.Process.Pid, config.LogPath())
	return nil
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	pidData, err := os.ReadFile(config.PIDPath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println(`{"status": "not_running"}`)
			return nil
		}
		return fmt.Errorf("reading PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(pidData))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("invalid PID %q: %w", pidStr, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println(`{"status": "not_running"}`)
		return nil
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Println(`{"status": "not_running"}`)
		return nil
	}

	// Wait for shutdown
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			fmt.Printf(`{"status": "stopped", "pid": %d}`+"\n", pid)
			return nil
		}
	}

	// Force kill
	proc.Signal(syscall.SIGKILL)
	fmt.Printf(`{"status": "killed", "pid": %d}`+"\n", pid)
	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	if !cli.IsRunning() {
		fmt.Println(`{"running": false}`)
		return nil
	}
	sendAndPrint("status", nil)
	return nil
}

func runDaemonRestart(cmd *cobra.Command, args []string) error {
	if err := runDaemonStop(cmd, args); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	return runDaemonStart(cmd, args)
}

func runDaemonForeground(cmd *cobra.Command, args []string) error {
	d := daemon.New(cfg, config.SocketPath())
	ctx := context.Background()
	return d.Run(ctx)
}
