package daemon

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/qfex/cli/internal/auth"
	"github.com/qfex/cli/internal/protocol"
)

// Server listens on a Unix domain socket and dispatches IPC requests.
type Server struct {
	socketPath string
	d          *Daemon
	log        *log.Logger
}

func newServer(socketPath string, d *Daemon, logger *log.Logger) *Server {
	return &Server{socketPath: socketPath, d: d, log: logger}
}

func (s *Server) Run(ctx context.Context) error {
	// Remove stale socket
	os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.socketPath, err)
	}
	defer func() {
		ln.Close()
		os.Remove(s.socketPath)
	}()

	s.log.Printf("Server: listening on %s", s.socketPath)

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			s.log.Printf("Server: accept error: %v", err)
			continue
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req protocol.Request
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(conn, protocol.Response{OK: false, Error: "invalid JSON: " + err.Error()})
			continue
		}

		// Watch commands keep the connection open
		if req.Cmd == protocol.CmdWatch {
			s.handleWatch(ctx, conn, &req)
			return
		}

		resp := s.dispatch(ctx, &req)
		writeResponse(conn, resp)
	}
}

func (s *Server) handleWatch(ctx context.Context, conn net.Conn, req *protocol.Request) {
	var params protocol.WatchParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeResponse(conn, protocol.Response{OK: false, Error: "invalid params: " + err.Error()})
		return
	}

	// Subscribe to MDS channel if needed
	if err := s.d.ensureMDSSubscription(params.Stream, params.Symbol, params.Interval); err != nil {
		writeResponse(conn, protocol.Response{OK: false, Error: err.Error()})
		return
	}

	w := &Watcher{
		stream:   params.Stream,
		symbol:   params.Symbol,
		interval: params.Interval,
		ch:       make(chan json.RawMessage, 64),
	}
	s.d.state.addWatcher(w)
	defer s.d.state.removeWatcher(w)

	// Send current value immediately if available
	if initial := s.d.getCurrentValue(params.Stream, params.Symbol, params.Interval); initial != nil {
		evt := protocol.Event{Type: params.Stream, Data: initial}
		writeEvent(conn, evt)
	}

	// Stream updates
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		// Detect client disconnect by reading; a real EOF/reset cancels the stream.
		// Timeout errors are ignored — they just mean the client sent no data (expected).
		buf := make([]byte, 1)
		for {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, err := conn.Read(buf)
			if err == nil {
				continue
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			cancel()
			return
		}
	}()

	for {
		select {
		case data := <-w.ch:
			evt := protocol.Event{Type: params.Stream, Data: data}
			if err := writeEvent(conn, evt); err != nil {
				return
			}
		case <-connCtx.Done():
			return
		}
	}
}

func (s *Server) dispatch(ctx context.Context, req *protocol.Request) protocol.Response {
	switch req.Cmd {
	case protocol.CmdPing:
		return okResp(json.RawMessage(`"pong"`))

	case protocol.CmdStatus:
		return s.handleStatus()

	// ---- Market data ----
	case protocol.CmdGetBBO:
		var p protocol.GetBBOParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetBBO(p)

	case protocol.CmdGetOrderBook:
		var p protocol.GetOrderBookParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetOrderBook(p)

	case protocol.CmdGetTrades:
		var p protocol.GetTradesParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetTrades(p)

	case protocol.CmdGetCandles:
		var p protocol.GetCandlesParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetCandles(p)

	case protocol.CmdGetMarkPrice:
		var p protocol.GetMarkPriceParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetMarkPrice(p)

	case protocol.CmdGetFundingRate:
		var p protocol.GetFundingRateParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetFundingRate(p)

	case protocol.CmdGetOpenInterest:
		var p protocol.GetOpenInterestParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetOpenInterest(p)

	// ---- Orders ----
	case protocol.CmdPlaceOrder:
		var p protocol.PlaceOrderParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handlePlaceOrder(ctx, p)

	case protocol.CmdCancelOrder:
		var p protocol.CancelOrderParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleCancelOrder(ctx, p)

	case protocol.CmdCancelAll:
		return s.handleCancelAll(ctx)

	case protocol.CmdModifyOrder:
		var p protocol.ModifyOrderParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleModifyOrder(ctx, p)

	case protocol.CmdGetOrder:
		var p protocol.GetOrderParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetOrder(ctx, p)

	case protocol.CmdGetOrders:
		var p protocol.GetOrdersParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetOrders(ctx, p)

	// ---- Positions ----
	case protocol.CmdGetPositions:
		return s.handleGetPositions()

	case protocol.CmdClosePosition:
		var p protocol.ClosePositionParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleClosePosition(ctx, p)

	// ---- Account ----
	case protocol.CmdGetBalance:
		return s.handleGetBalance(ctx)

	case protocol.CmdGetLeverage:
		var p protocol.GetLeverageParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetLeverage(ctx, p)

	case protocol.CmdSetLeverage:
		var p protocol.SetLeverageParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleSetLeverage(ctx, p)

	case protocol.CmdGetAvailableLeverage:
		var p protocol.GetLeverageParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetAvailableLeverage(ctx, p)

	// ---- TWAP ----
	case protocol.CmdAddTwap:
		var p protocol.AddTwapParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleAddTwap(ctx, p)

	// ---- Stop orders ----
	case protocol.CmdCancelStopOrder:
		var p protocol.CancelStopOrderParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleCancelStopOrder(ctx, p)

	case protocol.CmdModifyStopOrder:
		var p protocol.ModifyStopOrderParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleModifyStopOrder(ctx, p)

	// ---- Fills & trades ----
	case protocol.CmdGetFills:
		var p protocol.GetFillsParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetFills(p)

	case protocol.CmdGetUserTrades:
		var p protocol.GetUserTradesParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleGetUserTrades(ctx, p)

	case protocol.CmdGetDepositAddress:
		return s.handleGetDepositAddress(ctx)

	case protocol.CmdCancelOnDisconnect:
		var p protocol.CancelOnDisconnectParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(err.Error())
		}
		return s.handleCancelOnDisconnect(ctx, p)

	default:
		return errResp(fmt.Sprintf("unknown command: %s", req.Cmd))
	}
}

// ---- Handlers ----

func (s *Server) handleStatus() protocol.Response {
	env := "prod"
	if s.d.cfg.IsUAT() {
		env = "uat"
	}
	return okResp(mustMarshal(map[string]any{
		"running":       true,
		"env":           env,
		"mds_url":       s.d.cfg.MDS(),
		"trade_url":     s.d.cfg.TradeWS(),
		"mds_connected": s.d.mds.conn != nil,
		"trade_authed":  s.d.trade.IsAuthed(),
	}))
}

func (s *Server) handleGetBBO(p protocol.GetBBOParams) protocol.Response {
	bbo := s.d.state.getBBO(p.Symbol)
	if bbo == nil {
		// Subscribe and wait briefly
		s.d.mds.Subscribe("bbo", []string{p.Symbol})
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if bbo = s.d.state.getBBO(p.Symbol); bbo != nil {
				break
			}
		}
	}
	if bbo == nil {
		return errResp(fmt.Sprintf("no BBO data for %s (subscribed, waiting for data)", p.Symbol))
	}
	return okResp(mustMarshal(bbo))
}

func (s *Server) handleGetOrderBook(p protocol.GetOrderBookParams) protocol.Response {
	ob := s.d.state.getOrderBook(p.Symbol)
	if ob == nil {
		s.d.mds.Subscribe("level2", []string{p.Symbol})
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if ob = s.d.state.getOrderBook(p.Symbol); ob != nil {
				break
			}
		}
	}
	if ob == nil {
		return errResp(fmt.Sprintf("no order book data for %s", p.Symbol))
	}
	depth := p.Depth
	if depth > 0 && ob != nil {
		trimmed := *ob
		if len(trimmed.Bid) > depth {
			trimmed.Bid = trimmed.Bid[:depth]
		}
		if len(trimmed.Ask) > depth {
			trimmed.Ask = trimmed.Ask[:depth]
		}
		return okResp(mustMarshal(&trimmed))
	}
	return okResp(mustMarshal(ob))
}

func (s *Server) handleGetTrades(p protocol.GetTradesParams) protocol.Response {
	trades := s.d.state.getRecentTrades(p.Symbol, p.Limit)
	if len(trades) == 0 {
		s.d.mds.Subscribe("trade", []string{p.Symbol})
	}
	return okResp(mustMarshal(trades))
}

func (s *Server) handleGetCandles(p protocol.GetCandlesParams) protocol.Response {
	c := s.d.state.getCandle(p.Symbol, p.Interval)
	if c == nil {
		s.d.mds.SubscribeCandles([]string{p.Symbol}, []string{p.Interval})
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if c = s.d.state.getCandle(p.Symbol, p.Interval); c != nil {
				break
			}
		}
	}
	if c == nil {
		return errResp(fmt.Sprintf("no candle data for %s/%s", p.Symbol, p.Interval))
	}
	return okResp(mustMarshal(c))
}

func (s *Server) handleGetMarkPrice(p protocol.GetMarkPriceParams) protocol.Response {
	mp := s.d.state.getMarkPrice(p.Symbol)
	if mp == nil {
		s.d.mds.Subscribe("mark_price", []string{p.Symbol})
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if mp = s.d.state.getMarkPrice(p.Symbol); mp != nil {
				break
			}
		}
	}
	if mp == nil {
		return errResp(fmt.Sprintf("no mark price data for %s", p.Symbol))
	}
	return okResp(mustMarshal(mp))
}

func (s *Server) handleGetFundingRate(p protocol.GetFundingRateParams) protocol.Response {
	fr := s.d.state.getFundingRate(p.Symbol)
	if fr == nil {
		s.d.mds.Subscribe("funding", []string{p.Symbol})
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if fr = s.d.state.getFundingRate(p.Symbol); fr != nil {
				break
			}
		}
	}
	if fr == nil {
		return errResp(fmt.Sprintf("no funding rate data for %s", p.Symbol))
	}
	return okResp(mustMarshal(fr))
}

func (s *Server) handleGetOpenInterest(p protocol.GetOpenInterestParams) protocol.Response {
	oi := s.d.state.getOpenInterest(p.Symbol)
	if oi == nil {
		s.d.mds.Subscribe("open_interest", []string{p.Symbol})
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if oi = s.d.state.getOpenInterest(p.Symbol); oi != nil {
				break
			}
		}
	}
	if oi == nil {
		return errResp(fmt.Sprintf("no open interest data for %s", p.Symbol))
	}
	return okResp(mustMarshal(oi))
}

func (s *Server) handlePlaceOrder(ctx context.Context, p protocol.PlaceOrderParams) protocol.Response {
	// When waiting for final status we need a known client_order_id to correlate
	// the terminal response, which may arrive milliseconds after the ACK.
	if p.WaitForFinal && p.ClientOrderID == "" {
		p.ClientOrderID = newClientOrderID()
	}

	cmd := map[string]any{
		"type": "add_order",
		"params": map[string]any{
			"symbol":              p.Symbol,
			"side":                p.Side,
			"order_type":          p.OrderType,
			"order_time_in_force": p.TimeInForce,
			"quantity":            p.Quantity,
			"price":               p.Price,
			"take_profit":         p.TakeProfit,
			"stop_loss":           p.StopLoss,
			"client_order_id":     p.ClientOrderID,
		},
	}
	params := cmd["params"].(map[string]any)
	if p.TakeProfit == 0 {
		delete(params, "take_profit")
	}
	if p.StopLoss == 0 {
		delete(params, "stop_loss")
	}
	if p.ClientOrderID == "" {
		delete(params, "client_order_id")
	}

	// Pre-register the final-status pending BEFORE sending the order so we
	// cannot miss a terminal response that arrives right after the ACK.
	var finalCh chan json.RawMessage
	if p.WaitForFinal {
		finalCh = s.d.trade.PreRegisterFinal(p.ClientOrderID)
	}

	ackData, err := s.d.trade.Send(ctx, cmd, "order_response", p.ClientOrderID)
	if err != nil {
		return errResp(err.Error())
	}

	if !p.WaitForFinal {
		return okResp(ackData)
	}

	// If the ACK is already terminal (e.g. immediate REJECTED), return it.
	var ack Order
	if err := json.Unmarshal(ackData, &ack); err != nil || ack.Status != "ACK" {
		return okResp(ackData)
	}

	finalData, err := s.d.trade.WaitOnFinal(ctx, finalCh, 30*time.Second)
	if err != nil {
		// Timed out (e.g. GTC resting on book) — return the ACK we have.
		return okResp(ackData)
	}
	return okResp(finalData)
}

func (s *Server) handleCancelOrder(ctx context.Context, p protocol.CancelOrderParams) protocol.Response {
	cmd := map[string]any{
		"type": "cancel_order",
		"params": map[string]any{
			"symbol":               p.Symbol,
			"order_id":             p.OrderID,
			"cancel_order_id_type": p.CancelIDType,
		},
	}
	// Cancel responses are always terminal (CANCELLED, NO_SUCH_ORDER, etc.).
	// Match by order_id when the id type is order_id for precision.
	matchID := ""
	if p.CancelIDType == "order_id" {
		matchID = p.OrderID
	}
	data, err := s.d.trade.Send(ctx, cmd, "order_response", matchID)
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleCancelAll(ctx context.Context) protocol.Response {
	cmd := map[string]any{
		"type":   "cancel_all_orders",
		"params": map[string]any{},
	}
	data, err := s.d.trade.Send(ctx, cmd, "ack", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleModifyOrder(ctx context.Context, p protocol.ModifyOrderParams) protocol.Response {
	params := map[string]any{
		"symbol":     p.Symbol,
		"order_id":   p.OrderID,
		"side":       p.Side,
		"order_type": p.OrderType,
	}
	if p.Price != 0 {
		params["price"] = p.Price
	}
	if p.Quantity != 0 {
		params["quantity"] = p.Quantity
	}
	if p.TakeProfit != 0 {
		params["take_profit"] = p.TakeProfit
	}
	if p.StopLoss != 0 {
		params["stop_loss"] = p.StopLoss
	}
	cmd := map[string]any{"type": "modify_order", "params": params}
	// Modify responses are always terminal (MODIFIED, CANNOT_MODIFY_NO_SUCH_ORDER, etc.).
	// Match by order_id for precision.
	data, err := s.d.trade.Send(ctx, cmd, "order_response", p.OrderID)
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleGetOrder(ctx context.Context, p protocol.GetOrderParams) protocol.Response {
	// Check local cache first
	if o := s.d.state.getOrder(p.OrderID); o != nil {
		return okResp(mustMarshal(o))
	}
	cmd := map[string]any{
		"type":   "get_order",
		"params": map[string]any{"order_id": p.OrderID, "symbol": p.Symbol},
	}
	data, err := s.d.trade.Send(ctx, cmd, "order_response", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleGetOrders(ctx context.Context, p protocol.GetOrdersParams) protocol.Response {
	params := map[string]any{}
	if p.Limit > 0 {
		params["limit"] = p.Limit
	}
	if p.Offset > 0 {
		params["offset"] = p.Offset
	}
	if p.Symbol != "" {
		params["symbol"] = p.Symbol
	}
	cmd := map[string]any{"type": "get_user_orders", "params": params}
	data, err := s.d.trade.Send(ctx, cmd, "all_orders_response", "")
	if err != nil {
		// Fall back to local cache
		orders := s.d.state.getAllOrders()
		return okResp(mustMarshal(orders))
	}
	return okResp(data)
}

func (s *Server) handleGetPositions() protocol.Response {
	positions := s.d.state.getAllPositions()
	return okResp(mustMarshal(positions))
}

func (s *Server) handleClosePosition(ctx context.Context, p protocol.ClosePositionParams) protocol.Response {
	params := map[string]any{"symbol": p.Symbol}
	if p.ClientOrderID != "" {
		params["client_order_id"] = p.ClientOrderID
	}
	cmd := map[string]any{"type": "close_position", "params": params}
	data, err := s.d.trade.Send(ctx, cmd, "order_response", p.ClientOrderID)
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleGetBalance(ctx context.Context) protocol.Response {
	if bal := s.d.state.getBalance(); bal != nil {
		return okResp(mustMarshal(bal))
	}
	cmd := map[string]any{
		"type":   "subscribe",
		"params": map[string]any{"channels": []string{"balances"}},
	}
	s.d.trade.SendNoWait(cmd)
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if bal := s.d.state.getBalance(); bal != nil {
			return okResp(mustMarshal(bal))
		}
	}
	return errResp("no balance data available")
}

func (s *Server) handleGetLeverage(ctx context.Context, p protocol.GetLeverageParams) protocol.Response {
	params := map[string]any{}
	if p.Limit > 0 {
		params["limit"] = p.Limit
	}
	if p.Offset > 0 {
		params["offset"] = p.Offset
	}
	cmd := map[string]any{"type": "get_user_leverage", "params": params}
	data, err := s.d.trade.Send(ctx, cmd, "user_leverage_response", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleSetLeverage(ctx context.Context, p protocol.SetLeverageParams) protocol.Response {
	cmd := map[string]any{
		"type":   "set_user_leverage",
		"params": map[string]any{"symbol": p.Symbol, "leverage": p.Leverage},
	}
	data, err := s.d.trade.Send(ctx, cmd, "ack", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleGetAvailableLeverage(ctx context.Context, p protocol.GetLeverageParams) protocol.Response {
	params := map[string]any{}
	if p.Limit > 0 {
		params["limit"] = p.Limit
	}
	if p.Offset > 0 {
		params["offset"] = p.Offset
	}
	cmd := map[string]any{"type": "get_available_leverage_levels", "params": params}
	data, err := s.d.trade.Send(ctx, cmd, "available_leverage_levels_response", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleAddTwap(ctx context.Context, p protocol.AddTwapParams) protocol.Response {
	params := map[string]any{
		"symbol":              p.Symbol,
		"side":                p.Side,
		"total_quantity":      p.TotalQuantity,
		"num_orders":          p.NumOrders,
		"order_interval_secs": p.OrderIntervalSecs,
		"reduce_only":         p.ReduceOnly,
	}
	if p.ClientTwapID != "" {
		params["client_twap_id"] = p.ClientTwapID
	}
	cmd := map[string]any{"type": "add_twap", "params": params}
	data, err := s.d.trade.Send(ctx, cmd, "twap_response", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleCancelStopOrder(ctx context.Context, p protocol.CancelStopOrderParams) protocol.Response {
	cmd := map[string]any{
		"type":   "cancel_stop_order",
		"params": map[string]any{"stop_order_id": p.StopOrderID},
	}
	data, err := s.d.trade.Send(ctx, cmd, "ack", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleModifyStopOrder(ctx context.Context, p protocol.ModifyStopOrderParams) protocol.Response {
	cmd := map[string]any{
		"type": "modify_stop_order",
		"params": map[string]any{
			"stop_order_id": p.StopOrderID,
			"symbol":        p.Symbol,
			"price":         p.Price,
			"quantity":      p.Quantity,
		},
	}
	data, err := s.d.trade.Send(ctx, cmd, "ack", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleGetFills(p protocol.GetFillsParams) protocol.Response {
	fills := s.d.state.getFills(p.Limit, p.Symbol)
	return okResp(mustMarshal(fills))
}

func (s *Server) handleGetUserTrades(ctx context.Context, p protocol.GetUserTradesParams) protocol.Response {
	params := map[string]any{}
	if p.Limit > 0 {
		params["limit"] = p.Limit
	}
	if p.Offset > 0 {
		params["offset"] = p.Offset
	}
	if p.OrderID != "" {
		params["order_id"] = p.OrderID
	}
	if p.StartTS != 0 {
		params["start_ts"] = p.StartTS
	}
	if p.EndTS != 0 {
		params["end_ts"] = p.EndTS
	}
	cmd := map[string]any{"type": "get_user_trades", "params": params}
	data, err := s.d.trade.Send(ctx, cmd, "user_trades_response", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

func (s *Server) handleGetDepositAddress(ctx context.Context) protocol.Response {
	if !s.d.cfg.HasCredentials() {
		return errResp("credentials required: set public_key and secret_key in config")
	}

	headers, err := auth.RESTHeaders(s.d.cfg.PublicKey, s.d.cfg.SecretKey)
	if err != nil {
		return errResp(fmt.Sprintf("building auth headers: %v", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.d.cfg.BankerURL(), nil)
	if err != nil {
		return errResp(fmt.Sprintf("building request: %v", err))
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errResp(fmt.Sprintf("request failed: %v", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errResp(fmt.Sprintf("reading response: %v", err))
	}
	if resp.StatusCode != http.StatusOK {
		return errResp(fmt.Sprintf("banker returned %d: %s", resp.StatusCode, string(body)))
	}
	return okResp(json.RawMessage(body))
}

func (s *Server) handleCancelOnDisconnect(ctx context.Context, p protocol.CancelOnDisconnectParams) protocol.Response {
	cmd := map[string]any{
		"type":   "cancel_on_disconnect",
		"params": map[string]any{"cancel_on_disconnect": p.Enable},
	}
	data, err := s.d.trade.Send(ctx, cmd, "ack", "")
	if err != nil {
		return errResp(err.Error())
	}
	return okResp(data)
}

// ---- Helpers ----

func newClientOrderID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func okResp(data json.RawMessage) protocol.Response {
	return protocol.Response{OK: true, Data: data}
}

func errResp(msg string) protocol.Response {
	return protocol.Response{OK: false, Error: msg}
}

func writeResponse(conn net.Conn, resp protocol.Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(b)
	return err
}

func writeEvent(conn net.Conn, evt protocol.Event) error {
	b, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(b)
	return err
}
