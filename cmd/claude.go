package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// agentMDContent is written as both CLAUDE.md and AGENTS.md.
const agentMDContent = `# qfex CLI

You can run any ` + "`qfex`" + ` command freely — treat it like ` + "`curl`" + ` or ` + "`jq`" + `. All output is JSON on stdout, errors on stderr, non-zero exit code on failure.

## Key facts

- **No login needed for market data.** ` + "`qfex daemon start`" + ` is sufficient.
- **Daemon must be running** for market data and trading commands. Start it with ` + "`qfex daemon start`" + `. It is idempotent — safe to call even if already running.
- **REST commands work without the daemon**: ` + "`qfex market symbols`" + `, ` + "`qfex market refdata`" + `, ` + "`qfex market metrics`" + `, ` + "`qfex history *`" + `, ` + "`qfex account fees`" + `, ` + "`qfex account pnl`" + `, ` + "`qfex account deposit`" + `.
- **All output is JSON.** Pipe to ` + "`jq`" + ` for extraction.
- **` + "`--wait`" + ` on order place** blocks until the terminal status (FILLED, CANCELLED, REJECTED, etc.) instead of returning after the initial ACK.

## Startup sequence

` + "```sh" + `
qfex daemon start          # idempotent
qfex daemon status         # verify mds_connected: true
qfex market symbols        # list tradeable symbols (no auth)
qfex market bbo AAPL-USD   # current best bid/offer
` + "```" + `

## Trading (requires credentials)

` + "```sh" + `
qfex login                 # set API keys interactively
qfex daemon restart        # apply credentials
qfex daemon status         # verify trade_authed: true

qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1 --wait
qfex account balance
qfex position list
` + "```" + `

## Environment

` + "```sh" + `
qfex env           # show current environment (prod/uat)
qfex env uat       # switch to UAT (qfex.io) — no real funds
qfex env prod      # switch to production (qfex.com)
` + "```" + `

## Useful patterns

` + "```sh" + `
# Extract mid price
qfex market bbo AAPL-USD | jq '(.bid[0][0] | tonumber + (.ask[0][0] | tonumber)) / 2'

# Watch for fills
qfex watch fills | jq '{symbol: .symbol, price: .price, qty: .quantity, side: .side}'

# Place and confirm fill
RESULT=$(qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1 --wait)
echo $RESULT | jq '{status: .status, filled: (.quantity - .quantity_remaining)}'

# List all open orders
qfex order list | jq '.[] | {id: .order_id, symbol: .symbol, side: .side, price: .price}'
` + "```" + `

## Error handling

Exit code is non-zero on failure. Error message is on stderr.

` + "```sh" + `
if ! qfex order place --symbol AAPL-USD --side BUY --type LIMIT --tif GTC --qty 1 --price 200; then
  echo "Order failed" >&2
  exit 1
fi
` + "```" + `
`

var claudeCmd = &cobra.Command{
	Use:   "agents",
	Short: "AI agent integration helpers",
	Long:  `Helpers for integrating qfex with AI coding agents (Claude Code, Codex).`,
}

var claudeInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create agent context files and configure tool permissions",
	Long: `Configures AI coding agents to use qfex without permission prompts.

By default (global):
  - Writes ~/.claude/CLAUDE.md (Claude Code loads this in every session)
  - Adds Bash(qfex*) to ~/.claude/settings.json
  - Adds qfex to ~/.codex/config.toml allowed_programs

With --local: writes CLAUDE.md and AGENTS.md to the current directory instead.
Note: AGENTS.md is only written locally — Codex discovers it by directory traversal,
not from a global path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		local, _ := cmd.Flags().GetBool("local")
		if local {
			return agentInitLocal()
		}
		return agentInitGlobal()
	},
}

func writeFileIfAbsent(path, content string) (wrote bool, err error) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("%s already exists — skipping\n", path)
		return false, nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("Created %s\n", path)
	return true, nil
}

func agentInitLocal() error {
	_, err1 := writeFileIfAbsent("CLAUDE.md", agentMDContent)
	_, err2 := writeFileIfAbsent("AGENTS.md", agentMDContent)
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	fmt.Println("Claude Code and Codex will load this context automatically when working in this directory.")
	return nil
}

func agentInitGlobal() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding home directory: %w", err)
	}

	if err := setupClaudeGlobal(home); err != nil {
		return err
	}
	if err := setupCodexGlobal(home); err != nil {
		return err
	}

	// AGENTS.md is written locally — Codex discovers it by directory traversal.
	if _, err := writeFileIfAbsent("AGENTS.md", agentMDContent); err != nil {
		return err
	}
	return nil
}

func setupClaudeGlobal(home string) error {
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	if _, err := writeFileIfAbsent(filepath.Join(dir, "CLAUDE.md"), agentMDContent); err != nil {
		return err
	}

	// Add Bash(qfex*) to allowedTools in ~/.claude/settings.json
	settingsPath := filepath.Join(dir, "settings.json")
	settings := map[string]any{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", settingsPath, err)
		}
	}

	const tool = "Bash(qfex*)"
	existing, _ := settings["allowedTools"].([]any)
	for _, t := range existing {
		if t == tool {
			fmt.Printf("%s already in allowedTools in %s — skipping\n", tool, settingsPath)
			return nil
		}
	}
	settings["allowedTools"] = append(existing, tool)
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", settingsPath, err)
	}
	fmt.Printf("Added %q to allowedTools in %s\n", tool, settingsPath)
	return nil
}

func setupCodexGlobal(home string) error {
	dir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	// Add qfex to allowed_programs in ~/.codex/config.toml
	configPath := filepath.Join(dir, "config.toml")
	existing, _ := os.ReadFile(configPath)
	if bytes.Contains(existing, []byte("qfex")) {
		fmt.Printf("qfex already present in %s — skipping\n", configPath)
		return nil
	}

	addition := "\n[sandbox]\nallowed_programs = [\"qfex\"]\n"
	// If a [sandbox] section already exists, append just the line instead.
	if strings.Contains(string(existing), "[sandbox]") {
		addition = "allowed_programs = [\"qfex\"]\n"
	}

	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", configPath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(addition); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}
	fmt.Printf("Added qfex to allowed_programs in %s\n", configPath)
	return nil
}

func init() {
	rootCmd.AddCommand(claudeCmd)
	claudeCmd.AddCommand(claudeInitCmd)
	claudeInitCmd.Flags().Bool("local", false, "Write CLAUDE.md and AGENTS.md to the current directory instead")
}
