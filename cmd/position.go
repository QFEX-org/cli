package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var positionCmd = &cobra.Command{
	Use:   "position",
	Short: "Position management commands",
}

var (
	posSymbol        string
	posClientOrderID string
)

var listPositionsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all open positions",
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetPositions, nil)
	},
}

var closePositionCmd = &cobra.Command{
	Use:   "close",
	Short: "Close an open position",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()
		if posSymbol == "" {
			return fmt.Errorf("required: --symbol")
		}
		sendAndPrint(protocol.CmdClosePosition, protocol.ClosePositionParams{
			Symbol:        posSymbol,
			ClientOrderID: posClientOrderID,
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(positionCmd)
	positionCmd.AddCommand(listPositionsCmd)
	positionCmd.AddCommand(closePositionCmd)

	closePositionCmd.Flags().StringVar(&posSymbol, "symbol", "", "Symbol of position to close")
	closePositionCmd.Flags().StringVar(&posClientOrderID, "client-order-id", "", "Client order ID for the closing order")
}
