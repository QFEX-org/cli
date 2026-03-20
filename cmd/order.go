package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var orderCmd = &cobra.Command{
	Use:   "order",
	Short: "Order management commands",
}

var (
	orderSymbol       string
	orderSide         string
	orderType         string
	orderTIF          string
	orderQty          float64
	orderPrice        float64
	orderTP           float64
	orderSL           float64
	orderClientID     string
	orderID           string
	orderCancelIDType string
	orderLimit        int
	orderOffset       int
	orderReduceOnly   bool
	orderWait         bool
)

var placeOrderCmd = &cobra.Command{
	Use:   "place",
	Short: "Place a new order",
	Long: `Place a new order on QFEX.

Examples:
  # Market buy
  qfex order place --symbol AAPL-USD --side BUY --type MARKET --tif IOC --qty 1

  # Limit sell
  qfex order place --symbol AAPL-USD --side SELL --type LIMIT --tif GTC --qty 1 --price 200

  # Limit with take profit and stop loss
  qfex order place --symbol AAPL-USD --side BUY --type LIMIT --tif GTC --qty 1 --price 150 --tp 180 --sl 140`,
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()

		if orderSymbol == "" || orderSide == "" || orderType == "" || orderTIF == "" || orderQty == 0 {
			return fmt.Errorf("required: --symbol, --side, --type, --tif, --qty")
		}
		if (orderType == "LIMIT" || orderType == "ALO") && orderPrice == 0 {
			return fmt.Errorf("--price is required for %s orders", orderType)
		}

		params := protocol.PlaceOrderParams{
			Symbol:        orderSymbol,
			Side:          orderSide,
			OrderType:     orderType,
			TimeInForce:   orderTIF,
			Quantity:      orderQty,
			Price:         orderPrice,
			ReduceOnly:    orderReduceOnly,
			TakeProfit:    orderTP,
			StopLoss:      orderSL,
			ClientOrderID: orderClientID,
			WaitForFinal:  orderWait,
		}
		sendAndPrint(protocol.CmdPlaceOrder, params)
		return nil
	},
}

var cancelOrderCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel an order",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()

		if orderSymbol == "" || orderID == "" {
			return fmt.Errorf("required: --symbol, --order-id")
		}
		if orderCancelIDType == "" {
			orderCancelIDType = "order_id"
		}
		sendAndPrint(protocol.CmdCancelOrder, protocol.CancelOrderParams{
			Symbol:       orderSymbol,
			OrderID:      orderID,
			CancelIDType: orderCancelIDType,
		})
		return nil
	},
}

var cancelAllCmd = &cobra.Command{
	Use:   "cancel-all",
	Short: "Cancel all open orders",
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdCancelAll, protocol.CancelAllParams{})
	},
}

var modifyOrderCmd = &cobra.Command{
	Use:   "modify",
	Short: "Modify an existing order",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()

		if orderSymbol == "" || orderID == "" || orderSide == "" || orderType == "" {
			return fmt.Errorf("required: --symbol, --order-id, --side, --type")
		}
		sendAndPrint(protocol.CmdModifyOrder, protocol.ModifyOrderParams{
			Symbol:     orderSymbol,
			OrderID:    orderID,
			Side:       orderSide,
			OrderType:  orderType,
			Price:      orderPrice,
			Quantity:   orderQty,
			TakeProfit: orderTP,
			StopLoss:   orderSL,
		})
		return nil
	},
}

var listOrdersCmd = &cobra.Command{
	Use:   "list",
	Short: "List active orders",
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetOrders, protocol.GetOrdersParams{
			Symbol: orderSymbol,
			Limit:  orderLimit,
			Offset: orderOffset,
		})
	},
}

var getOrderCmd = &cobra.Command{
	Use:   "get",
	Short: "Get order details",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()
		if orderID == "" || orderSymbol == "" {
			return fmt.Errorf("required: --order-id, --symbol")
		}
		sendAndPrint(protocol.CmdGetOrder, protocol.GetOrderParams{
			Symbol:  orderSymbol,
			OrderID: orderID,
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(orderCmd)
	orderCmd.AddCommand(placeOrderCmd)
	orderCmd.AddCommand(cancelOrderCmd)
	orderCmd.AddCommand(cancelAllCmd)
	orderCmd.AddCommand(modifyOrderCmd)
	orderCmd.AddCommand(listOrdersCmd)
	orderCmd.AddCommand(getOrderCmd)

	// Shared flags on order subcommands
	for _, cmd := range []*cobra.Command{placeOrderCmd, cancelOrderCmd, modifyOrderCmd, listOrdersCmd, getOrderCmd} {
		cmd.Flags().StringVar(&orderSymbol, "symbol", "", "Trading symbol (e.g. AAPL-USD)")
	}

	// Place order flags
	placeOrderCmd.Flags().StringVar(&orderSide, "side", "", "Order side: BUY or SELL")
	placeOrderCmd.Flags().StringVar(&orderType, "type", "LIMIT", "Order type: LIMIT, MARKET, ALO")
	placeOrderCmd.Flags().StringVar(&orderTIF, "tif", "GTC", "Time in force: GTC, IOC, FOK")
	placeOrderCmd.Flags().Float64Var(&orderQty, "qty", 0, "Order quantity")
	placeOrderCmd.Flags().Float64Var(&orderPrice, "price", 0, "Limit price")
	placeOrderCmd.Flags().Float64Var(&orderTP, "tp", 0, "Take profit price")
	placeOrderCmd.Flags().Float64Var(&orderSL, "sl", 0, "Stop loss price")
	placeOrderCmd.Flags().BoolVar(&orderReduceOnly, "reduce-only", false, "Place the order in reduce-only mode")
	placeOrderCmd.Flags().StringVar(&orderClientID, "client-order-id", "", "Client-assigned order ID")
	placeOrderCmd.Flags().BoolVar(&orderWait, "wait", false, "Wait for final status after ACK (FILLED, CANCELLED, REJECTED, etc.)")

	// Cancel order flags
	cancelOrderCmd.Flags().StringVar(&orderID, "order-id", "", "Order ID to cancel")
	cancelOrderCmd.Flags().StringVar(&orderCancelIDType, "id-type", "order_id", "ID type: order_id, client_order_id, twap_id, client_twap_id")

	// Modify order flags
	modifyOrderCmd.Flags().StringVar(&orderID, "order-id", "", "Order ID to modify")
	modifyOrderCmd.Flags().StringVar(&orderSide, "side", "", "Current side of the order: BUY or SELL")
	modifyOrderCmd.Flags().StringVar(&orderType, "type", "", "Current type of the order: LIMIT, MARKET, ALO")
	modifyOrderCmd.Flags().Float64Var(&orderQty, "qty", 0, "New quantity")
	modifyOrderCmd.Flags().Float64Var(&orderPrice, "price", 0, "New limit price")
	modifyOrderCmd.Flags().Float64Var(&orderTP, "tp", 0, "New take profit price")
	modifyOrderCmd.Flags().Float64Var(&orderSL, "sl", 0, "New stop loss price")

	// Get order flags
	getOrderCmd.Flags().StringVar(&orderID, "order-id", "", "Order ID")

	// List orders flags
	listOrdersCmd.Flags().IntVar(&orderLimit, "limit", 50, "Maximum number of orders")
	listOrdersCmd.Flags().IntVar(&orderOffset, "offset", 0, "Offset for pagination")
}
