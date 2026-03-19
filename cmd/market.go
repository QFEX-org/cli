package cmd

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var symbolsCmd = &cobra.Command{
	Use:   "symbols",
	Short: "List all active trading symbols",
	Run: func(cmd *cobra.Command, args []string) {
		for _, s := range fetchSymbols() {
			fmt.Println(s)
		}
	},
}

var marketCmd = &cobra.Command{
	Use:   "market",
	Short: "Market data commands",
}

var (
	marketDepth    int
	marketLimit    int
	candleInterval string

	// REST-only flags
	marketFrom       string
	marketTo         string
	marketInterval   string
	marketResolution string
	marketTicker     string
	marketTime       string
	marketStart      string
	marketEnd        string
	marketIntervalM  int64
)

var bboCmd = &cobra.Command{
	Use:               "bbo <symbol>",
	Short:             "Get best bid and offer for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetBBO, protocol.GetBBOParams{Symbol: args[0]})
	},
}

var orderbookCmd = &cobra.Command{
	Use:   "orderbook <symbol>",
	Short:             "Get the order book for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetOrderBook, protocol.GetOrderBookParams{
			Symbol: args[0],
			Depth:  marketDepth,
		})
	},
}

var tradesCmd = &cobra.Command{
	Use:               "trades <symbol>",
	Short:             "Get recent public trades for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
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
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
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
	Use:               "mark-price <symbol>",
	Short:             "Get the mark price for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetMarkPrice, protocol.GetMarkPriceParams{Symbol: args[0]})
	},
}

var fundingRateCmd = &cobra.Command{
	Use:               "funding-rate <symbol>",
	Short:             "Get the funding rate for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetFundingRate, protocol.GetFundingRateParams{Symbol: args[0]})
	},
}

var openInterestCmd = &cobra.Command{
	Use:               "open-interest <symbol>",
	Short:             "Get open interest for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetOpenInterest, protocol.GetOpenInterestParams{Symbol: args[0]})
	},
}

var refdataCmd = &cobra.Command{
	Use:   "refdata",
	Short: "Get symbol reference data",
	Long:  `Get reference data for all symbols or a specific ticker.`,
	Run: func(cmd *cobra.Command, args []string) {
		params := url.Values{}
		if marketTicker != "" {
			params.Set("ticker", marketTicker)
		}
		printResult(apiGet("/refdata", params, false))
	},
}

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Get symbol metrics (mark price, volume, open interest)",
	Run: func(cmd *cobra.Command, args []string) {
		printResult(apiGet("/symbols/metrics", nil, false))
	},
}

var candlesHistoryCmd = &cobra.Command{
	Use:               "candles-history <symbol>",
	Short:             "Get OHLCV candle history for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		if marketResolution == "" || marketFrom == "" || marketTo == "" {
			return fmt.Errorf("required: --resolution, --from, --to")
		}
		params := url.Values{}
		params.Set("resolution", marketResolution)
		params.Set("fromISO", marketFrom)
		params.Set("toISO", marketTo)
		printResult(apiGet("/candles/"+args[0], params, false))
		return nil
	},
}

var fundingHistoryCmd = &cobra.Command{
	Use:               "funding-history <symbol>",
	Short:             "Get historic funding rates for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		if marketIntervalM == 0 || marketFrom == "" || marketTo == "" {
			return fmt.Errorf("required: --interval, --from, --to")
		}
		params := url.Values{}
		params.Set("intervalMinutes", strconv.FormatInt(marketIntervalM, 10))
		params.Set("fromISO", marketFrom)
		params.Set("toISO", marketTo)
		printResult(apiGet("/funding/"+args[0], params, false))
		return nil
	},
}

var oiHistoryCmd = &cobra.Command{
	Use:               "oi-history <symbol>",
	Short:             "Get open interest history for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		if marketIntervalM == 0 || marketFrom == "" || marketTo == "" {
			return fmt.Errorf("required: --interval, --from, --to")
		}
		params := url.Values{}
		params.Set("intervalMinutes", strconv.FormatInt(marketIntervalM, 10))
		params.Set("fromISO", marketFrom)
		params.Set("toISO", marketTo)
		printResult(apiGet("/open-interest/"+args[0], params, false))
		return nil
	},
}

var settlementCalendarCmd = &cobra.Command{
	Use:   "settlement-calendar",
	Short: "Get settlement calendar entries",
	Run: func(cmd *cobra.Command, args []string) {
		params := url.Values{}
		if marketTicker != "" {
			params.Set("symbol", marketTicker)
		}
		if marketTime != "" {
			params.Set("time", marketTime)
		}
		printResult(apiGet("/settlement-calendar", params, false))
	},
}

var settlementPricesCmd = &cobra.Command{
	Use:   "settlement-prices",
	Short: "Get settlement prices",
	Run: func(cmd *cobra.Command, args []string) {
		params := url.Values{}
		if marketTicker != "" {
			params.Set("symbol", marketTicker)
		}
		if marketStart != "" {
			params.Set("start", marketStart)
		}
		if marketEnd != "" {
			params.Set("end", marketEnd)
		}
		if marketLimit > 0 {
			params.Set("limit", strconv.Itoa(marketLimit))
		}
		printResult(apiGet("/settlement-prices", params, false))
	},
}

var longShortCmd = &cobra.Command{
	Use:               "long-short <symbol>",
	Short:             "Get long/short user ratio history for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		if marketInterval == "" || marketFrom == "" || marketTo == "" {
			return fmt.Errorf("required: --interval, --from, --to")
		}
		params := url.Values{}
		params.Set("interval", marketInterval)
		params.Set("fromISO", marketFrom)
		params.Set("toISO", marketTo)
		printResult(apiGet("/symbol-long-short/"+args[0], params, false))
		return nil
	},
}

var takerVolumeCmd = &cobra.Command{
	Use:               "taker-volume <symbol>",
	Short:             "Get taker volume history for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		if marketIntervalM == 0 || marketFrom == "" || marketTo == "" {
			return fmt.Errorf("required: --interval, --from, --to")
		}
		params := url.Values{}
		params.Set("intervalMinutes", strconv.FormatInt(marketIntervalM, 10))
		params.Set("fromISO", marketFrom)
		params.Set("toISO", marketTo)
		printResult(apiGet("/taker-volume/"+args[0], params, false))
		return nil
	},
}

var underlierCmd = &cobra.Command{
	Use:               "underlier <symbol>",
	Short:             "Get the current underlier price for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		sendAndPrint(protocol.CmdGetUnderlierPrice, protocol.GetUnderlierPriceParams{Symbol: args[0]})
	},
}

var underlierHistoryCmd = &cobra.Command{
	Use:               "underlier-history <symbol>",
	Short:             "Get underlier OHLC history for a symbol",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: symbolCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		if marketInterval == "" || marketFrom == "" || marketTo == "" {
			return fmt.Errorf("required: --interval, --from, --to")
		}
		params := url.Values{}
		params.Set("interval", marketInterval)
		params.Set("fromISO", marketFrom)
		params.Set("toISO", marketTo)
		printResult(apiGet("/underlier/"+args[0], params, false))
		return nil
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

	marketCmd.AddCommand(symbolsCmd)

	// REST commands (no daemon required)
	marketCmd.AddCommand(refdataCmd)
	marketCmd.AddCommand(metricsCmd)
	marketCmd.AddCommand(candlesHistoryCmd)
	marketCmd.AddCommand(fundingHistoryCmd)
	marketCmd.AddCommand(oiHistoryCmd)
	marketCmd.AddCommand(settlementCalendarCmd)
	marketCmd.AddCommand(settlementPricesCmd)
	marketCmd.AddCommand(longShortCmd)
	marketCmd.AddCommand(takerVolumeCmd)
	marketCmd.AddCommand(underlierCmd)
	marketCmd.AddCommand(underlierHistoryCmd)

	orderbookCmd.Flags().IntVar(&marketDepth, "depth", 0, "Number of levels to show (0 = all)")
	tradesCmd.Flags().IntVar(&marketLimit, "limit", 20, "Number of trades to show")
	candlesCmd.Flags().StringVar(&candleInterval, "interval", "", "Candle interval (1MIN, 5MINS, 15MINS, 1HOUR, 4HOURS, 1DAY)")

	refdataCmd.Flags().StringVar(&marketTicker, "ticker", "", "Filter by ticker (e.g. AAPL-USD)")

	candlesHistoryCmd.Flags().StringVar(&marketResolution, "resolution", "", "Candle resolution (e.g. 1MIN, 1HOUR, 1DAY)")
	candlesHistoryCmd.Flags().StringVar(&marketFrom, "from", "", "Start time in ISO 8601 (e.g. 2024-01-01T00:00:00Z)")
	candlesHistoryCmd.Flags().StringVar(&marketTo, "to", "", "End time in ISO 8601")

	fundingHistoryCmd.Flags().Int64Var(&marketIntervalM, "interval", 0, "Interval in minutes")
	fundingHistoryCmd.Flags().StringVar(&marketFrom, "from", "", "Start time in ISO 8601")
	fundingHistoryCmd.Flags().StringVar(&marketTo, "to", "", "End time in ISO 8601")

	oiHistoryCmd.Flags().Int64Var(&marketIntervalM, "interval", 0, "Interval in minutes")
	oiHistoryCmd.Flags().StringVar(&marketFrom, "from", "", "Start time in ISO 8601")
	oiHistoryCmd.Flags().StringVar(&marketTo, "to", "", "End time in ISO 8601")

	settlementCalendarCmd.Flags().StringVar(&marketTicker, "symbol", "", "Filter by symbol")
	settlementCalendarCmd.Flags().StringVar(&marketTime, "time", "", "Query timestamp in ISO 8601")

	settlementPricesCmd.Flags().StringVar(&marketTicker, "symbol", "", "Filter by symbol")
	settlementPricesCmd.Flags().StringVar(&marketStart, "start", "", "Start time in ISO 8601")
	settlementPricesCmd.Flags().StringVar(&marketEnd, "end", "", "End time in ISO 8601")
	settlementPricesCmd.Flags().IntVar(&marketLimit, "limit", 0, "Max results (default 100, max 1000)")

	longShortCmd.Flags().StringVar(&marketInterval, "interval", "", "Time interval (e.g. 1h, 4h, 1d)")
	longShortCmd.Flags().StringVar(&marketFrom, "from", "", "Start time in ISO 8601")
	longShortCmd.Flags().StringVar(&marketTo, "to", "", "End time in ISO 8601")

	takerVolumeCmd.Flags().Int64Var(&marketIntervalM, "interval", 0, "Interval in minutes")
	takerVolumeCmd.Flags().StringVar(&marketFrom, "from", "", "Start time in ISO 8601")
	takerVolumeCmd.Flags().StringVar(&marketTo, "to", "", "End time in ISO 8601")

	underlierHistoryCmd.Flags().StringVar(&marketInterval, "interval", "", "Time interval (e.g. 1h, 4h, 1d)")
	underlierHistoryCmd.Flags().StringVar(&marketFrom, "from", "", "Start time in ISO 8601")
	underlierHistoryCmd.Flags().StringVar(&marketTo, "to", "", "End time in ISO 8601")
}
