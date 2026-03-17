package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/qfex/cli/internal/config"
	"github.com/qfex/cli/internal/protocol"
)

// Daemon coordinates the MDS/Trade WS connections and the IPC server.
type Daemon struct {
	cfg   *config.Config
	state *State
	mds   *MDSWS
	trade *TradeWS
	srv   *Server
	log   *log.Logger
}

func New(cfg *config.Config, socketPath string) *Daemon {
	logger := log.New(os.Stderr, "[qfex-daemon] ", log.LstdFlags)
	state := newState()
	mds := newMDSWS(cfg.MDS(), state, logger)
	trade := newTradeWS(cfg.TradeWS(), cfg, state, logger)
	d := &Daemon{cfg: cfg, state: state, mds: mds, trade: trade, log: logger}
	d.srv = newServer(socketPath, d, logger)
	return d
}

// Run starts all subsystems and blocks until shutdown.
func (d *Daemon) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle SIGTERM / SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-sigCh:
			d.log.Printf("Daemon: received signal, shutting down")
			cancel()
		case <-ctx.Done():
		}
	}()

	// Write PID file
	if err := writePID(); err != nil {
		d.log.Printf("Daemon: warning: could not write PID file: %v", err)
	}
	defer removePID()

	d.log.Printf("Daemon: starting")

	// Start MDS WebSocket
	go d.mds.Run(ctx)

	// Start Trade WebSocket (only if credentials configured)
	if d.cfg.HasCredentials() {
		go d.trade.Run(ctx)
	} else {
		d.log.Printf("Daemon: no API credentials configured; trade features disabled")
	}

	// Run IPC server (blocks)
	return d.srv.Run(ctx)
}

// ensureMDSSubscription subscribes to the appropriate MDS channel for the given stream.
func (d *Daemon) ensureMDSSubscription(stream, symbol, interval string) error {
	switch stream {
	case protocol.StreamBBO:
		return d.mds.Subscribe("bbo", []string{symbol})
	case protocol.StreamOrderBook:
		return d.mds.Subscribe("level2", []string{symbol})
	case protocol.StreamTrades:
		return d.mds.Subscribe("trade", []string{symbol})
	case protocol.StreamMarkPrice:
		return d.mds.Subscribe("mark_price", []string{symbol})
	case protocol.StreamFundingRate:
		return d.mds.Subscribe("funding", []string{symbol})
	case protocol.StreamOpenInterest:
		return d.mds.Subscribe("open_interest", []string{symbol})
	case protocol.StreamPositions, protocol.StreamBalance, protocol.StreamFills, protocol.StreamOrders:
		// These come from the Trade WS, no MDS subscription needed
		return nil
	default:
		return fmt.Errorf("unknown stream: %s", stream)
	}
}

// getCurrentValue returns the current cached value for a stream/symbol as JSON.
func (d *Daemon) getCurrentValue(stream, symbol, interval string) []byte {
	switch stream {
	case protocol.StreamBBO:
		if v := d.state.getBBO(symbol); v != nil {
			return mustMarshal(v)
		}
	case protocol.StreamOrderBook:
		if v := d.state.getOrderBook(symbol); v != nil {
			return mustMarshal(v)
		}
	case protocol.StreamMarkPrice:
		if v := d.state.getMarkPrice(symbol); v != nil {
			return mustMarshal(v)
		}
	case protocol.StreamFundingRate:
		if v := d.state.getFundingRate(symbol); v != nil {
			return mustMarshal(v)
		}
	case protocol.StreamBalance:
		if v := d.state.getBalance(); v != nil {
			return mustMarshal(v)
		}
	case protocol.StreamPositions:
		positions := d.state.getAllPositions()
		if len(positions) > 0 {
			return mustMarshal(positions)
		}
	}
	return nil
}

func writePID() error {
	if err := os.MkdirAll(config.DataDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(config.PIDPath(), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
}

func removePID() {
	os.Remove(config.PIDPath())
}
