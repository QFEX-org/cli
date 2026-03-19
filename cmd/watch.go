package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch live streaming data",
	Long:  `Subscribe to real-time data streams. Press Ctrl+C to stop.`,
}

var watchCandleInterval string

func init() {
	rootCmd.AddCommand(watchCmd)

	watchCmd.AddCommand(makeWatchSymbolCmd("bbo", "Watch best bid/offer", protocol.StreamBBO))
	watchCmd.AddCommand(makeWatchSymbolCmd("orderbook", "Watch order book updates", protocol.StreamOrderBook))
	watchCmd.AddCommand(makeWatchSymbolCmd("trades", "Watch public trades", protocol.StreamTrades))
	watchCmd.AddCommand(makeWatchSymbolCmd("mark-price", "Watch mark price", protocol.StreamMarkPrice))
	watchCmd.AddCommand(makeWatchSymbolCmd("funding-rate", "Watch funding rate", protocol.StreamFundingRate))
	watchCmd.AddCommand(makeWatchSymbolCmd("underlier", "Watch underlier price", protocol.StreamUnderlier))

	watchCmd.AddCommand(makeWatchAccountCmd("positions", "Watch position updates", protocol.StreamPositions))
	watchCmd.AddCommand(makeWatchAccountCmd("balance", "Watch balance updates", protocol.StreamBalance))
	watchCmd.AddCommand(makeWatchAccountCmd("fills", "Watch fill updates", protocol.StreamFills))
	watchCmd.AddCommand(makeWatchAccountCmd("orders", "Watch order updates", protocol.StreamOrders))

	candleWatchCmd := &cobra.Command{
		Use:               "candles <symbol>",
		Short:             "Watch candle updates",
		ValidArgsFunction: symbolCompletion,
		Long: `Watch real-time candle (OHLCV) updates for a symbol.

Available intervals: 1MIN, 5MINS, 15MINS, 1HOUR, 4HOURS, 1DAY
If --interval is omitted, all intervals are streamed.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			requireDaemon()
			runWatch(protocol.WatchParams{
				Stream:   protocol.StreamCandles,
				Symbol:   args[0],
				Interval: watchCandleInterval,
			})
		},
	}
	candleWatchCmd.Flags().StringVar(&watchCandleInterval, "interval", "", "Candle interval (1MIN, 5MINS, 15MINS, 1HOUR, 4HOURS, 1DAY)")
	watchCmd.AddCommand(candleWatchCmd)
}

func makeWatchSymbolCmd(use, short, stream string) *cobra.Command {
	return &cobra.Command{
		Use:               use + " <symbol>",
		Short:             short,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: symbolCompletion,
		Run: func(cmd *cobra.Command, args []string) {
			requireDaemon()
			runWatch(protocol.WatchParams{Stream: stream, Symbol: args[0]})
		},
	}
}

func makeWatchAccountCmd(use, short, stream string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Run: func(cmd *cobra.Command, args []string) {
			requireDaemon()
			requireAuth()
			runWatch(protocol.WatchParams{Stream: stream})
		},
	}
}

func runWatch(params protocol.WatchParams) {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	fmt.Fprintf(os.Stderr, "Watching %s", params.Stream)
	if params.Symbol != "" {
		fmt.Fprintf(os.Stderr, " for %s", params.Symbol)
	}
	fmt.Fprintln(os.Stderr, " (Ctrl+C to stop)")

	err := cli.Watch(ctx, params, func(evt protocol.Event) error {
		var v any
		if err := json.Unmarshal(evt.Data, &v); err != nil {
			fmt.Println(string(evt.Data))
			return nil
		}
		out, _ := json.Marshal(v)
		fmt.Println(string(out))
		return nil
	})

	if err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
