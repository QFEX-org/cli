# qfex CLI

[![CI](https://github.com/QFEX-org/cli/actions/workflows/ci.yml/badge.svg)](https://github.com/QFEX-org/cli/actions/workflows/ci.yml)
[![Release](https://github.com/QFEX-org/cli/actions/workflows/release.yml/badge.svg)](https://github.com/QFEX-org/cli/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/qfex/cli.svg)](https://pkg.go.dev/github.com/qfex/cli)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE.md)

A command-line interface for the [QFEX](https://qfex.com) perpetual futures exchange. Built for both humans and AI agents.

All output is JSON. Every command is stateless from the caller's perspective — a background daemon manages WebSocket connections.

**Docs:** [CLI reference](https://docs.qfex.com/api-reference/cli) · [API key setup](https://docs.qfex.com/api-reference/introduction)

---

## Installation

```sh
brew install QFEX-org/tap/qfex
```

---

## AI Agent Setup

Run once to configure everything automatically:

```sh
# Global (default): configures ~/.claude/ and ~/.codex/ so qfex does not repeatedly prompt for permission
qfex agents init

# Local: writes CLAUDE.md + AGENTS.md to the current project directory
qfex agents init --local
```

`qfex agents init` writes `~/.claude/CLAUDE.md` (loaded by Claude Code in every session), adds `Bash(qfex*)` to `~/.claude/settings.json`, and adds `qfex` to `~/.codex/config.toml`. Use `--local` to write `CLAUDE.md` and `AGENTS.md` to the current directory instead — useful for project-specific context or for Codex, which discovers `AGENTS.md` by directory traversal rather than a global path.

To configure manually:

**Claude Code** — add to `~/.claude/settings.json`:
```json
{
  "allowedTools": ["Bash(qfex*)"]
}
```

**Codex** — add to `~/.codex/config.toml`:
```toml
[sandbox]
allowed_programs = ["qfex"]
```

---

## Quick Start

No login required for market data:

```sh
# Start the daemon
qfex daemon start

# View market data — no credentials needed
qfex market symbols
qfex market bbo AAPL-USD
qfex market orderbook AAPL-USD
qfex market trades AAPL-USD
```

To place orders or view account data, log in first:

```sh
qfex login
qfex daemon restart   # applies credentials

qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1
qfex account balance
```

To generate API keys: sign in at [qfex.com](https://qfex.com), navigate to Developer Settings, and click "Generate public and secret API Keys". Full instructions at [docs.qfex.com/api-reference/introduction](https://docs.qfex.com/api-reference/introduction).

---

## How It Works

```
┌─────────────────────────────────────────────────────────┐
│                    qfex CLI (client)                    │
│  Any command → Unix socket → Daemon → QFEX WebSockets   │
└─────────────────────────────────────────────────────────┘
                           │
              ~/.local/share/qfex/daemon.sock
                           │
┌─────────────────────────────────────────────────────────┐
│                    qfex daemon                          │
│                                                         │
│  MDS WebSocket (wss://mds.qfex.com/)                   │
│    → order books, BBO, trades, candles, funding rates   │
│                                                         │
│  Trade WebSocket (wss://trade.qfex.com/)               │
│    → orders, positions, fills, balance (requires auth)  │
│                                                         │
│  In-memory cache → instant responses to CLI queries     │
└─────────────────────────────────────────────────────────┘
```

The daemon maintains persistent WebSocket connections and caches state. The CLI sends a JSON request over a Unix socket and prints the JSON response. No WebSocket code in your scripts.

---

## Configuration

Run `qfex login` to set credentials interactively. This saves to `~/.config/qfex/config.yaml`.

```yaml
public_key: qfex_pub_xxxxx     # Required for trading
secret_key: qfex_secret_xxxxx  # Required for trading
env: prod                      # "prod" (default) or "uat"
```

Market data commands work without credentials. Trading commands (orders, positions, balance) require credentials.

### Environments

QFEX runs two environments:

- **Production** — [qfex.com](https://qfex.com). Real funds.
- **UAT** — [qfex.io](https://qfex.io). Identical API and behaviour, separate exchange instance with no real funds. Use this for testing strategies, integrations, and order flow before going live.

The CLI connects to production by default. To use UAT, select it during `qfex login` or set `env: uat` in the config file.

| `env` | Trade WebSocket | MDS WebSocket |
|-------|----------------|---------------|
| `prod` (default) | `wss://trade.qfex.com/` | `wss://mds.qfex.com/` |
| `uat` | `wss://trade.qfex.io/` | `wss://mds.qfex.io/` |

To switch environments, run `qfex login` again or edit the config file and restart the daemon:

```sh
qfex daemon restart
qfex daemon status
# "env": "uat", "trade_url": "wss://trade.qfex.io/"
```

**Paths used:**

| Path | Purpose |
|------|---------|
| `~/.config/qfex/config.yaml` | API credentials and environment |
| `~/.local/share/qfex/daemon.sock` | IPC socket |
| `~/.local/share/qfex/daemon.pid` | Daemon PID |
| `~/.local/share/qfex/daemon.log` | Daemon logs |

The paths follow XDG conventions. Set `$XDG_CONFIG_HOME`, `$XDG_DATA_HOME`, or `$XDG_RUNTIME_DIR` to override.

---

## Commands

### Daemon

```sh
qfex daemon start     # Start daemon in background
qfex daemon stop      # Stop daemon
qfex daemon status    # Check if running and connected
qfex daemon restart   # Stop then start
```

**Status output:**
```json
{
  "running": true,
  "env": "prod",
  "mds_url": "wss://mds.qfex.com/",
  "trade_url": "wss://trade.qfex.com/",
  "mds_connected": true,
  "trade_authed": true
}
```

---

### Market Data

No credentials required for any of these.

```sh
# Best bid and offer
qfex market bbo AAPL-USD

# Order book (optional depth limit)
qfex market orderbook AAPL-USD
qfex market orderbook AAPL-USD --depth 5

# Recent public trades
qfex market trades AAPL-USD
qfex market trades AAPL-USD --limit 50

# Candles (live, latest candle per interval)
qfex market candles AAPL-USD --interval 1MIN

# Derivatives data
qfex market mark-price AAPL-USD
qfex market funding-rate AAPL-USD
qfex market open-interest AAPL-USD

# REST endpoints (no daemon required)
qfex market refdata                         # All symbol reference data
qfex market refdata --ticker AAPL-USD
qfex market metrics                         # Mark price, volume, OI for all symbols
qfex market candles-history AAPL-USD --resolution 1MIN --from 2024-01-01T00:00:00Z --to 2024-01-02T00:00:00Z
qfex market funding-history AAPL-USD --interval 60 --from 2024-01-01T00:00:00Z --to 2024-01-02T00:00:00Z
qfex market oi-history AAPL-USD --interval 60 --from 2024-01-01T00:00:00Z --to 2024-01-02T00:00:00Z
qfex market long-short AAPL-USD --interval 1h --from 2024-01-01T00:00:00Z --to 2024-01-02T00:00:00Z
qfex market taker-volume AAPL-USD --interval 60 --from 2024-01-01T00:00:00Z --to 2024-01-02T00:00:00Z
qfex market underlier AAPL-USD --interval 1h --from 2024-01-01T00:00:00Z --to 2024-01-02T00:00:00Z
qfex market settlement-calendar
qfex market settlement-prices
```

**Example BBO output:**
```json
{
  "symbol": "AAPL-USD",
  "time": "2026-03-17T20:42:15.719Z",
  "bid": [["254.04", "7.877"]],
  "ask": [["254.16", "7.873"]]
}
```

---

### Orders

Requires credentials.

```sh
# Place a limit order
qfex order place --symbol AAPL-USD --side BUY --type LIMIT --tif GTC --qty 1 --price 200

# Place a market order
qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1

# Place with take profit and stop loss
qfex order place --symbol AAPL-USD --side BUY --type LIMIT --tif GTC --qty 1 --price 200 --tp 230 --sl 180

# Place with a client-assigned ID (useful for tracking)
qfex order place --symbol AAPL-USD --side BUY --type LIMIT --tif GTC --qty 1 --price 200 --client-order-id my-order-001

# List active orders
qfex order list
qfex order list --symbol AAPL-USD

# Get a specific order
qfex order get --symbol AAPL-USD --order-id 5b309929-206f-40ec-804d-cbe46e81afc1

# Cancel an order
qfex order cancel --symbol AAPL-USD --order-id 5b309929-206f-40ec-804d-cbe46e81afc1
qfex order cancel --symbol AAPL-USD --order-id my-order-001 --id-type client_order_id

# Cancel all orders
qfex order cancel-all

# Modify an order
qfex order modify --symbol AAPL-USD --order-id 5b309929-... --side BUY --type LIMIT --price 205 --qty 2
```

**Order place/response fields:**

| Field | Values |
|-------|--------|
| `--side` | `BUY`, `SELL` |
| `--type` | `LIMIT`, `MARKET`, `ALO` |
| `--tif` | `GTC`, `IOC`, `FOK` |
| `--id-type` | `order_id`, `client_order_id`, `twap_id`, `client_twap_id` |

#### Order response lifecycle

Every order command returns JSON describing the outcome. Understanding the response flow matters when scripting or building agents.

**Placing an order — two-phase response:**

The exchange always sends an `ACK` first to confirm receipt, then may immediately send a second terminal response. By default `order place` returns after the ACK. Use `--wait` to block until the terminal response arrives.

```
add_order sent
    │
    ▼
ACK  ← returned immediately (default)
    │
    ▼ (may follow within milliseconds)
FILLED / CANCELLED / REJECTED / ...  ← returned with --wait
```

```sh
# Default: returns ACK immediately, order may still be open
qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1

# --wait: blocks until FILLED, CANCELLED, REJECTED, etc.
qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1 --wait
```

When to use `--wait`:

| Order type | Without `--wait` | With `--wait` |
|-----------|-----------------|--------------|
| `MARKET` | Returns ACK; fill happens async | Returns `FILLED` or `REJECTED` |
| `IOC` | Returns ACK; fill/cancel happens async | Returns `FILLED` (partial or full) or `CANCELLED` (no fill at all) |
| `FOK` | Returns ACK; fill or full cancel happens async | Returns `FILLED` (full only) or `CANCELLED` |
| `LIMIT GTC` | Returns ACK; order rests on book | Returns ACK again after 30s timeout (order is live) |

For `LIMIT GTC` with `--wait`, the command waits up to 30 seconds for a fill or cancellation. If neither arrives, it returns the original ACK — the order is still live on the book.

**Important — `FILLED` does not mean fully filled.** A partial fill also returns `FILLED`. Always check `quantity_remaining` to determine how much was actually executed:

- `quantity_remaining == 0` → fully filled
- `quantity_remaining > 0` → partially filled (the unfilled portion was cancelled for IOC, or the order is still open for GTC)

For IOC orders specifically, the sequence is: fill what's available → return `FILLED` with `quantity_remaining > 0` if partial, or `CANCELLED` if nothing was filled at all.

**Cancelling an order — single terminal response:**

```
cancel_order sent
    │
    ▼
CANCELLED  or  NO_SUCH_ORDER
```

**Modifying an order — single terminal response:**

```
modify_order sent
    │
    ▼
MODIFIED  or  CANNOT_MODIFY_NO_SUCH_ORDER  or  CANNOT_MODIFY_PARTIAL_FILL
```

**All possible order statuses:**

| Status | Meaning |
|--------|---------|
| `ACK` | Order received and live on the book |
| `FILLED` | Order filled — partially or fully. Check `quantity_remaining` (`0` = full fill, `> 0` = partial fill) |
| `MODIFIED` | Order successfully modified |
| `CANCELLED` | Order cancelled (by user, IOC/FOK expiry, or STP) |
| `CANCELLED_STP` | Cancelled by Self-Trade Prevention |
| `REJECTED` | Generic rejection |
| `NO_SUCH_ORDER` | Order ID not found |
| `CANNOT_MODIFY_NO_SUCH_ORDER` | Modify target not found |
| `CANNOT_MODIFY_PARTIAL_FILL` | Cannot modify a partially filled order |
| `FAILED_MARGIN_CHECK` | Insufficient margin |
| `PRICE_LESS_THAN_MIN_PRICE` | Price below allowed minimum |
| `PRICE_GREATER_THAN_MAX_PRICE` | Price above allowed maximum |
| `QUANTITY_LESS_THAN_MIN_QUANTITY` | Quantity too small |
| `QUANTITY_GREATER_THAN_MAX_QUANTITY` | Quantity too large |
| `REJECTED_WOULD_BREACH_MAX_NOTIONAL` | Order exceeds max notional |
| `REJECTED_MARKET_CLOSED` | Market is not currently open |
| `REJECTED_TOO_MANY_OPEN_ORDERS` | Open order limit reached |
| `REJECTED_OPEN_INTEREST_LIMIT` | Open interest limit reached |
| `INVALID_TAKEPROFIT_PRICE` | Invalid take profit price |
| `INVALID_STOPLOSS_PRICE` | Invalid stop loss price |
| `INVALID_TIME_IN_FORCE` | Invalid TIF for this order type |
| `RATE_LIMITED` | Too many requests |

---

### Positions

Requires credentials.

```sh
# List all open positions
qfex position list

# Close a position
qfex position close --symbol AAPL-USD
qfex position close --symbol AAPL-USD --client-order-id my-close-001
```

---

### Account

Requires credentials.

```sh
# Account balance
qfex account balance

# Fee tiers
qfex account fees

# Hourly PnL
qfex account pnl
qfex account pnl --symbol AAPL-USD --limit-hours 48

# Deposit address (USDC on Arbitrum)
qfex account deposit

# Leverage
qfex account leverage get
qfex account leverage available
qfex account leverage set --symbol AAPL-USD --leverage 10

# Enable cancel-on-disconnect (orders cancelled if connection drops)
qfex account cod --enable
qfex account cod --enable=false
```

#### Depositing funds

QFEX accepts USDC on the Arbitrum network. To fund your account:

1. Get your deposit address:
   ```sh
   qfex account deposit
   ```
   ```json
   { "address": "0x9140391891450b139272d1906b0e89dee7016f03" }
   ```
2. Send USDC (Arbitrum) to the returned `address`.

#### Withdrawals

Withdrawals require two-factor authentication (2FA) and cannot be initiated from the CLI. Use the QFEX web app to withdraw funds.

---

### Historic Data

Requires credentials. No daemon required.

```sh
# Filled/closed orders
qfex history orders
qfex history orders --symbol AAPL-USD --limit 50

# Historic TWAP orders
qfex history twaps
qfex history twaps --symbol AAPL-USD

# Trade history
qfex history trades
qfex history trades --symbol AAPL-USD --start 2024-01-01T00:00:00Z

# CSV exports
qfex history orders-csv > orders.csv
qfex history twaps-csv > twaps.csv
qfex history trades-csv > trades.csv
```

---

### TWAP Orders

```sh
# Buy 100 units of AAPL-USD, split into 10 orders, one every 30 seconds
qfex twap add --symbol AAPL-USD --side BUY --qty 100 --num-orders 10 --interval 30

# Reduce-only (only reduces an existing position)
qfex twap add --symbol AAPL-USD --side SELL --qty 50 --num-orders 5 --interval 60 --reduce-only

# With a client ID for tracking
qfex twap add --symbol AAPL-USD --side BUY --qty 100 --num-orders 10 --interval 30 --client-twap-id rebalance-001
```

---

### Stop Orders

```sh
# Cancel a stop order
qfex stop cancel --stop-order-id <id>

# Modify a stop order
qfex stop modify --stop-order-id <id> --symbol AAPL-USD --price 190 --qty 1
```

---

### Fills & Trade History

```sh
# Recent fills (daemon caches last 200)
qfex fills list
qfex fills list --limit 20

# User trade history (queries the exchange)
qfex trades list
qfex trades list --limit 50
qfex trades list --order-id <order-id>
```

---

### Live Streaming (watch)

Streams JSON events to stdout, one per line. Press Ctrl+C to stop.

```sh
# Market data streams
qfex watch bbo AAPL-USD
qfex watch orderbook AAPL-USD
qfex watch trades AAPL-USD
qfex watch mark-price AAPL-USD
qfex watch funding-rate AAPL-USD

# Candles (all intervals, or a specific one)
qfex watch candles AAPL-USD
qfex watch candles AAPL-USD --interval 1MIN

# Account streams (requires credentials)
qfex watch positions
qfex watch balance
qfex watch fills
qfex watch orders
```

Each line is a JSON object. Pipe to `jq` for filtering:

```sh
qfex watch bbo AAPL-USD | jq '.bid[0][0]'
```

---


---

## For AI Agents

The daemon must be running before any other command works. A minimal agent workflow:

```sh
# 1. Start daemon (idempotent — safe to call even if already running)
qfex daemon start

# 2. Check it's ready (trade_authed only required for trading)
qfex daemon status
# Market data only: {"mds_connected": true, "running": true}
# With credentials: {"mds_connected": true, "running": true, "trade_authed": true}

# 3. Issue commands — all output is JSON
# Market data needs no credentials:
qfex market bbo AAPL-USD
# Trading requires credentials (qfex login + daemon restart):
qfex order place --symbol AAPL-USD --side BUY --type LIMIT --tif GTC --qty 1 --price 200

# 4. Parse responses with jq or your language's JSON library
qfex market bbo AAPL-USD | jq '{bid: .bid[0][0], ask: .ask[0][0]}'

# 5. Stream data (runs until you kill it)
qfex watch bbo AAPL-USD &
WATCH_PID=$!
# ... do something with the stream ...
kill $WATCH_PID
```

**Placing orders from an agent:**

Use `--wait` when you need to know the result before proceeding:

```sh
RESULT=$(qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1 --wait)
STATUS=$(echo "$RESULT" | jq -r '.status')
REMAINING=$(echo "$RESULT" | jq -r '.quantity_remaining')

if [ "$STATUS" = "FILLED" ] && [ "$REMAINING" = "0" ]; then
  echo "Order fully filled"
elif [ "$STATUS" = "FILLED" ]; then
  echo "Order partially filled, remaining: $REMAINING"
elif [ "$STATUS" = "CANCELLED" ]; then
  echo "Order not filled at all (IOC — no liquidity)"
else
  echo "Order rejected: $STATUS"
fi
```

**Error handling:** When a command fails, the exit code is non-zero and stderr contains a human-readable error.

**IPC protocol:** Connect directly to the Unix socket and send newline-delimited JSON:

```json
{"cmd": "get_bbo", "params": {"symbol": "AAPL-USD"}}
```

Response:
```json
{"ok": true, "data": {"symbol": "AAPL-USD", "bid": [...], "ask": [...]}}
```

---

## Autocomplete

To enable autocomplete, add the following to your `.bashrc` or `.bash_profile`:

```bash
# you can also generate completions for zsh and fish shells by replacing `bash` with `zsh` or `fish`
source <(qfex completion bash)
```
---

## Troubleshooting

**Daemon won't start:**
```sh
cat ~/.local/share/qfex/daemon.log
```

**Not authenticated (trading commands fail):**
- Run `qfex login` to set credentials, then `qfex daemon restart`
- Run `qfex daemon status` — `trade_authed` should be `true`

**Stale socket (daemon crashed):**
```sh
rm ~/.local/share/qfex/daemon.sock
qfex daemon start
```

**No data for a symbol:**
The daemon subscribes on first request. Re-run the command — data arrives within a few hundred milliseconds once subscribed.

---

## Developer

### Building from source

```sh
git clone https://github.com/QFEX-org/cli
cd cli
go build -o qfex .
```

### Running tests

```sh
go test ./...
```

### Cutting a release

See [RELEASING.md](RELEASING.md).
