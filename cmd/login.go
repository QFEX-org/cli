package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/qfex/cli/internal/config"
	"github.com/qfex/cli/internal/oauth"
)

var loginAPIKey bool

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with QFEX",
	Long: `Authenticate with QFEX.

By default, opens your browser to log in with your QFEX account (Google / email).
The access token is saved locally and used for all subsequent commands.

To use API keys (public key + secret key) instead, pass --api-key:

  qfex login --api-key

To generate API keys:
  1. Sign in at https://qfex.com and select your profile (bottom right).
  2. Navigate to Developer Settings.
  3. Click "Generate public and secret API Keys".
  4. Copy the keys immediately — the secret is only shown once.

Full instructions: https://docs.qfex.com/api-reference/introduction`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginAPIKey {
			return runAPIKeyLogin(cmd, args)
		}
		return runBrowserLogin(cmd, args)
	},
}

func runBrowserLogin(cmd *cobra.Command, args []string) error {
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

	authURL := oauth.ProdAuthURL
	clientID := oauth.ProdClientID
	if env == "uat" {
		authURL = oauth.UATAuthURL
		clientID = oauth.UATClientID
	}

	tokens, err := oauth.RunBrowserFlow(context.Background(), oauth.Config{
		AuthURL:  authURL,
		ClientID: clientID,
	})
	if err != nil {
		return err
	}

	cfg.AccessToken = tokens.AccessToken
	cfg.RefreshToken = tokens.RefreshToken
	// Clear any stale API key credentials.
	cfg.PublicKey = ""
	cfg.SecretKey = ""
	if env == "uat" {
		cfg.Env = "uat"
	} else {
		cfg.Env = ""
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	if email := oauth.EmailFromToken(tokens.AccessToken); email != "" {
		fmt.Printf("Logged in as %s (%s). Config saved to %s\n", email, env, config.Path())
	} else {
		fmt.Printf("Logged in (%s). Config saved to %s\n", env, config.Path())
	}
	fmt.Println("Restarting daemon...")
	return runDaemonRestart(cmd, args)
}

func runAPIKeyLogin(cmd *cobra.Command, args []string) error {
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
	// Clear any stale JWT credentials.
	cfg.AccessToken = ""
	cfg.RefreshToken = ""
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
	fmt.Println("Restarting daemon...")
	return runDaemonRestart(cmd, args)
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove credentials from the local config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.PublicKey = ""
		cfg.SecretKey = ""
		cfg.AccessToken = ""
		cfg.RefreshToken = ""
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

	loginCmd.Flags().BoolVar(&loginAPIKey, "api-key", false, "Use API key login (public key + secret key) instead of browser")
}
