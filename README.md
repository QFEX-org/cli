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
env: prod                    # "prod" (default) or "uat"
```

Market data commands work without credentials. Trading commands (orders, positions, balance) require credentials.

### Environments

| `env` | Trade WebSocket | MDS WebSocket |
|-------|----------------|---------------|
| `prod` (default) | `wss://trade.qfex.com/` | `wss://mds.qfex.com/` |
| `uat` | `wss://trade.qfex.io/` | `wss://mds.qfex.io/` |

UAT is identical to production in behaviour and API shape, but uses a separate exchange instance at `qfex.io`. Use it for testing order placement without risking real funds.

To switch environments, change `env` in the config file and restart the daemon:

```sh
# Switch to UAT
echo "env: uat" >> ~/.config/qfex/config.yaml
qfex daemon restart

# Confirm which environment is active
qfex daemon status
# "env": "uat", "trade_url": "wss://trade.qfex.io/"

# Switch back to prod
sed -i '' 's/env: uat/env: prod/' ~/.config/qfex/config.yaml
qfex daemon restart
```

You can also override the URLs directly if you need a custom endpoint (takes precedence over `env`):

```yaml
trade_ws_url: wss://trade.qfex.io/
mds_url: wss://mds.qfex.io/
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

Cancel always returns a single terminal response directly (no ACK phase):

```
cancel_order sent
    │
    ▼
CANCELLED  or  NO_SUCH_ORDER
```

```sh
qfex order cancel --symbol AAPL-USD --order-id <id>
```

**Modifying an order — single terminal response:**

```
modify_order sent
    │
    ▼
MODIFIED  or  CANNOT_MODIFY_NO_SUCH_ORDER  or  CANNOT_MODIFY_PARTIAL_FILL
```

```sh
qfex order modify --symbol AAPL-USD --order-id <id> --side BUY --type LIMIT --price 205
```

**All possible order statuses:**

| Status | Meaning |
|--------|---------|
| `ACK` | Order received and live on the book |
| `FILLED` | Order filled — partially or fully. Check `quantity_remaining` to know how much executed (`0` = full fill, `> 0` = partial fill) |
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

**Example responses:**

```json
// ACK (order placed, resting on book)
{"order_id": "5b309929-...", "status": "ACK", "symbol": "AAPL-USD", "side": "BUY", "type": "LIMIT", "quantity": 1, "price": 200, "quantity_remaining": 1}

// FILLED — full fill (quantity_remaining == 0)
{"order_id": "5b309929-...", "status": "FILLED", "symbol": "AAPL-USD", "side": "BUY", "quantity": 1, "price": 200, "quantity_remaining": 0}

// FILLED — partial fill (quantity_remaining > 0, unfilled portion was cancelled for IOC)
{"order_id": "5b309929-...", "status": "FILLED", "symbol": "AAPL-USD", "side": "BUY", "quantity": 1, "price": 200, "quantity_remaining": 0.4}

// CANCELLED (IOC/FOK — nothing was filled at all)
{"order_id": "5b309929-...", "status": "CANCELLED", "symbol": "AAPL-USD", "quantity_remaining": 1}

// REJECTED (e.g. margin failure)
{"order_id": "5b309929-...", "status": "FAILED_MARGIN_CHECK", "symbol": "AAPL-USD"}
```

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

# Deposit address (USDC on Arbitrum)
qfex account deposit

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

#### Depositing funds

QFEX accepts USDC on the Arbitrum network. To fund your account:

1. Get your deposit address:
   ```sh
   qfex account deposit
   ```
   ```json
   {
     "address": "0x9140391891450b139272d1906b0e89dee7016f03",
     "available_allowance": 39501032.95
   }
   ```

2. Send USDC (Arbitrum) to the returned `address`.

3. `available_allowance` is the remaining deposit capacity on your account in USD. Deposits above this limit will not be credited.

The deposit address is fetched from `https://banker.qfex.com/address` (or `https://banker.qfex.io/address` on UAT) using HMAC authentication.

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

**Placing orders from an agent:**

Without `--wait`, `order place` returns the ACK and exits. The order may not be filled yet. This is fine if you don't need to confirm execution, but makes it hard to know the outcome synchronously.

Use `--wait` when you need to know the result before proceeding:

```sh
# Place a market buy and wait for confirmation of fill or rejection
RESULT=$(qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1 --wait)
STATUS=$(echo "$RESULT" | jq -r '.status')
REMAINING=$(echo "$RESULT" | jq -r '.quantity_remaining')

if [ "$STATUS" = "FILLED" ] && [ "$REMAINING" = "0" ]; then
  echo "Order fully filled"
elif [ "$STATUS" = "FILLED" ]; then
  # Partial fill — FILLED is returned even when only part of the order executed.
  # quantity_remaining tells you how much was NOT filled.
  echo "Order partially filled, remaining: $REMAINING"
elif [ "$STATUS" = "CANCELLED" ]; then
  echo "Order not filled at all (IOC — no liquidity)"
else
  echo "Order rejected: $STATUS"
fi
```

For `LIMIT GTC` orders with `--wait`, the command blocks up to 30 seconds waiting for a fill or cancellation. If neither arrives (the order is sitting on the book), the original ACK is returned. Check `status == "ACK"` to detect this case.

```sh
RESULT=$(qfex order place --symbol AAPL-USD --side BUY --type LIMIT --tif GTC --qty 1 --price 150 --wait)
STATUS=$(echo "$RESULT" | jq -r '.status')

if [ "$STATUS" = "ACK" ]; then
  ORDER_ID=$(echo "$RESULT" | jq -r '.order_id')
  echo "Order resting on book: $ORDER_ID"
elif [ "$STATUS" = "FILLED" ]; then
  echo "Limit order filled immediately"
fi
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
