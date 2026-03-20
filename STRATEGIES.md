# Trading Strategies

Example shell strategies using the qfex CLI. All examples assume you are logged in (`qfex login`) and the daemon is running.

> **Risk notice:** These are illustrative examples only. Perpetual futures trading carries significant risk of loss. Always test on UAT first (`qfex env uat`) before running anything on production.

---

## 1. Bracket Order

Enter at a limit price with take profit and stop loss attached in a single command. The exchange manages the exit — no polling loop required on your end.

```sh
qfex order place \
  --symbol AAPL-USD \
  --side BUY \
  --type LIMIT \
  --tif GTC \
  --qty 5 \
  --price 250.00 \
  --tp 262.50 \
  --sl 243.75
```

If you want to confirm the entry filled before continuing in a script, use `--wait`:

```sh
RESULT=$(qfex order place \
  --symbol AAPL-USD \
  --side BUY \
  --type LIMIT \
  --tif GTC \
  --qty 5 \
  --price 250.00 \
  --tp 262.50 \
  --sl 243.75 \
  --wait)

STATUS=$(echo "$RESULT" | jq -r '.status')

case "$STATUS" in
  FILLED)
    echo "Entered. TP at 262.50, SL at 243.75."
    ;;
  ACK)
    echo "Order is resting on the book — waiting for fill."
    ;;
  *)
    echo "Unexpected status: $STATUS"
    ;;
esac
```

---

## 2. Dollar-Cost Averaging (DCA)

Buy a fixed quantity at regular intervals regardless of price. Spreads entry cost over time rather than trying to time a single entry point.

```sh
SYMBOL="AAPL-USD"
QTY=1           # units per buy
ORDERS=10       # total number of buys
INTERVAL=300    # seconds between buys (5 min)

for i in $(seq 1 $ORDERS); do
  echo "Buy $i / $ORDERS..."

  RESULT=$(qfex order place \
    --symbol "$SYMBOL" \
    --side BUY \
    --type MARKET \
    --tif IOC \
    --qty $QTY \
    --wait)

  STATUS=$(echo "$RESULT" | jq -r '.status')
  FILLED=$(echo "$RESULT" | jq -r '.quantity_remaining | tonumber | . == 0')
  echo "  $STATUS  fully_filled=$FILLED"

  if [ $i -lt $ORDERS ]; then
    sleep $INTERVAL
  fi
done

echo "DCA complete. Current position:"
qfex position list | jq '.[] | select(.symbol == "'"$SYMBOL"'")'
```

---

## 3. TWAP Execution

Split a large order into equal-sized child orders sent at fixed intervals. Reduces market impact compared to hitting the book all at once. The daemon manages the schedule — you don't need to keep a loop running.

```sh
# Buy 100 units over ~5 minutes: 10 orders, one every 30 seconds
qfex twap add \
  --symbol AAPL-USD \
  --side BUY \
  --qty 100 \
  --num-orders 10 \
  --interval 30
```

Monitor fills as they arrive:

```sh
qfex watch fills | jq 'select(.symbol == "AAPL-USD") | {price, quantity, side}'
```

Exit the resulting position with a TWAP sell using `--reduce-only` so it can never flip you short:

```sh
QTY=$(qfex position list \
  | jq -r '.[] | select(.symbol == "AAPL-USD") | .quantity | ltrimstr("-")')

qfex twap add \
  --symbol AAPL-USD \
  --side SELL \
  --qty "$QTY" \
  --num-orders 10 \
  --interval 30 \
  --reduce-only
```

---

## 4. Funding Rate Fade

On perpetuals the funding rate reflects the imbalance between longs and shorts. A persistently high positive rate means longs are paying shorts heavily — a sign the market may be stretched. Going short to collect funding, and vice versa, is a common perp strategy.

```sh
SYMBOL="AAPL-USD"
THRESHOLD="0.0005"   # 0.05% — adjust to taste
QTY=5

FUNDING=$(qfex market funding-rate "$SYMBOL" | jq -r '.funding_rate')
echo "Funding rate: $FUNDING"

ABOVE=$(awk "BEGIN { print ($FUNDING >  $THRESHOLD) ? 1 : 0 }")
BELOW=$(awk "BEGIN { print ($FUNDING < -$THRESHOLD) ? 1 : 0 }")

if [ "$ABOVE" = "1" ]; then
  echo "Rate strongly positive — shorting to collect funding."
  qfex order place \
    --symbol "$SYMBOL" \
    --side SELL \
    --type MARKET \
    --tif IOC \
    --qty $QTY \
    --wait | jq '{status, price}'

elif [ "$BELOW" = "1" ]; then
  echo "Rate strongly negative — longing to collect funding."
  qfex order place \
    --symbol "$SYMBOL" \
    --side BUY \
    --type MARKET \
    --tif IOC \
    --qty $QTY \
    --wait | jq '{status, price}'

else
  echo "Rate within normal range. No trade."
fi
```

Schedule this around the funding settlement window (typically every 8 hours) and always pair it with a stop loss or a maximum hold period.

---

## 5. Flatten All Positions

Cancel every open order and close every open position in one pass. Useful as an emergency exit or end-of-session cleanup.

```sh
echo "Cancelling all open orders..."
qfex order cancel-all

echo "Closing all open positions..."
qfex position list | jq -r '.[].symbol' | while read -r SYMBOL; do
  echo "  Closing $SYMBOL..."
  qfex position close --symbol "$SYMBOL"
done

echo "Done. Remaining positions:"
qfex position list
```

To watch the closing fills arrive in real time, run this in a separate terminal before executing the script:

```sh
qfex watch fills | jq '{symbol: .symbol, side: .side, price: .price, qty: .quantity}'
```

---

## 6. Simple Quoting (Market Making)

Place a bid and an ask symmetrically around the mid price, then cancel and replace them as the market moves. The loop earns the spread when both sides fill.

```sh
SYMBOL="GOLD-USD"
QTY=1
SPREAD=0.20    # total spread in dollars (0.10 each side of mid)
INTERVAL=10    # seconds between requotes

BID_ID=""
ASK_ID=""

cleanup() {
  echo "Shutting down — cancelling open quotes..."
  [ -n "$BID_ID" ] && qfex order cancel --symbol "$SYMBOL" --order-id "$BID_ID" 2>/dev/null
  [ -n "$ASK_ID" ] && qfex order cancel --symbol "$SYMBOL" --order-id "$ASK_ID" 2>/dev/null
  exit 0
}
trap cleanup INT TERM

while true; do
  BBO=$(qfex market bbo "$SYMBOL")
  MID=$(echo "$BBO" | jq '((.bid[0][0] | tonumber) + (.ask[0][0] | tonumber)) / 2')

  BID_PRICE=$(awk "BEGIN { printf \"%.2f\", $MID - $SPREAD/2 }")
  ASK_PRICE=$(awk "BEGIN { printf \"%.2f\", $MID + $SPREAD/2 }")
  echo "Mid: $MID  Quoting $BID_PRICE / $ASK_PRICE"

  # Cancel previous quotes (ignore errors if already filled)
  [ -n "$BID_ID" ] && qfex order cancel --symbol "$SYMBOL" --order-id "$BID_ID" 2>/dev/null
  [ -n "$ASK_ID" ] && qfex order cancel --symbol "$SYMBOL" --order-id "$ASK_ID" 2>/dev/null

  # Place new quotes
  BID_ID=$(qfex order place \
    --symbol "$SYMBOL" --side BUY --type LIMIT --tif GTC \
    --qty $QTY --price "$BID_PRICE" | jq -r '.order_id')

  ASK_ID=$(qfex order place \
    --symbol "$SYMBOL" --side SELL --type LIMIT --tif GTC \
    --qty $QTY --price "$ASK_PRICE" | jq -r '.order_id')

  sleep $INTERVAL
done
```

Press Ctrl+C to cancel both quotes and exit cleanly.

> **Note:** This loop has no inventory management. In practice you would track your net position and skew or pause quotes on one side once inventory builds up in one direction.
