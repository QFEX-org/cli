package cmd

import (
	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var tradesUserCmd = &cobra.Command{
	Use:   "trades",
	Short: "User trade history commands",
}

var (
	tradesLimit   int
	tradesOffset  int
	tradesOrderID string
	tradesStartTS int64
	tradesEndTS   int64
)

var listTradesCmd = &cobra.Command{
	Use:   "list",
	Short: "List user trade history",
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetUserTrades, protocol.GetUserTradesParams{
			Limit:   tradesLimit,
			Offset:  tradesOffset,
			OrderID: tradesOrderID,
			StartTS: tradesStartTS,
			EndTS:   tradesEndTS,
		})
	},
}

func init() {
	rootCmd.AddCommand(tradesUserCmd)
	tradesUserCmd.AddCommand(listTradesCmd)

	listTradesCmd.Flags().IntVar(&tradesLimit, "limit", 50, "Maximum number of trades to show")
	listTradesCmd.Flags().IntVar(&tradesOffset, "offset", 0, "Pagination offset")
	listTradesCmd.Flags().StringVar(&tradesOrderID, "order-id", "", "Filter by order ID")
	listTradesCmd.Flags().Int64Var(&tradesStartTS, "start-ts", 0, "Start timestamp (Unix seconds)")
	listTradesCmd.Flags().Int64Var(&tradesEndTS, "end-ts", 0, "End timestamp (Unix seconds)")
}
