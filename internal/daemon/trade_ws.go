package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/qfex/cli/internal/auth"
	"github.com/qfex/cli/internal/build"
	"github.com/qfex/cli/internal/config"
	"github.com/qfex/cli/internal/oauth"
)

// tradeMessage represents any message from the Trade WebSocket.
// Direct responses use top-level keys; subscription updates use the envelope format.
type tradeMessage struct {
	// Envelope fields (subscription updates)
	Type         string          `json:"type,omitempty"`
	ConnectionID string          `json:"connection_id,omitempty"`
	MessageID    int64           `json:"message_id,omitempty"`
	Channel      string          `json:"channel,omitempty"`
	Contents     json.RawMessage `json:"contents,omitempty"`

	// Direct response keys
	Authenticated              *bool           `json:"authenticated,omitempty"`
	Subscribed                 *string         `json:"subscribed,omitempty"`
	Unsubscribed               *string         `json:"unsubscribed,omitempty"`
	OrderResponse              json.RawMessage `json:"order_response,omitempty"`
	FillResponse               json.RawMessage `json:"fill_response,omitempty"`
	AllOrdersResponse          json.RawMessage `json:"all_orders_response,omitempty"`
	UserTradesResponse         json.RawMessage `json:"user_trades_response,omitempty"`
	UserLeverageResponse       json.RawMessage `json:"user_leverage_response,omitempty"`
	AvailableLeverageResponse  json.RawMessage `json:"available_leverage_levels_response,omitempty"`
	PositionResponse           json.RawMessage `json:"position_response,omitempty"`
	BalanceResponse            json.RawMessage `json:"balance_response,omitempty"`
	StopOrderResponse          json.RawMessage `json:"stop_order_response,omitempty"`
	TwapResponse               json.RawMessage `json:"twap_response,omitempty"`
	Ack                        json.RawMessage `json:"ack,omitempty"`
	Err                        json.RawMessage `json:"err,omitempty"`
}

// terminalOrderStatuses are order statuses that are final (not ACK).
var terminalOrderStatuses = map[string]bool{
	"FILLED":                              true,
	"CANCELLED":                           true,
	"CANCELLED_STP":                       true,
	"REJECTED":                            true,
	"NO_SUCH_ORDER":                       true,
	"INVALID_ORDER_TYPE":                  true,
	"BAD_SYMBOL":                          true,
	"PRICE_LESS_THAN_MIN_PRICE":           true,
	"PRICE_GREATER_THAN_MAX_PRICE":        true,
	"CANNOT_MODIFY_PARTIAL_FILL":          true,
	"FAILED_MARGIN_CHECK":                 true,
	"INVALID_TICK_SIZE_PRECISION_PRICE":   true,
	"INVALID_TICK_SIZE_PRECISION_QUANTITY": true,
	"QUANTITY_LESS_THAN_MIN_QUANTITY":     true,
	"QUANTITY_GREATER_THAN_MAX_QUANTITY":  true,
	"INVALID_TIME_IN_FORCE":               true,
	"REJECTED_WOULD_BREACH_MAX_NOTIONAL":  true,
	"CANNOT_MODIFY_NO_SUCH_ORDER":         true,
	"REJECTED_MARKET_CLOSED":              true,
	"REJECTED_FAILED_TO_PROCESS":          true,
	"INVALID_TAKEPROFIT_PRICE":            true,
	"INVALID_STOPLOSS_PRICE":              true,
	"RATE_LIMITED":                        true,
	"REJECTED_TOO_MANY_OPEN_ORDERS":       true,
	"REJECTED_OPEN_INTEREST_LIMIT":        true,
	"MODIFIED":                            true,
}

// pendingRequest waits for a specific response from the Trade WS.
type pendingRequest struct {
	// responseKey is the JSON key to match (e.g. "order_response")
	responseKey string
	// clientOrderID filters order responses (empty = match first)
	clientOrderID string
	// skipIfACK skips order_response messages with status "ACK", waiting for a terminal status.
	skipIfACK bool
	ch        chan json.RawMessage
}

// TradeWS manages the authenticated Trade WebSocket connection.
type TradeWS struct {
	url   string
	cfg   *config.Config
	state *State
	log   *log.Logger

	mu          sync.Mutex
	conn        *websocket.Conn
	authed      bool
	authWaiters []chan error

	pendingMu sync.Mutex
	pending   []*pendingRequest

	// Channel to receive authenticated notification
	authCh chan struct{}
}

func newTradeWS(url string, cfg *config.Config, state *State, logger *log.Logger) *TradeWS {
	return &TradeWS{
		url:    url,
		cfg:    cfg,
		state:  state,
		log:    logger,
		authCh: make(chan struct{}, 1),
	}
}

// Run connects and maintains the Trade WebSocket, reconnecting on failure.
func (t *TradeWS) Run(ctx context.Context) {
	if !t.cfg.HasCredentials() {
		t.log.Printf("Trade WS: no credentials configured, skipping")
		return
	}

	backoff := time.Second
	for {
		if err := t.connect(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			t.log.Printf("Trade WS: connection error: %v; retrying in %s", err, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			backoff = time.Duration(math.Min(float64(backoff*2), float64(30*time.Second)))
			continue
		}
		backoff = time.Second
	}
}

func (t *TradeWS) connect(ctx context.Context) error {
	// Proactively refresh the JWT before connecting so that a reconnect after
	// token expiry (e.g. daemon running overnight) authenticates successfully.
	if t.cfg.HasJWT() && oauth.IsTokenExpired(t.cfg.AccessToken) {
		t.log.Printf("Trade WS: access token expired, refreshing...")
		tokens, err := oauth.RefreshTokens(ctx, oauth.SupabaseURLForEnv(t.cfg.Env), t.cfg.RefreshToken)
		if err != nil {
			t.log.Printf("Trade WS: token refresh failed: %v", err)
		} else {
			t.cfg.AccessToken = tokens.AccessToken
			t.cfg.RefreshToken = tokens.RefreshToken
			if err := config.Save(t.cfg); err != nil {
				t.log.Printf("Trade WS: failed to save refreshed token: %v", err)
			}
		}
	}

	httpHeaders := http.Header{"User-Agent": {build.UserAgent()}}
	if t.cfg.HasJWT() {
		httpHeaders["Authorization"] = []string{"Bearer " + t.cfg.AccessToken}
	} else {
		headers, err := auth.RESTHeaders(t.cfg.PublicKey, t.cfg.SecretKey)
		if err != nil {
			return fmt.Errorf("auth headers: %w", err)
		}
		for k, v := range headers {
			httpHeaders[k] = []string{v}
		}
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, t.url, httpHeaders)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	t.log.Printf("Trade WS: connected to %s", t.url)

	t.mu.Lock()
	t.conn = conn
	t.authed = false
	t.mu.Unlock()

	defer func() {
		conn.Close()
		t.mu.Lock()
		if t.conn == conn {
			t.conn = nil
			t.authed = false
		}
		t.mu.Unlock()
		// Fail all pending requests
		t.pendingMu.Lock()
		for _, p := range t.pending {
			select {
			case p.ch <- nil:
			default:
			}
		}
		t.pending = nil
		t.pendingMu.Unlock()
	}()

	if err := t.sendAuth(conn); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	// Read loop
	for {
		if ctx.Err() != nil {
			return nil
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		t.dispatch(conn, data)
	}
}


func (t *TradeWS) sendAuth(conn *websocket.Conn) error {
	if t.cfg.HasJWT() {
		msg := map[string]any{
			"type": "auth",
			"params": map[string]any{
				"jwt": t.cfg.AccessToken,
			},
		}
		return conn.WriteJSON(msg)
	}
	creds, err := auth.NewHMACCredentials(t.cfg.PublicKey, t.cfg.SecretKey)
	if err != nil {
		return err
	}
	msg := map[string]any{
		"type": "auth",
		"params": map[string]any{
			"hmac": map[string]any{
				"public_key": creds.PublicKey,
				"nonce":      creds.Nonce,
				"unix_ts":    creds.UnixTS,
				"signature":  creds.Signature,
			},
		},
	}
	return conn.WriteJSON(msg)
}

func (t *TradeWS) dispatch(conn *websocket.Conn, data []byte) {
	var msg tradeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.log.Printf("Trade WS: parse error: %v", err)
		return
	}

	// Authentication response
	if msg.Authenticated != nil {
		if *msg.Authenticated {
			t.log.Printf("Trade WS: authenticated")
			t.mu.Lock()
			t.authed = true
			t.mu.Unlock()
			// Subscribe to user channels
			t.subscribeAll(conn)
		} else {
			t.log.Printf("Trade WS: authentication failed")
		}
		return
	}

	// Error response
	if len(msg.Err) > 0 {
		t.log.Printf("Trade WS: error: %s", string(msg.Err))
		// Fail first pending request if any
		t.failPending(msg.Err)
		return
	}

	// Ack response
	if len(msg.Ack) > 0 {
		t.resolvePending("ack", "", msg.Ack)
		return
	}

	// Subscription confirmations
	if msg.Subscribed != nil {
		t.log.Printf("Trade WS: subscribed to %s", *msg.Subscribed)
		return
	}

	// Direct responses to commands
	if len(msg.OrderResponse) > 0 {
		var order Order
		if err := json.Unmarshal(msg.OrderResponse, &order); err == nil {
			// Update state for non-terminal statuses
			switch order.Status {
			case "ACK", "MODIFIED":
				t.state.setOrder(&order)
			case "FILLED", "CANCELLED", "CANCELLED_STP":
				t.state.removeOrder(order.OrderID)
			}
			// Resolve pending request by client_order_id or generic order_response
			t.resolvePending("order_response", order.ClientOrderID, msg.OrderResponse)
			t.resolvePending("order_response", order.OrderID, msg.OrderResponse)
		}
		return
	}

	if len(msg.FillResponse) > 0 {
		var fill Fill
		if err := json.Unmarshal(msg.FillResponse, &fill); err == nil {
			t.state.addFill(&fill)
		}
		return
	}

	if len(msg.AllOrdersResponse) > 0 {
		t.resolvePending("all_orders_response", "", msg.AllOrdersResponse)
		return
	}

	if len(msg.UserTradesResponse) > 0 {
		t.resolvePending("user_trades_response", "", msg.UserTradesResponse)
		return
	}

	if len(msg.UserLeverageResponse) > 0 {
		t.resolvePending("user_leverage_response", "", msg.UserLeverageResponse)
		return
	}

	if len(msg.AvailableLeverageResponse) > 0 {
		t.resolvePending("available_leverage_levels_response", "", msg.AvailableLeverageResponse)
		return
	}

	if len(msg.PositionResponse) > 0 {
		t.resolvePending("position_response", "", msg.PositionResponse)
		return
	}

	if len(msg.BalanceResponse) > 0 {
		var bal Balance
		if err := json.Unmarshal(msg.BalanceResponse, &bal); err == nil {
			t.state.setBalance(&bal)
		}
		t.resolvePending("balance_response", "", msg.BalanceResponse)
		return
	}

	if len(msg.TwapResponse) > 0 {
		t.resolvePending("twap_response", "", msg.TwapResponse)
		return
	}

	if len(msg.StopOrderResponse) > 0 {
		t.resolvePending("stop_order_response", "", msg.StopOrderResponse)
		return
	}

	// Envelope / subscription updates
	if msg.Type != "" && len(msg.Contents) > 0 {
		t.dispatchEnvelope(&msg)
	}
}

func (t *TradeWS) dispatchEnvelope(msg *tradeMessage) {
	switch msg.Type {
	case "balance_update", "v4_balances", "subscribed":
		if msg.Channel == "v4_balances" || msg.Type == "balance_update" {
			var items []Balance
			if err := json.Unmarshal(msg.Contents, &items); err == nil && len(items) > 0 {
				t.state.setBalance(&items[0])
			}
		} else if msg.Channel == "v4_positions" || msg.Type == "position_update" {
			var items []Position
			if err := json.Unmarshal(msg.Contents, &items); err == nil {
				for i := range items {
					t.state.setPosition(&items[i])
				}
			}
		}
	case "position_update":
		var items []Position
		if err := json.Unmarshal(msg.Contents, &items); err == nil {
			for i := range items {
				t.state.setPosition(&items[i])
			}
		}
	}
}

func (t *TradeWS) subscribeAll(conn *websocket.Conn) {
	subs := []map[string]any{
		{"type": "subscribe", "params": map[string]any{"channels": []string{"order_responses"}}},
		{"type": "subscribe", "params": map[string]any{"channels": []string{"positions"}}},
		{"type": "subscribe", "params": map[string]any{"channels": []string{"balances"}}},
		{"type": "subscribe", "params": map[string]any{"channels": []string{"fills"}}},
	}
	for _, s := range subs {
		if err := conn.WriteJSON(s); err != nil {
			t.log.Printf("Trade WS: subscribe error: %v", err)
		}
	}
}

// Send sends a command to the Trade WS and waits for the expected response.
// responseKey is the JSON key expected in the response (e.g. "order_response").
// clientOrderID is used to match order responses (can be empty).
// Returns the raw JSON of the matched response value.
func (t *TradeWS) Send(ctx context.Context, cmd map[string]any, responseKey, clientOrderID string) (json.RawMessage, error) {
	t.mu.Lock()
	conn := t.conn
	authed := t.authed
	t.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected to trade WebSocket")
	}
	if !authed {
		return nil, fmt.Errorf("not authenticated")
	}

	p := &pendingRequest{
		responseKey:   responseKey,
		clientOrderID: clientOrderID,
		ch:            make(chan json.RawMessage, 1),
	}

	t.pendingMu.Lock()
	t.pending = append(t.pending, p)
	t.pendingMu.Unlock()

	defer func() {
		t.pendingMu.Lock()
		next := t.pending[:0]
		for _, x := range t.pending {
			if x != p {
				next = append(next, x)
			}
		}
		t.pending = next
		t.pendingMu.Unlock()
	}()

	if err := conn.WriteJSON(cmd); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	timeout := 10 * time.Second
	select {
	case data := <-p.ch:
		if data == nil {
			return nil, fmt.Errorf("connection closed while waiting for response")
		}
		// Check if it's an error
		var errCheck struct {
			ErrorCode string `json:"error_code"`
		}
		if json.Unmarshal(data, &errCheck) == nil && errCheck.ErrorCode != "" {
			return nil, fmt.Errorf("QFEX error: %s", errCheck.ErrorCode)
		}
		return data, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for %s response", responseKey)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendNoWait sends a command without waiting for a response.
func (t *TradeWS) SendNoWait(cmd map[string]any) error {
	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	return conn.WriteJSON(cmd)
}

// IsAuthed returns whether the connection is authenticated.
func (t *TradeWS) IsAuthed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.authed
}

// WaitAuth waits until the connection is authenticated or ctx expires.
func (t *TradeWS) WaitAuth(ctx context.Context) error {
	deadline := time.Now().Add(15 * time.Second)
	for {
		if t.IsAuthed() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("authentication timeout")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (t *TradeWS) resolvePending(responseKey, clientOrderID string, data json.RawMessage) bool {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	for i, p := range t.pending {
		if p.responseKey != responseKey {
			continue
		}
		if p.clientOrderID != "" && clientOrderID != "" && p.clientOrderID != clientOrderID {
			continue
		}
		// If this waiter only wants terminal statuses, skip ACK responses.
		if p.skipIfACK && responseKey == "order_response" {
			var o struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(data, &o) == nil && o.Status == "ACK" {
				continue
			}
		}
		select {
		case p.ch <- data:
		default:
		}
		t.pending = append(t.pending[:i], t.pending[i+1:]...)
		return true
	}
	return false
}

// PreRegisterFinal registers a pending request for the terminal order_response
// for the given clientOrderID before the order is sent. This avoids a race where
// the terminal status (FILLED, CANCELLED, etc.) arrives before WaitOnFinal is called.
// The caller must call WaitOnFinal with the returned channel.
func (t *TradeWS) PreRegisterFinal(clientOrderID string) chan json.RawMessage {
	p := &pendingRequest{
		responseKey:   "order_response",
		clientOrderID: clientOrderID,
		skipIfACK:     true,
		ch:            make(chan json.RawMessage, 1),
	}
	t.pendingMu.Lock()
	t.pending = append(t.pending, p)
	t.pendingMu.Unlock()
	return p.ch
}

// WaitOnFinal waits on a channel returned by PreRegisterFinal.
func (t *TradeWS) WaitOnFinal(ctx context.Context, ch chan json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	defer func() {
		t.pendingMu.Lock()
		next := t.pending[:0]
		for _, p := range t.pending {
			if p.ch != ch {
				next = append(next, p)
			}
		}
		t.pending = next
		t.pendingMu.Unlock()
	}()

	select {
	case data := <-ch:
		if data == nil {
			return nil, fmt.Errorf("connection closed while waiting for final status")
		}
		return data, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for final order status")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *TradeWS) failPending(errData json.RawMessage) {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	if len(t.pending) > 0 {
		p := t.pending[0]
		select {
		case p.ch <- errData:
		default:
		}
		t.pending = t.pending[1:]
	}
}
