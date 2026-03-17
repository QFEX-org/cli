# qfex CLI

A command-line interface for the [QFEX](https://qfex.com) perpetual futures exchange. Built for both humans and AI agents.

All output is JSON. Every command is stateless from the caller's perspective — a background daemon manages WebSocket connections.

---

## Quick Start

```sh
# 1. Build
go build -o qfex .

# 2. Configure credentials (optional for market data, required for trading)
mkdir -p ~/.config/qfex
cat > ~/.config/qfex/config.yaml <<EOF
public_key: qfex_pub_xxxxx
secret_key: your_secret_key
EOF

# 3. Start the daemon
./qfex daemon start

# 4. Get market data
./qfex market bbo AAPL-USD

# 5. When done
./qfex daemon stop
```

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

**File:** `~/.config/qfex/config.yaml`

```yaml
public_key: qfex_pub_xxxxx   # Required for trading
secret_key: your_secret_key  # Required for trading
```

Market data commands work without credentials. Trading commands (orders, positions, balance) require credentials.

**Paths used:**

| Path | Purpose |
|------|---------|
| `~/.config/qfex/config.yaml` | API credentials |
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
{"mds_connected": true, "running": true, "trade_authed": true}
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

# Candles (intervals: 1MIN, 5MINS, 15MINS, 1HOUR, 4HOURS, 1DAY)
qfex market candles AAPL-USD --interval 1MIN

# Derivatives data
qfex market mark-price AAPL-USD
qfex market funding-rate AAPL-USD
qfex market open-interest AAPL-USD
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

**Order response statuses:** `ACK`, `FILLED`, `MODIFIED`, `CANCELLED`, `REJECTED`, `RATE_LIMITED`, and others.

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

**Example position output:**
```json
[
  {
    "symbol": "AAPL-USD",
    "position": 5.0,
    "average_price": 198.50,
    "unrealised_pnl": 77.50,
    "leverage": 10,
    "initial_margin": 99.25
  }
]
```

---

### Account

Requires credentials.

```sh
# Account balance
qfex account balance

# View current leverage settings
qfex account leverage get

# View available leverage levels
qfex account leverage available

# Set leverage for a symbol
qfex account leverage set --symbol AAPL-USD --leverage 10

# Enable cancel-on-disconnect (orders cancelled if connection drops)
qfex account cod --enable
qfex account cod --enable=false
```

---

### TWAP Orders

Time-Weighted Average Price orders split a large order into many small market orders.

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

Requires credentials.

```sh
# Recent fills (daemon caches last 200)
qfex fills list
qfex fills list --limit 20

# User trade history (queries the exchange)
qfex trades list
qfex trades list --limit 50
qfex trades list --order-id <order-id>
qfex trades list --start-ts 1760000000 --end-ts 1760100000
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

## Available Symbols

Equities: `AAPL-USD`, `PLTR-USD`, `NVDA-USD`, `GOOGL-USD`, `TSLA-USD`, `MSFT-USD`, `META-USD`, `HOOD-USD`, `CRWV-USD`, `INTC-USD`, `CRCL-USD`, `SNDK-USD`, `RKLB-USD`

Commodities: `GOLD-USD`, `SILVER-USD`, `URANIUM-USD`, `CL-USD`, `NATGAS-USD`, `COPPER-USD`

Indices: `US100-USD`, `US500-USD`

Forex: `EUR-USD`, `GBP-USD`

---

## For AI Agents

The daemon must be running before any other command works. A minimal agent workflow:

```sh
# 1. Start daemon (idempotent — safe to call even if already running)
qfex daemon start

# 2. Check it's ready
qfex daemon status
# Expected: {"mds_connected": true, "running": true, "trade_authed": true}

# 3. Issue commands — all output is JSON
qfex market bbo AAPL-USD
qfex order place --symbol AAPL-USD --side BUY --type LIMIT --tif GTC --qty 1 --price 200

# 4. Parse responses with jq or your language's JSON library
qfex market bbo AAPL-USD | jq '{bid: .bid[0][0], ask: .ask[0][0]}'

# 5. Stream data (runs until you kill it)
qfex watch bbo AAPL-USD &
WATCH_PID=$!
# ... do something with the stream ...
kill $WATCH_PID
```

**Error handling:** When a command fails, the exit code is non-zero and stderr contains a human-readable error. Stdout still contains valid JSON with `{"ok": false, "error": "..."}` from the daemon layer, but the CLI will exit 1.

**IPC protocol:** If you want to speak to the daemon directly (bypassing the CLI), connect to the Unix socket and send newline-delimited JSON:

```json
{"cmd": "get_bbo", "params": {"symbol": "AAPL-USD"}}
```

Response:
```json
{"ok": true, "data": {"symbol": "AAPL-USD", "bid": [...], "ask": [...]}}
```

Available IPC commands match the CLI commands: `get_bbo`, `get_orderbook`, `get_trades`, `get_mark_price`, `get_funding_rate`, `get_open_interest`, `place_order`, `cancel_order`, `cancel_all`, `modify_order`, `get_order`, `get_orders`, `get_positions`, `close_position`, `get_balance`, `get_leverage`, `set_leverage`, `get_available_leverage`, `add_twap`, `cancel_stop_order`, `modify_stop_order`, `get_fills`, `get_user_trades`, `cancel_on_disconnect`, `watch`, `status`, `ping`.

---

## Troubleshooting

**Daemon won't start:**
```sh
cat ~/.local/share/qfex/daemon.log
```

**Not authenticated (trading commands fail):**
- Check `~/.config/qfex/config.yaml` has valid `public_key` and `secret_key`
- Run `qfex daemon status` — `trade_authed` should be `true`

**Stale socket (daemon crashed):**
```sh
rm ~/.local/share/qfex/daemon.sock
qfex daemon start
```

**No data for a symbol:**
The daemon subscribes on first request. Re-run the command — data arrives within a few hundred milliseconds once subscribed.
