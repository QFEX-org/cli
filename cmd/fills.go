package cmd

import (
	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var fillsCmd = &cobra.Command{
	Use:   "fills",
	Short: "Fill (execution) commands",
}

var fillsLimit int

var listFillsCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent fills",
	Long:  `List recently executed fills (trades). The daemon caches the most recent 200 fills.`,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetFills, protocol.GetFillsParams{Limit: fillsLimit})
	},
}

func init() {
	rootCmd.AddCommand(fillsCmd)
	fillsCmd.AddCommand(listFillsCmd)

	listFillsCmd.Flags().IntVar(&fillsLimit, "limit", 50, "Maximum number of fills to show")
}
