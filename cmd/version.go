package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
	Use:   "env",
	Short: "Show the current environment (prod or UAT)",
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.IsUAT() {
			fmt.Println("uat (qfex.io)")
		} else {
			fmt.Println("prod (qfex.com)")
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(envCmd)
}
