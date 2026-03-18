package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/config"
)

// Version is set at build time via ldflags: -X github.com/qfex/cli/cmd.Version=v1.2.3
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}

var envCmd = &cobra.Command{
	Use:   "env [prod|uat]",
	Short: "Show or set the environment (prod or uat)",
	Args:  cobra.MaximumNArgs(1),
	ValidArgs: []string{"prod", "uat"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			if cfg.IsUAT() {
				fmt.Println("uat (qfex.io)")
			} else {
				fmt.Println("prod (qfex.com)")
			}
			return nil
		}

		target := args[0]
		if target != "prod" && target != "uat" {
			return fmt.Errorf("unknown environment %q: must be 'prod' or 'uat'", target)
		}

		if cfg.HasCredentials() {
			fmt.Fprintf(os.Stderr, "Error: you are logged in — run 'qfex logout' before switching environments\n")
			os.Exit(1)
		}

		current := "prod"
		if cfg.IsUAT() {
			current = "uat"
		}
		if target == current {
			fmt.Printf("Already on %s\n", target)
			return nil
		}

		cfg.Env = target
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Switched to %s\n", target)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(envCmd)
}
