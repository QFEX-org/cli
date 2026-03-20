package protocol

import "encoding/json"

// IPC command names (CLI → Daemon)
const (
	// Market data
	CmdGetBBO             = "get_bbo"
	CmdGetOrderBook       = "get_orderbook"
	CmdGetTrades          = "get_trades"
	CmdGetCandles         = "get_candles"
	CmdGetMarkPrice       = "get_mark_price"
	CmdGetFundingRate     = "get_funding_rate"
	CmdGetOpenInterest    = "get_open_interest"
	CmdGetUnderlierPrice  = "get_underlier_price"

	// Orders
	CmdPlaceOrder   = "place_order"
	CmdCancelOrder  = "cancel_order"
	CmdCancelAll    = "cancel_all"
	CmdModifyOrder  = "modify_order"
	CmdGetOrder     = "get_order"
	CmdGetOrders    = "get_orders"

	// Positions
	CmdGetPositions  = "get_positions"
	CmdClosePosition = "close_position"

	// Account
	CmdGetBalance           = "get_balance"
	CmdGetLeverage          = "get_leverage"
	CmdSetLeverage          = "set_leverage"
	CmdGetAvailableLeverage = "get_available_leverage"

	// TWAP
	CmdAddTwap = "add_twap"

	// Stop orders
	CmdCancelStopOrder = "cancel_stop_order"
	CmdModifyStopOrder = "modify_stop_order"

	// Fills & trades
	CmdGetFills      = "get_fills"
	CmdGetUserTrades = "get_user_trades"

	// Streaming
	CmdWatch = "watch"

	// Control
	CmdPing               = "ping"
	CmdStatus             = "status"
	CmdCancelOnDisconnect = "cancel_on_disconnect"
)

// Watch stream names
const (
	StreamBBO          = "bbo"
	StreamOrderBook    = "orderbook"
	StreamTrades       = "trades"
	StreamPositions    = "positions"
	StreamBalance      = "balance"
	StreamFills        = "fills"
	StreamOrders       = "orders"
	StreamMarkPrice    = "mark_price"
	StreamFundingRate  = "funding_rate"
	StreamOpenInterest = "open_interest"
	StreamCandles      = "candle"
	StreamUnderlier    = "underlier"
)

// Request is a message from CLI to daemon (newline-delimited JSON).
type Request struct {
	Cmd    string          `json:"cmd"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is a message from daemon to CLI for one-shot commands.
type Response struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Event is a streaming message from daemon to CLI.
type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// ---- Param structs ----

type GetBBOParams struct {
	Symbol string `json:"symbol"`
}

type GetOrderBookParams struct {
	Symbol string `json:"symbol"`
	Depth  int    `json:"depth,omitempty"`
}

type GetTradesParams struct {
	Symbol string `json:"symbol"`
	Limit  int    `json:"limit,omitempty"`
}

type GetCandlesParams struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Limit    int    `json:"limit,omitempty"`
}

type GetMarkPriceParams struct {
	Symbol string `json:"symbol"`
}

type GetFundingRateParams struct {
	Symbol string `json:"symbol"`
}

type GetOpenInterestParams struct {
	Symbol string `json:"symbol"`
}

type GetUnderlierPriceParams struct {
	Symbol string `json:"symbol"`
}

type PlaceOrderParams struct {
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	OrderType     string  `json:"order_type"`
	TimeInForce   string  `json:"order_time_in_force"`
	Quantity      float64 `json:"quantity"`
	Price         float64 `json:"price,omitempty"`
	ReduceOnly    bool    `json:"reduce_only,omitempty"`
	TakeProfit    float64 `json:"take_profit,omitempty"`
	StopLoss      float64 `json:"stop_loss,omitempty"`
	ClientOrderID string  `json:"client_order_id,omitempty"`
	// WaitForFinal waits for the terminal status after the initial ACK.
	WaitForFinal bool `json:"wait_for_final,omitempty"`
}

type CancelOrderParams struct {
	Symbol       string `json:"symbol"`
	OrderID      string `json:"order_id"`
	CancelIDType string `json:"cancel_order_id_type"`
}

type CancelAllParams struct{}

type ModifyOrderParams struct {
	Symbol     string  `json:"symbol"`
	OrderID    string  `json:"order_id"`
	Side       string  `json:"side"`
	OrderType  string  `json:"order_type"`
	Price      float64 `json:"price,omitempty"`
	Quantity   float64 `json:"quantity,omitempty"`
	TakeProfit float64 `json:"take_profit,omitempty"`
	StopLoss   float64 `json:"stop_loss,omitempty"`
}

type GetOrderParams struct {
	Symbol  string `json:"symbol"`
	OrderID string `json:"order_id"`
}

type GetOrdersParams struct {
	Symbol string `json:"symbol,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

type ClosePositionParams struct {
	Symbol        string `json:"symbol"`
	ClientOrderID string `json:"client_order_id,omitempty"`
}

type GetLeverageParams struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type SetLeverageParams struct {
	Symbol   string  `json:"symbol"`
	Leverage float64 `json:"leverage"`
}

type AddTwapParams struct {
	Symbol            string  `json:"symbol"`
	Side              string  `json:"side"`
	TotalQuantity     float64 `json:"total_quantity"`
	NumOrders         int     `json:"num_orders"`
	OrderIntervalSecs int     `json:"order_interval_secs"`
	ReduceOnly        bool    `json:"reduce_only"`
	ClientTwapID      string  `json:"client_twap_id,omitempty"`
}

type CancelStopOrderParams struct {
	StopOrderID string `json:"stop_order_id"`
}

type ModifyStopOrderParams struct {
	StopOrderID string  `json:"stop_order_id"`
	Symbol      string  `json:"symbol"`
	Price       float64 `json:"price"`
	Quantity    float64 `json:"quantity"`
}

type GetUserTradesParams struct {
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
	OrderID string `json:"order_id,omitempty"`
	StartTS int64  `json:"start_ts,omitempty"`
	EndTS   int64  `json:"end_ts,omitempty"`
}

type GetFillsParams struct {
	Limit  int    `json:"limit,omitempty"`
	Symbol string `json:"symbol,omitempty"`
}

type WatchParams struct {
	Stream   string `json:"stream"`
	Symbol   string `json:"symbol,omitempty"`
	Interval string `json:"interval,omitempty"`
}

type CancelOnDisconnectParams struct {
	Enable bool `json:"enable"`
}
