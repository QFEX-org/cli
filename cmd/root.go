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

Commands that require live market data or trading (order, watch, account balance, etc.)
communicate with a background daemon. Run 'qfex daemon start' first for those.

REST-based commands (market refdata, market metrics, history, etc.) work without the daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		env := "prod (qfex.com)"
		if cfg.IsUAT() {
			env = "UAT (qfex.io)"
		}
		fmt.Fprintf(os.Stderr, "Environment: %s\n", env)
		if cfg.HasCredentials() {
			fmt.Fprintf(os.Stderr, "Logged in   public key: %s\n\n", cfg.PublicKey)
		} else {
			fmt.Fprintf(os.Stderr, "Not logged in  run 'qfex login' to add your API credentials\n\n")
		}
		cmd.Help()
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
		fmt.Fprintf(os.Stderr, "Warning: not authenticated to trading API\nRun 'qfex login' or set credentials in %s, then restart the daemon\n", config.Path())
	}
}
