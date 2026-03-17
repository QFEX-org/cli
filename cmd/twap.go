package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var twapCmd = &cobra.Command{
	Use:   "twap",
	Short: "TWAP order commands",
}

var (
	twapSymbol       string
	twapSide         string
	twapQty          float64
	twapNumOrders    int
	twapInterval     int
	twapReduceOnly   bool
	twapClientID     string
	stopOrderID      string
	stopSymbol       string
	stopPrice        float64
	stopQty          float64
)

var addTwapCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a TWAP order",
	Long: `Create a Time-Weighted Average Price (TWAP) order.

The order will be split into num-orders market orders executed over the
specified interval.

Example:
  qfex twap add --symbol AAPL-USD --side BUY --qty 100 --num-orders 10 --interval 30`,
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()

		if twapSymbol == "" || twapSide == "" || twapQty == 0 || twapNumOrders == 0 || twapInterval == 0 {
			return fmt.Errorf("required: --symbol, --side, --qty, --num-orders, --interval")
		}
		sendAndPrint(protocol.CmdAddTwap, protocol.AddTwapParams{
			Symbol:            twapSymbol,
			Side:              twapSide,
			TotalQuantity:     twapQty,
			NumOrders:         twapNumOrders,
			OrderIntervalSecs: twapInterval,
			ReduceOnly:        twapReduceOnly,
			ClientTwapID:      twapClientID,
		})
		return nil
	},
}

var stopOrderCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop order management",
}

var cancelStopOrderCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a stop order",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()
		if stopOrderID == "" {
			return fmt.Errorf("required: --stop-order-id")
		}
		sendAndPrint(protocol.CmdCancelStopOrder, protocol.CancelStopOrderParams{
			StopOrderID: stopOrderID,
		})
		return nil
	},
}

var modifyStopOrderCmd = &cobra.Command{
	Use:   "modify",
	Short: "Modify a stop order",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()
		if stopOrderID == "" || stopSymbol == "" || stopPrice == 0 || stopQty == 0 {
			return fmt.Errorf("required: --stop-order-id, --symbol, --price, --qty")
		}
		sendAndPrint(protocol.CmdModifyStopOrder, protocol.ModifyStopOrderParams{
			StopOrderID: stopOrderID,
			Symbol:      stopSymbol,
			Price:       stopPrice,
			Quantity:    stopQty,
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(twapCmd)
	twapCmd.AddCommand(addTwapCmd)

	rootCmd.AddCommand(stopOrderCmd)
	stopOrderCmd.AddCommand(cancelStopOrderCmd)
	stopOrderCmd.AddCommand(modifyStopOrderCmd)

	addTwapCmd.Flags().StringVar(&twapSymbol, "symbol", "", "Trading symbol")
	addTwapCmd.Flags().StringVar(&twapSide, "side", "", "Order side: BUY or SELL")
	addTwapCmd.Flags().Float64Var(&twapQty, "qty", 0, "Total quantity to execute")
	addTwapCmd.Flags().IntVar(&twapNumOrders, "num-orders", 0, "Number of child orders")
	addTwapCmd.Flags().IntVar(&twapInterval, "interval", 0, "Interval between orders in seconds")
	addTwapCmd.Flags().BoolVar(&twapReduceOnly, "reduce-only", false, "Reduce-only mode")
	addTwapCmd.Flags().StringVar(&twapClientID, "client-twap-id", "", "Client-assigned TWAP ID")

	cancelStopOrderCmd.Flags().StringVar(&stopOrderID, "stop-order-id", "", "Stop order ID to cancel")
	modifyStopOrderCmd.Flags().StringVar(&stopOrderID, "stop-order-id", "", "Stop order ID to modify")
	modifyStopOrderCmd.Flags().StringVar(&stopSymbol, "symbol", "", "Symbol")
	modifyStopOrderCmd.Flags().Float64Var(&stopPrice, "price", 0, "New trigger price")
	modifyStopOrderCmd.Flags().Float64Var(&stopQty, "qty", 0, "New quantity")
}
