package daemon

import (
	"encoding/json"
	"sync"
	"time"
)

// ---- Market data types ----

type BBO struct {
	Symbol string     `json:"symbol"`
	Time   time.Time  `json:"time"`
	Bid    [][]string `json:"bid"`
	Ask    [][]string `json:"ask"`
}

type OrderBook struct {
	Symbol  string     `json:"symbol"`
	Time    time.Time  `json:"time"`
	Bid     [][]string `json:"bid"`
	Ask     [][]string `json:"ask"`
	SigFigs int        `json:"sig_figs"`
}

type PublicTrade struct {
	TradeID       string    `json:"trade_id"`
	Time          time.Time `json:"time"`
	Symbol        string    `json:"symbol"`
	Size          string    `json:"size"`
	Price         string    `json:"price"`
	Side          string    `json:"side"`
	ExecutionType string    `json:"execution_type,omitempty"`
}

type Candle struct {
	Start      time.Time `json:"start"`
	Symbol     string    `json:"symbol"`
	Resolution string    `json:"resolution"`
	Open       string    `json:"open"`
	High       string    `json:"high"`
	Low        string    `json:"low"`
	Close      string    `json:"close"`
	USDVolume  int64     `json:"usdVolume"`
	Trades     int       `json:"trades"`
}

type MarkPrice struct {
	Symbol string    `json:"symbol"`
	Price  string    `json:"price"`
	Time   time.Time `json:"time"`
}

type FundingRate struct {
	Symbol        string    `json:"symbol"`
	Rate          string    `json:"funding_rate"`
	TimeRemaining int       `json:"time_remaining"`
	Time          time.Time `json:"time"`
}

type OpenInterest struct {
	Symbol       string    `json:"symbol"`
	OpenInterest string    `json:"open_interest"`
	Time         time.Time `json:"time"`
}

// ---- Account types ----

type Balance struct {
	ID               string  `json:"id,omitempty"`
	UserID           string  `json:"user_id,omitempty"`
	Deposit          float64 `json:"deposit"`
	RealisedPnl      float64 `json:"realised_pnl"`
	UnrealisedPnl    float64 `json:"unrealised_pnl"`
	OrderMargin      float64 `json:"order_margin"`
	PositionMargin   float64 `json:"position_margin"`
	AvailableBalance float64 `json:"available_balance"`
	NetFunding       float64 `json:"net_funding"`
	Fees             float64 `json:"fees,omitempty"`
}

type Position struct {
	ID                string  `json:"id,omitempty"`
	Symbol            string  `json:"symbol"`
	Position          float64 `json:"position"`
	AveragePrice      float64 `json:"average_price"`
	RealisedPnl       float64 `json:"realised_pnl"`
	UnrealisedPnl     float64 `json:"unrealised_pnl"`
	NetFunding        float64 `json:"net_funding"`
	Leverage          float64 `json:"leverage"`
	InitialMargin     float64 `json:"initial_margin"`
	MaintenanceMargin float64 `json:"maintenance_margin"`
	OpenOrders        float64 `json:"open_orders"`
	OpenQuantity      float64 `json:"open_quantity"`
	MarginAlloc       float64 `json:"margin_alloc"`
}

type Order struct {
	OrderID           string  `json:"order_id"`
	Symbol            string  `json:"symbol"`
	Status            string  `json:"status"`
	Quantity          float64 `json:"quantity"`
	Price             float64 `json:"price"`
	TakeProfit        float64 `json:"take_profit,omitempty"`
	StopLoss          float64 `json:"stop_loss,omitempty"`
	Side              string  `json:"side"`
	Type              string  `json:"type"`
	TimeInForce       string  `json:"time_in_force"`
	ClientOrderID     string  `json:"client_order_id,omitempty"`
	QuantityRemaining float64 `json:"quantity_remaining"`
	UpdateTime        float64 `json:"update_time"`
	TradeID           string  `json:"trade_id,omitempty"`
	UserID            string  `json:"user_id,omitempty"`
}

type Fill struct {
	TradeID           string  `json:"trade_id"`
	UserID            string  `json:"user_id,omitempty"`
	Symbol            string  `json:"symbol"`
	Price             float64 `json:"price"`
	Quantity          float64 `json:"quantity"`
	Side              string  `json:"side"`
	AggressorSide     string  `json:"aggressor_side,omitempty"`
	OrderID           string  `json:"order_id"`
	Fee               float64 `json:"fee"`
	OrderType         string  `json:"order_type"`
	TIF               string  `json:"tif,omitempty"`
	OrderPrice        float64 `json:"order_price"`
	ClientOrderID     string  `json:"client_order_id,omitempty"`
	RemainingQuantity float64 `json:"remaining_quantity"`
	RealisedPnl       float64 `json:"realised_pnl"`
	Timestamp         float64 `json:"timestamp"`
}

// ---- Watcher ----

type Watcher struct {
	stream   string
	symbol   string
	interval string
	ch       chan json.RawMessage
}

// ---- State ----

const maxRecentTrades = 200
const maxRecentFills = 200

type State struct {
	mu sync.RWMutex

	// MDS
	orderBooks    map[string]*OrderBook
	bbos          map[string]*BBO
	recentTrades  map[string][]*PublicTrade
	candles       map[string]map[string]*Candle // symbol → interval → candle
	markPrices    map[string]*MarkPrice
	fundingRates  map[string]*FundingRate
	openInterests map[string]*OpenInterest

	// Trade WS
	positions  map[string]*Position
	balance    *Balance
	openOrders map[string]*Order // order_id → order
	fills      []*Fill

	// Watchers
	watcherMu sync.Mutex
	watchers  []*Watcher
}

func newState() *State {
	return &State{
		orderBooks:    make(map[string]*OrderBook),
		bbos:          make(map[string]*BBO),
		recentTrades:  make(map[string][]*PublicTrade),
		candles:       make(map[string]map[string]*Candle),
		markPrices:    make(map[string]*MarkPrice),
		fundingRates:  make(map[string]*FundingRate),
		openInterests: make(map[string]*OpenInterest),
		positions:     make(map[string]*Position),
		openOrders:    make(map[string]*Order),
	}
}

func (s *State) setBBO(b *BBO) {
	s.mu.Lock()
	s.bbos[b.Symbol] = b
	s.mu.Unlock()
	s.notify("bbo", b.Symbol, mustMarshal(b))
}

func (s *State) getBBO(symbol string) *BBO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bbos[symbol]
}

func (s *State) setOrderBook(ob *OrderBook) {
	s.mu.Lock()
	s.orderBooks[ob.Symbol] = ob
	s.mu.Unlock()
	s.notify("orderbook", ob.Symbol, mustMarshal(ob))
}

func (s *State) getOrderBook(symbol string) *OrderBook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orderBooks[symbol]
}

func (s *State) addPublicTrade(t *PublicTrade) {
	s.mu.Lock()
	trades := append(s.recentTrades[t.Symbol], t)
	if len(trades) > maxRecentTrades {
		trades = trades[len(trades)-maxRecentTrades:]
	}
	s.recentTrades[t.Symbol] = trades
	s.mu.Unlock()
	s.notify("trades", t.Symbol, mustMarshal(t))
}

func (s *State) getRecentTrades(symbol string, limit int) []*PublicTrade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trades := s.recentTrades[symbol]
	if limit > 0 && len(trades) > limit {
		trades = trades[len(trades)-limit:]
	}
	return trades
}

func (s *State) setCandle(c *Candle) {
	s.mu.Lock()
	if s.candles[c.Symbol] == nil {
		s.candles[c.Symbol] = make(map[string]*Candle)
	}
	s.candles[c.Symbol][c.Resolution] = c
	s.mu.Unlock()
}

func (s *State) getCandle(symbol, interval string) *Candle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m := s.candles[symbol]; m != nil {
		return m[interval]
	}
	return nil
}

func (s *State) setMarkPrice(mp *MarkPrice) {
	s.mu.Lock()
	s.markPrices[mp.Symbol] = mp
	s.mu.Unlock()
	s.notify("mark_price", mp.Symbol, mustMarshal(mp))
}

func (s *State) getMarkPrice(symbol string) *MarkPrice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.markPrices[symbol]
}

func (s *State) setFundingRate(fr *FundingRate) {
	s.mu.Lock()
	s.fundingRates[fr.Symbol] = fr
	s.mu.Unlock()
	s.notify("funding_rate", fr.Symbol, mustMarshal(fr))
}

func (s *State) getFundingRate(symbol string) *FundingRate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fundingRates[symbol]
}

func (s *State) setOpenInterest(oi *OpenInterest) {
	s.mu.Lock()
	s.openInterests[oi.Symbol] = oi
	s.mu.Unlock()
}

func (s *State) getOpenInterest(symbol string) *OpenInterest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.openInterests[symbol]
}

func (s *State) setBalance(b *Balance) {
	s.mu.Lock()
	s.balance = b
	s.mu.Unlock()
	s.notify("balance", "", mustMarshal(b))
}

func (s *State) getBalance() *Balance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.balance
}

func (s *State) setPosition(p *Position) {
	s.mu.Lock()
	s.positions[p.Symbol] = p
	s.mu.Unlock()
	s.notify("positions", p.Symbol, mustMarshal(p))
}

func (s *State) getAllPositions() []*Position {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Position, 0, len(s.positions))
	for _, p := range s.positions {
		out = append(out, p)
	}
	return out
}

func (s *State) setOrder(o *Order) {
	s.mu.Lock()
	s.openOrders[o.OrderID] = o
	s.mu.Unlock()
	s.notify("orders", o.Symbol, mustMarshal(o))
}

func (s *State) removeOrder(orderID string) {
	s.mu.Lock()
	delete(s.openOrders, orderID)
	s.mu.Unlock()
}

func (s *State) getOrder(orderID string) *Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.openOrders[orderID]
}

func (s *State) getAllOrders() []*Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Order, 0, len(s.openOrders))
	for _, o := range s.openOrders {
		out = append(out, o)
	}
	return out
}

func (s *State) addFill(f *Fill) {
	s.mu.Lock()
	s.fills = append(s.fills, f)
	if len(s.fills) > maxRecentFills {
		s.fills = s.fills[len(s.fills)-maxRecentFills:]
	}
	s.mu.Unlock()
	s.notify("fills", f.Symbol, mustMarshal(f))
}

func (s *State) getFills(limit int) []*Fill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fills := s.fills
	if limit > 0 && len(fills) > limit {
		fills = fills[len(fills)-limit:]
	}
	return fills
}

// ---- Watchers ----

func (s *State) addWatcher(w *Watcher) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()
	s.watchers = append(s.watchers, w)
}

func (s *State) removeWatcher(w *Watcher) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()
	next := s.watchers[:0]
	for _, existing := range s.watchers {
		if existing != w {
			next = append(next, existing)
		}
	}
	s.watchers = next
}

func (s *State) notify(stream, symbol string, data json.RawMessage) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()
	for _, w := range s.watchers {
		if w.stream != stream {
			continue
		}
		if w.symbol != "" && w.symbol != symbol {
			continue
		}
		select {
		case w.ch <- data:
		default: // drop if slow consumer
		}
	}
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
