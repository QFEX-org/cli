package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/client"
	"github.com/qfex/cli/internal/config"
	"github.com/qfex/cli/internal/protocol"
)

var (
	cfg        *config.Config
	cli        *client.Client
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "qfex",
	Short: "QFEX trading CLI",
	Long: `qfex is a CLI for interacting with the QFEX perpetual futures exchange.

It communicates with a background daemon that maintains WebSocket connections
to the QFEX Market Data Service and Trading API.

Run 'qfex daemon start' to start the daemon before using other commands.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip daemon check for daemon commands and config
		if cmd.Name() == "start" || cmd.Name() == "stop" ||
			cmd.Name() == "status" || cmd.Name() == "run" ||
			cmd.Name() == "config" || cmd.Name() == "qfex" {
			return nil
		}
		if !cli.IsRunning() {
			return fmt.Errorf("qfex daemon is not running\nRun 'qfex daemon start' to start it")
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", true, "Output as JSON (default)")
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
		cfg = &config.Config{}
	}
	cli = client.New(config.SocketPath())
}

// mustSend sends a command to the daemon and exits on error.
func mustSend(cmd string, params any) json.RawMessage {
	resp, err := cli.Send(rootCmd.Context(), cmd, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}
	return resp.Data
}

// printJSON prints JSON data, pretty-printed.
func printJSON(data json.RawMessage) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Println(string(data))
		return
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(string(data))
		return
	}
	fmt.Println(string(out))
}

// printResult prints command output as pretty JSON.
func printResult(data json.RawMessage) {
	printJSON(data)
}

// sendAndPrint is a convenience wrapper.
func sendAndPrint(cmd string, params any) {
	data := mustSend(cmd, params)
	printResult(data)
}

// requireDaemon checks daemon is running and fatals otherwise.
func requireDaemon() {
	if !cli.IsRunning() {
		fmt.Fprintln(os.Stderr, "Error: qfex daemon is not running\nRun 'qfex daemon start' to start it")
		os.Exit(1)
	}
}

// requireAuth checks that the daemon is authenticated for trading.
func requireAuth() {
	resp, err := cli.Send(rootCmd.Context(), protocol.CmdStatus, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		return
	}
	var status struct {
		TradeAuthed bool `json:"trade_authed"`
	}
	if json.Unmarshal(resp.Data, &status) == nil && !status.TradeAuthed {
		fmt.Fprintln(os.Stderr, "Warning: not authenticated to trading API\nSet public_key and secret_key in ~/.config/qfex/config.yaml")
	}
}
