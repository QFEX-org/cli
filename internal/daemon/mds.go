package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// mdsMessage is the raw incoming message from the MDS WebSocket.
// It uses a flat structure where type identifies the payload.
// Note: some numeric fields (usdVolume, trades, time_remaining) are sent as
// quoted strings by the exchange, so we use json.Number to handle both forms.
type mdsMessage struct {
	Type         string          `json:"type"`
	Sequence     int64           `json:"sequence,omitempty"`
	ConnectionID string          `json:"connection_id,omitempty"`
	MessageID    int64           `json:"message_id,omitempty"`
	Channel      string          `json:"channel,omitempty"`
	ID           string          `json:"id,omitempty"`
	Contents     json.RawMessage `json:"contents,omitempty"`

	// Flat fields used in some message formats
	Symbol        string          `json:"symbol,omitempty"`
	Time          string          `json:"time,omitempty"`
	Bid           [][]string      `json:"bid,omitempty"`
	Ask           [][]string      `json:"ask,omitempty"`
	SigFigs       int             `json:"sig_figs,omitempty"`
	TradeID       string          `json:"trade_id,omitempty"`
	Size          string          `json:"size,omitempty"`
	Price         string          `json:"price,omitempty"`
	Side          string          `json:"side,omitempty"`
	ExecutionType string          `json:"execution_type,omitempty"`
	Start         string          `json:"start,omitempty"`
	Resolution    string          `json:"resolution,omitempty"`
	Open          string          `json:"open,omitempty"`
	High          string          `json:"high,omitempty"`
	Low           string          `json:"low,omitempty"`
	Close         string          `json:"close,omitempty"`
	USDVolume     json.Number     `json:"usdVolume,omitempty"`
	Trades        json.Number     `json:"trades,omitempty"`
	FundingRate   string          `json:"funding_rate,omitempty"`
	TimeRemaining json.Number     `json:"time_remaining,omitempty"`
	OpenInterest  string          `json:"open_interest,omitempty"`
	MinPrice      string          `json:"min_price,omitempty"`
	MaxPrice      string          `json:"max_price,omitempty"`
	Source        string          `json:"source,omitempty"`
}

// MDSWS manages the connection to the QFEX Market Data Service WebSocket.
type MDSWS struct {
	url   string
	state *State
	log   *log.Logger

	mu          sync.Mutex
	conn        *websocket.Conn
	subscribed  map[string]bool // channel keys that have been subscribed
}

func newMDSWS(url string, state *State, logger *log.Logger) *MDSWS {
	return &MDSWS{
		url:        url,
		state:      state,
		log:        logger,
		subscribed: make(map[string]bool),
	}
}

// Run connects and maintains the MDS WebSocket, reconnecting on failure.
func (m *MDSWS) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if err := m.connect(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			m.log.Printf("MDS: connection error: %v; retrying in %s", err, backoff)
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

func (m *MDSWS) connect(ctx context.Context) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, m.url, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	m.log.Printf("MDS: connected to %s", m.url)

	m.mu.Lock()
	m.conn = conn
	subs := make(map[string]bool, len(m.subscribed))
	for k, v := range m.subscribed {
		subs[k] = v
	}
	m.mu.Unlock()

	// Re-subscribe to all channels after reconnect
	for key := range subs {
		if err := m.resubscribe(conn, key); err != nil {
			m.log.Printf("MDS: resubscribe %s failed: %v", key, err)
		}
	}

	defer func() {
		conn.Close()
		m.mu.Lock()
		if m.conn == conn {
			m.conn = nil
		}
		m.mu.Unlock()
	}()

	for {
		if ctx.Err() != nil {
			return nil
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		m.dispatch(data)
	}
}

func (m *MDSWS) dispatch(data []byte) {
	var msgs []mdsMessage

	// Try array first, then single object
	if err := json.Unmarshal(data, &msgs); err != nil {
		var single mdsMessage
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			m.log.Printf("MDS: failed to parse message: %s", string(data))
			return
		}
		msgs = []mdsMessage{single}
	}

	for _, msg := range msgs {
		// If message has contents array, process each item
		if len(msg.Contents) > 0 {
			var items []json.RawMessage
			if err := json.Unmarshal(msg.Contents, &items); err == nil {
				for _, item := range items {
					var sub mdsMessage
					if err := json.Unmarshal(item, &sub); err == nil {
						if sub.Type == "" {
							sub.Type = msg.Type
						}
						m.processMessage(&sub)
					}
				}
				continue
			}
		}
		m.processMessage(&msg)
	}
}

func (m *MDSWS) processMessage(msg *mdsMessage) {
	t, err := parseTime(msg.Time)
	if err != nil {
		t = time.Now()
	}

	switch msg.Type {
	case "level2":
		ob := &OrderBook{
			Symbol:  msg.Symbol,
			Time:    t,
			Bid:     msg.Bid,
			Ask:     msg.Ask,
			SigFigs: msg.SigFigs,
		}
		m.state.setOrderBook(ob)

	case "trade":
		trade := &PublicTrade{
			TradeID:       msg.TradeID,
			Time:          t,
			Symbol:        msg.Symbol,
			Size:          msg.Size,
			Price:         msg.Price,
			Side:          msg.Side,
			ExecutionType: msg.ExecutionType,
		}
		m.state.addPublicTrade(trade)

	case "candle":
		startT, _ := parseTime(msg.Start)
		usdVol, _ := msg.USDVolume.Int64()
		tradeCount, _ := msg.Trades.Int64()
		c := &Candle{
			Start:      startT,
			Symbol:     msg.Symbol,
			Resolution: msg.Resolution,
			Open:       msg.Open,
			High:       msg.High,
			Low:        msg.Low,
			Close:      msg.Close,
			USDVolume:  usdVol,
			Trades:     int(tradeCount),
		}
		m.state.setCandle(c)

	case "bbo":
		bbo := &BBO{
			Symbol: msg.Symbol,
			Time:   t,
			Bid:    msg.Bid,
			Ask:    msg.Ask,
		}
		m.state.setBBO(bbo)

	case "mark_price":
		mp := &MarkPrice{
			Symbol: msg.Symbol,
			Price:  msg.Price,
			Time:   t,
		}
		m.state.setMarkPrice(mp)

	case "funding":
		timeRemaining, _ := msg.TimeRemaining.Int64()
		fr := &FundingRate{
			Symbol:        msg.Symbol,
			Rate:          msg.FundingRate,
			TimeRemaining: int(timeRemaining),
			Time:          t,
		}
		m.state.setFundingRate(fr)

	case "open_interest":
		oi := &OpenInterest{
			Symbol:       msg.Symbol,
			OpenInterest: msg.OpenInterest,
			Time:         t,
		}
		m.state.setOpenInterest(oi)

	case "underlier":
		// Store as mark price
		mp := &MarkPrice{
			Symbol: msg.Symbol,
			Price:  msg.Price,
			Time:   t,
		}
		m.state.setMarkPrice(mp)
	}
}

// Subscribe sends a subscription command for the given channel and symbol(s).
// The key is used to track subscriptions for re-subscribe on reconnect.
func (m *MDSWS) Subscribe(channel string, symbols []string) error {
	return m.subscribe(channel, symbols, nil)
}

// SubscribeCandles subscribes to candle data for the given symbol and intervals.
func (m *MDSWS) SubscribeCandles(symbols []string, intervals []string) error {
	return m.subscribe("candle", symbols, intervals)
}

func (m *MDSWS) subscribe(channel string, symbols []string, intervals []string) error {
	m.mu.Lock()
	conn := m.conn
	key := subscriptionKey(channel, symbols, intervals)
	m.subscribed[key] = true
	m.mu.Unlock()

	if conn == nil {
		return nil // will re-subscribe on connect
	}
	return m.sendSubscribe(conn, channel, symbols, intervals)
}

func (m *MDSWS) resubscribe(conn *websocket.Conn, key string) error {
	channel, symbols, intervals := parseSubscriptionKey(key)
	return m.sendSubscribe(conn, channel, symbols, intervals)
}

func (m *MDSWS) sendSubscribe(conn *websocket.Conn, channel string, symbols []string, intervals []string) error {
	msg := map[string]any{
		"type":     "subscribe",
		"channels": []string{channel},
		"symbols":  symbols,
	}
	if len(intervals) > 0 {
		msg["intervals"] = intervals
	}
	return conn.WriteJSON(msg)
}

func subscriptionKey(channel string, symbols []string, intervals []string) string {
	b, _ := json.Marshal(map[string]any{
		"channel":   channel,
		"symbols":   symbols,
		"intervals": intervals,
	})
	return string(b)
}

func parseSubscriptionKey(key string) (channel string, symbols []string, intervals []string) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(key), &m); err != nil {
		return
	}
	json.Unmarshal(m["channel"], &channel)
	json.Unmarshal(m["symbols"], &symbols)
	json.Unmarshal(m["intervals"], &intervals)
	return
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}
