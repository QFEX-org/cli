package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PublicKey string `yaml:"public_key"`
	SecretKey string `yaml:"secret_key"`

	// JWT auth — set by `qfex login` (browser OAuth flow).
	// Takes precedence over PublicKey/SecretKey when present.
	AccessToken  string `yaml:"access_token,omitempty"`
	RefreshToken string `yaml:"refresh_token,omitempty"`

	// Env selects the target environment: "prod" (default) or "uat".
	// UAT is identical to prod but uses qfex.io instead of qfex.com.
	Env string `yaml:"env,omitempty"`

	// UserID is the primary account ID (JWT "sub" claim), stored at login time.
	// Used to resolve "primary" in transfer --from/--to flags.
	UserID string `yaml:"user_id,omitempty"`

	// SelectedSubaccount stores the active child account to use for authenticated
	// REST requests and daemon trade auth. Empty means use the primary account.
	SelectedSubaccount string `yaml:"selected_subaccount,omitempty"`

	// Optional per-URL overrides (take precedence over Env).
	TradeWSURL string `yaml:"trade_ws_url,omitempty"`
	MDSURL     string `yaml:"mds_url,omitempty"`
}

func (c *Config) domain() string {
	if c.Env == "uat" {
		return "qfex.io"
	}
	return "qfex.com"
}

func (c *Config) TradeWS() string {
	if c.TradeWSURL != "" {
		return c.TradeWSURL
	}
	return "wss://trade." + c.domain() + "/"
}

func (c *Config) MDS() string {
	if c.MDSURL != "" {
		return c.MDSURL
	}
	return "wss://mds." + c.domain() + "/"
}

func (c *Config) BankerURL() string {
	return "https://banker." + c.domain() + "/address"
}

func (c *Config) APIURL() string {
	return "https://api." + c.domain()
}

func (c *Config) IsUAT() bool {
	return c.Env == "uat"
}

func (c *Config) HasCredentials() bool {
	return c.HasJWT() || (c.PublicKey != "" && c.SecretKey != "")
}

// HasJWT returns true when a JWT access token is stored (browser login).
func (c *Config) HasJWT() bool {
	return c.AccessToken != ""
}

func (c *Config) HasSelectedSubaccount() bool {
	return strings.TrimSpace(c.SelectedSubaccount) != ""
}

func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "qfex")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "qfex")
}

func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

func DataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "qfex")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "qfex")
}

func SocketPath() string {
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		return filepath.Join(runtime, "qfex-daemon.sock")
	}
	return filepath.Join(DataDir(), "daemon.sock")
}

func PIDPath() string {
	return filepath.Join(DataDir(), "daemon.pid")
}

func LogPath() string {
	return filepath.Join(DataDir(), "daemon.log")
}

func Load() (*Config, error) {
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), data, 0600)
}
