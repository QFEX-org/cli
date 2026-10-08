package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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

- **All output is JSON.** Pipe to ` + "`jq`" + ` for extraction.
- **The daemon starts automatically** when a command needs it — never run ` + "`qfex daemon start`" + ` manually.
- **` + "`--wait`" + ` on order place** blocks until the terminal status (FILLED, CANCELLED, REJECTED, etc.) instead of returning after the initial ACK.

## Sandboxed environments (Codex)

The daemon starts automatically and is the preferred way to access market data — it maintains persistent WebSocket connections for low-latency responses. If the daemon cannot start in a sandboxed environment, the following REST commands work without it as a fallback:

` + "```sh" + `
qfex market symbols                    # all tradeable symbols
qfex market refdata                    # reference data
qfex market metrics                    # mark price, volume, OI
qfex history orders                    # filled/closed order history
qfex history trades                    # trade history
qfex account fees                      # fee tiers
qfex account pnl                       # hourly PnL
qfex account deposit                   # deposit address
` + "```" + `

## Trading (requires credentials)

` + "```sh" + `
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
	Use:     "agents",
	Aliases: []string{"agent"},
	Short:   "AI agent integration helpers",
	Long:    `Helpers for integrating qfex with AI coding agents (Claude Code, Codex).`,
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

// Markers delimiting the qfex section inside an agent context file, so the
// section can be refreshed in place without touching anything the user wrote
// around it.
const (
	agentBlockBegin = "<!-- BEGIN qfex CLI -->"
	agentBlockEnd   = "<!-- END qfex CLI -->"
)

func agentBlock(content string) string {
	return agentBlockBegin + "\n" + content + "\n" + agentBlockEnd + "\n"
}

// writeAgentFile adds the qfex section to an agent context file, keeping
// whatever the file already holds. These are files users write themselves —
// ~/.claude/CLAUDE.md carries their global Claude Code instructions — so the
// section is appended or refreshed in place rather than replacing the file.
func writeAgentFile(path, content string) error {
	block := agentBlock(content)

	existing, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(block), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
		fmt.Printf("Created %s\n", path)
		return nil
	}

	updated, err := replaceAgentBlock(string(existing), block, content)
	if err != nil {
		return fmt.Errorf("updating %s: %w", path, err)
	}
	if updated == string(existing) {
		fmt.Printf("qfex context already present in %s — skipping\n", path)
		return nil
	}
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("Updated %s\n", path)
	return nil
}

// replaceAgentBlock returns existing with the qfex section refreshed: swapped
// in place when the markers are there, and appended otherwise. A file written
// by an earlier qfex version holds the bare content with no markers, so that
// section is matched too and wrapped, rather than left behind as a duplicate.
func replaceAgentBlock(existing, block, content string) (string, error) {
	begin := strings.Index(existing, agentBlockBegin)
	if begin != -1 {
		end := strings.Index(existing[begin:], agentBlockEnd)
		if end == -1 {
			return "", fmt.Errorf("found %s without a matching %s", agentBlockBegin, agentBlockEnd)
		}
		end += begin + len(agentBlockEnd)
		return existing[:begin] + block + dropLineBreak(existing[end:]), nil
	}

	if start, end := legacySection(existing, content); start != -1 {
		return existing[:start] + block + dropLineBreak(existing[end:]), nil
	}

	prefix := existing
	if prefix != "" && !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}
	if prefix != "" {
		prefix += "\n"
	}
	return prefix + block, nil
}

// dropLineBreak removes the newline that ended the replaced section's last
// line, which block supplies itself. Blank lines past it are the separation
// the user put before whatever comes next, so they are left alone.
func dropLineBreak(rest string) string {
	return strings.TrimPrefix(rest, "\n")
}

// legacySection locates a qfex section written before the markers existed: the
// run from content's own heading to the next top-level heading, or to the end
// of the file. Lines inside a fenced code block, like shell comments, are not
// headings. Matching the heading rather than the whole of content means a
// section written by a different release is still upgraded in place, instead
// of being appended beside the new one as a stale duplicate.
func legacySection(existing, content string) (start, end int) {
	heading, _, _ := strings.Cut(content, "\n")
	if !strings.HasPrefix(heading, "# ") {
		return -1, -1
	}
	start = lineIndex(existing, heading)
	if start == -1 {
		return -1, -1
	}
	end = len(existing)
	inFence := false
	for offset := start; offset < len(existing); {
		br := strings.IndexByte(existing[offset:], '\n')
		if br == -1 {
			break
		}
		next := offset + br + 1
		line := existing[next:]
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		} else if !inFence && strings.HasPrefix(line, "# ") {
			end = next
			break
		}
		offset = next
	}
	return start, len(strings.TrimRight(existing[:end], "\n"))
}

// lineIndex returns the offset of the first line of s equal to line, or -1.
func lineIndex(s, line string) int {
	for offset := 0; offset <= len(s); {
		if rest := s[offset:]; strings.HasPrefix(rest, line) &&
			(len(rest) == len(line) || rest[len(line)] == '\n') {
			return offset
		}
		br := strings.IndexByte(s[offset:], '\n')
		if br == -1 {
			return -1
		}
		offset += br + 1
	}
	return -1
}

func agentInitLocal() error {
	if err := writeAgentFile("CLAUDE.md", agentMDContent); err != nil {
		return err
	}
	if err := writeAgentFile("AGENTS.md", agentMDContent); err != nil {
		return err
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
	if err := writeAgentFile("AGENTS.md", agentMDContent); err != nil {
		return err
	}
	return nil
}

func setupClaudeGlobal(home string) error {
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	if err := writeAgentFile(filepath.Join(dir, "CLAUDE.md"), agentMDContent); err != nil {
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

	configPath := filepath.Join(dir, "config.toml")
	existing, _ := os.ReadFile(configPath)
	content := string(existing)
	updated := false

	if !bytes.Contains(existing, []byte("qfex")) {
		// network = "off" disables Codex's network sandbox so qfex can open
		// outbound WebSocket connections to the exchange.
		addition := "\n[sandbox]\nnetwork = \"off\"\nallowed_programs = [\"qfex\"]\n"
		// If a [sandbox] section already exists, append just the settings instead.
		if strings.Contains(content, "[sandbox]") {
			addition = "network = \"off\"\nallowed_programs = [\"qfex\"]\n"
		}
		content += addition
		updated = true
	}

	writableRoot := filepath.Join(home, ".local", "share", "qfex")
	workspaceSection := fmt.Sprintf("\n[sandbox_workspace_write]\nwritable_roots = [\"%s\"]\nnetwork_access = true\n", writableRoot)
	if !strings.Contains(content, "[sandbox_workspace_write]") && !strings.Contains(content, writableRoot) {
		content += workspaceSection
		updated = true
	}

	if !updated {
		fmt.Printf("Codex config already configured in %s — skipping\n", configPath)
		return nil
	}

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}
	fmt.Printf("Updated Codex config in %s\n", configPath)
	return nil
}

func init() {
	rootCmd.AddCommand(claudeCmd)
	claudeCmd.AddCommand(claudeInitCmd)
	claudeInitCmd.Flags().Bool("local", false, "Write CLAUDE.md and AGENTS.md to the current directory instead")
}
