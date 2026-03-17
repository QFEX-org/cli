package cmd

import (
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Historic data commands",
}

var (
	histSymbol        string
	histClientOrderID string
	histOrderID       string
	histStart         string
	histEnd           string
	histLimit         int64
	histOffset        int64
	histOrderType     string
	histDirection     string
)

var historicOrdersCmd = &cobra.Command{
	Use:   "orders",
	Short: "Get historic (filled/closed) orders",
	Run: func(cmd *cobra.Command, args []string) {
		params := url.Values{}
		if histSymbol != "" {
			params.Set("symbol", histSymbol)
		}
		if histClientOrderID != "" {
			params.Set("client_order_id", histClientOrderID)
		}
		if histStart != "" {
			params.Set("start", histStart)
		}
		if histEnd != "" {
			params.Set("end", histEnd)
		}
		if histLimit > 0 {
			params.Set("limit", strconv.FormatInt(histLimit, 10))
		}
		if histOffset > 0 {
			params.Set("offset", strconv.FormatInt(histOffset, 10))
		}
		printResult(apiGet("/user/historic-orders", params, true))
	},
}

var historicTwapsCmd = &cobra.Command{
	Use:   "twaps",
	Short: "Get historic TWAP orders",
	Run: func(cmd *cobra.Command, args []string) {
		params := url.Values{}
		if histSymbol != "" {
			params.Set("symbol", histSymbol)
		}
		if histLimit > 0 {
			params.Set("limit", strconv.FormatInt(histLimit, 10))
		}
		printResult(apiGet("/user/historic-twaps", params, true))
	},
}

var historicTradesCmd = &cobra.Command{
	Use:   "trades",
	Short: "Get trade history",
	Run: func(cmd *cobra.Command, args []string) {
		params := url.Values{}
		if histOrderID != "" {
			params.Set("exchange_order_id", histOrderID)
		}
		if histSymbol != "" {
			params.Set("symbol", histSymbol)
		}
		if histStart != "" {
			params.Set("start", histStart)
		}
		if histEnd != "" {
			params.Set("end", histEnd)
		}
		if histOrderType != "" {
			params.Set("ordertype", histOrderType)
		}
		if histDirection != "" {
			params.Set("orderdirection", histDirection)
		}
		if histLimit > 0 {
			params.Set("limit", strconv.FormatInt(histLimit, 10))
		}
		if histOffset > 0 {
			params.Set("offset", strconv.FormatInt(histOffset, 10))
		}
		printResult(apiGet("/user/trade", params, true))
	},
}

var historicOrdersCSVCmd = &cobra.Command{
	Use:   "orders-csv",
	Short: "Stream historic orders as CSV",
	Long:  `Streams all historic orders as CSV to stdout. Redirect to a file: qfex history orders-csv > orders.csv`,
	Run: func(cmd *cobra.Command, args []string) {
		apiStream("/historic-orders-csv", nil, true)
	},
}

var historicTwapsCSVCmd = &cobra.Command{
	Use:   "twaps-csv",
	Short: "Stream historic TWAP orders as CSV",
	Long:  `Streams all historic TWAP orders as CSV to stdout. Redirect to a file: qfex history twaps-csv > twaps.csv`,
	Run: func(cmd *cobra.Command, args []string) {
		params := url.Values{}
		if histSymbol != "" {
			params.Set("symbol", histSymbol)
		}
		apiStream("/historic-twaps-csv", params, true)
	},
}

var historicTradesCSVCmd = &cobra.Command{
	Use:   "trades-csv",
	Short: "Stream trade history as CSV",
	Long:  `Streams all trades as CSV to stdout. Redirect to a file: qfex history trades-csv > trades.csv`,
	Run: func(cmd *cobra.Command, args []string) {
		apiStream("/trades-csv", nil, true)
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.AddCommand(historicOrdersCmd)
	historyCmd.AddCommand(historicTwapsCmd)
	historyCmd.AddCommand(historicTradesCmd)
	historyCmd.AddCommand(historicOrdersCSVCmd)
	historyCmd.AddCommand(historicTwapsCSVCmd)
	historyCmd.AddCommand(historicTradesCSVCmd)

	// Shared flags
	for _, cmd := range []*cobra.Command{historicOrdersCmd, historicTwapsCmd, historicTradesCmd, historicTwapsCSVCmd} {
		cmd.Flags().StringVar(&histSymbol, "symbol", "", "Filter by symbol (e.g. AAPL-USD)")
	}
	for _, cmd := range []*cobra.Command{historicOrdersCmd, historicTradesCmd} {
		cmd.Flags().StringVar(&histStart, "start", "", "Start time in ISO 8601")
		cmd.Flags().StringVar(&histEnd, "end", "", "End time in ISO 8601")
		cmd.Flags().Int64Var(&histLimit, "limit", 100, "Max results (default 100, max 1000)")
		cmd.Flags().Int64Var(&histOffset, "offset", 0, "Pagination offset")
	}

	historicOrdersCmd.Flags().StringVar(&histClientOrderID, "client-order-id", "", "Filter by client order ID")

	historicTwapsCmd.Flags().Int64Var(&histLimit, "limit", 100, "Max results (default 100, max 1000)")

	historicTradesCmd.Flags().StringVar(&histOrderID, "order-id", "", "Filter by exchange order ID")
	historicTradesCmd.Flags().StringVar(&histOrderType, "order-type", "", "Filter by order type")
	historicTradesCmd.Flags().StringVar(&histDirection, "direction", "", "Filter by direction (BUY/SELL)")
}
