package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/client"
	"github.com/qfex/cli/internal/config"
	"github.com/qfex/cli/internal/oauth"
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
	Long:  `qfex is a CLI for interacting with the QFEX perpetual futures exchange.`,
	Run: func(cmd *cobra.Command, args []string) {
		env := "prod (qfex.com)"
		if cfg.IsUAT() {
			env = "UAT (qfex.io)"
		}
		fmt.Fprintf(os.Stderr, "Environment: %s\n", env)
		if cfg.HasJWT() {
			if email := oauth.EmailFromToken(cfg.AccessToken); email != "" {
				fmt.Fprintf(os.Stderr, "Logged in as %s\n\n", email)
			} else {
				fmt.Fprintf(os.Stderr, "Logged in\n\n")
			}
		} else if cfg.PublicKey != "" {
			fmt.Fprintf(os.Stderr, "Logged in   public key: %s\n\n", cfg.PublicKey)
		} else {
			fmt.Fprintf(os.Stderr, "Not logged in  run 'qfex login' to authenticate\n\n")
		}
		cmd.Help()
	},
}

func Execute() {
	rootCmd.SilenceUsage = true
	if err := rootCmd.Execute(); err != nil {
		if shouldShowUsage(err) {
			showUsageForArgs(os.Args[1:])
		}
		os.Exit(1)
	}
}

func shouldShowUsage(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "unknown flag") ||
		strings.Contains(msg, "flag needs an argument") ||
		strings.Contains(msg, "accepts") ||
		strings.Contains(msg, "requires at least") ||
		strings.Contains(msg, "requires at most") ||
		strings.Contains(msg, "requires exactly") ||
		strings.Contains(msg, "argument")
}

func showUsageForArgs(args []string) {
	cmd, _, err := rootCmd.Find(args)
	if err != nil || cmd == nil {
		_ = rootCmd.Help()
		return
	}
	_ = cmd.Help()
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

// requireDaemon ensures the daemon is running, starting it automatically if needed.
func requireDaemon() {
	if cli.IsRunning() {
		return
	}
	fmt.Fprintln(os.Stderr, "Starting daemon...")
	if err := runDaemonStart(rootCmd, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to start daemon: %v\n", err)
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
