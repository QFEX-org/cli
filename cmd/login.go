package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/qfex/cli/internal/config"
)

var loginRestart bool

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save your API credentials",
	Long: `Save your QFEX API public key and secret key to the local config file.

To generate API keys:
  1. Sign in at https://qfex.com and select your profile (bottom right).
  2. Navigate to Developer Settings.
  3. Click "Generate public and secret API Keys".
  4. Copy the keys immediately — the secret is only shown once.

Full instructions: https://docs.qfex.com/api-reference/introduction`,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Environment (prod/uat) [prod]: ")
		envInput, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		env := strings.TrimSpace(envInput)
		if env == "" {
			env = "prod"
		}
		if env != "prod" && env != "uat" {
			return fmt.Errorf("environment must be 'prod' or 'uat'")
		}

		fmt.Print("Public key: ")
		publicKey, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		publicKey = strings.TrimSpace(publicKey)
		if publicKey == "" {
			return fmt.Errorf("public key cannot be empty")
		}

		fmt.Print("Secret key: ")
		secretBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return err
		}
		secretKey := strings.TrimSpace(string(secretBytes))
		if secretKey == "" {
			return fmt.Errorf("secret key cannot be empty")
		}

		cfg.PublicKey = publicKey
		cfg.SecretKey = secretKey
		if env == "uat" {
			cfg.Env = "uat"
		} else {
			cfg.Env = ""
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Logged in with public key %s (%s)\n", publicKey, env)
		fmt.Printf("Config saved to %s\n", config.Path())
		if cli.IsRunning() {
			if loginRestart {
				fmt.Println("Restarting daemon...")
				return runDaemonRestart(cmd, args)
			}
			fmt.Print("Daemon is running. Restart now to apply credentials? (y/N): ")
			answer, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(answer)) == "y" {
				fmt.Println("Restarting daemon...")
				return runDaemonRestart(cmd, args)
			}
		}
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove API credentials from the local config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.PublicKey = ""
		cfg.SecretKey = ""
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println("Credentials removed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)

	loginCmd.Flags().BoolVar(&loginRestart, "restart", false, "Restart the daemon after saving credentials")
}
