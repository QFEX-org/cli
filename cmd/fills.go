package cmd

import (
	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var fillsCmd = &cobra.Command{
	Use:   "fills",
	Short: "Your private fill (execution) history",
}

var fillsLimit int
var fillsSymbol string

var listFillsCmd = &cobra.Command{
	Use:   "list",
	Short: "List your recent private fills",
	Long:  `List your recently executed fills (private executions from the Trade WS). The daemon caches the most recent 200 fills. For public trade data, use 'qfex watch trades <symbol>'.`,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetFills, protocol.GetFillsParams{Limit: fillsLimit, Symbol: fillsSymbol})
	},
}

func init() {
	rootCmd.AddCommand(fillsCmd)
	fillsCmd.AddCommand(listFillsCmd)

	listFillsCmd.Flags().IntVar(&fillsLimit, "limit", 50, "Maximum number of fills to show")
	listFillsCmd.Flags().StringVar(&fillsSymbol, "symbol", "", "Filter by symbol (e.g. AAPL-USD)")
}
