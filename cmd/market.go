package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var marketCmd = &cobra.Command{
	Use:   "market",
	Short: "Market data commands",
}

var (
	marketDepth    int
	marketLimit    int
	candleInterval string
)

var bboCmd = &cobra.Command{
	Use:   "bbo <symbol>",
	Short: "Get best bid and offer for a symbol",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetBBO, protocol.GetBBOParams{Symbol: args[0]})
	},
}

var orderbookCmd = &cobra.Command{
	Use:   "orderbook <symbol>",
	Short: "Get the order book for a symbol",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetOrderBook, protocol.GetOrderBookParams{
			Symbol: args[0],
			Depth:  marketDepth,
		})
	},
}

var tradesCmd = &cobra.Command{
	Use:   "trades <symbol>",
	Short: "Get recent public trades for a symbol",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetTrades, protocol.GetTradesParams{
			Symbol: args[0],
			Limit:  marketLimit,
		})
	},
}

var candlesCmd = &cobra.Command{
	Use:   "candles <symbol>",
	Short: "Get candles for a symbol",
	Long: `Get the latest candle for a symbol at the specified interval.

Available intervals: 1MIN, 5MINS, 15MINS, 1HOUR, 4HOURS, 1DAY`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		if candleInterval == "" {
			fmt.Println("Error: --interval is required")
			return
		}
		sendAndPrint(protocol.CmdGetCandles, protocol.GetCandlesParams{
			Symbol:   args[0],
			Interval: candleInterval,
		})
	},
}

var markPriceCmd = &cobra.Command{
	Use:   "mark-price <symbol>",
	Short: "Get the mark price for a symbol",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetMarkPrice, protocol.GetMarkPriceParams{Symbol: args[0]})
	},
}

var fundingRateCmd = &cobra.Command{
	Use:   "funding-rate <symbol>",
	Short: "Get the funding rate for a symbol",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetFundingRate, protocol.GetFundingRateParams{Symbol: args[0]})
	},
}

var openInterestCmd = &cobra.Command{
	Use:   "open-interest <symbol>",
	Short: "Get open interest for a symbol",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetOpenInterest, protocol.GetOpenInterestParams{Symbol: args[0]})
	},
}

func init() {
	rootCmd.AddCommand(marketCmd)
	marketCmd.AddCommand(bboCmd)
	marketCmd.AddCommand(orderbookCmd)
	marketCmd.AddCommand(tradesCmd)
	marketCmd.AddCommand(candlesCmd)
	marketCmd.AddCommand(markPriceCmd)
	marketCmd.AddCommand(fundingRateCmd)
	marketCmd.AddCommand(openInterestCmd)

	orderbookCmd.Flags().IntVar(&marketDepth, "depth", 0, "Number of levels to show (0 = all)")
	tradesCmd.Flags().IntVar(&marketLimit, "limit", 20, "Number of trades to show")
	candlesCmd.Flags().StringVar(&candleInterval, "interval", "", "Candle interval (1MIN, 5MINS, 15MINS, 1HOUR, 4HOURS, 1DAY)")
}
