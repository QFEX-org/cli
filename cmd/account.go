package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/protocol"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Account information commands",
}

var leverageCmd = &cobra.Command{
	Use:   "leverage",
	Short: "Leverage management commands",
}

var (
	levSymbol   string
	levLeverage float64
	levLimit    int
	levOffset   int
)

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Get account balance",
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetBalance, nil)
	},
}

var getLeverageCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current leverage settings",
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetLeverage, protocol.GetLeverageParams{
			Limit:  levLimit,
			Offset: levOffset,
		})
	},
}

var setLeverageCmd = &cobra.Command{
	Use:   "set",
	Short: "Set leverage for a symbol",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()
		if levSymbol == "" || levLeverage == 0 {
			return fmt.Errorf("required: --symbol, --leverage")
		}
		sendAndPrint(protocol.CmdSetLeverage, protocol.SetLeverageParams{
			Symbol:   levSymbol,
			Leverage: levLeverage,
		})
		return nil
	},
}

var getAvailableLeverageCmd = &cobra.Command{
	Use:   "available",
	Short: "Get available leverage levels",
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetAvailableLeverage, protocol.GetLeverageParams{
			Limit:  levLimit,
			Offset: levOffset,
		})
	},
}

var cancelOnDisconnectCmd = &cobra.Command{
	Use:   "cod",
	Short: "Enable or disable cancel-on-disconnect",
	Long:  `When enabled, all open orders will be cancelled if the connection drops.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		requireDaemon()
		requireAuth()
		enabled, _ := cmd.Flags().GetBool("enable")
		sendAndPrint(protocol.CmdCancelOnDisconnect, protocol.CancelOnDisconnectParams{Enable: enabled})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(accountCmd)
	accountCmd.AddCommand(balanceCmd)
	accountCmd.AddCommand(leverageCmd)
	accountCmd.AddCommand(cancelOnDisconnectCmd)

	leverageCmd.AddCommand(getLeverageCmd)
	leverageCmd.AddCommand(setLeverageCmd)
	leverageCmd.AddCommand(getAvailableLeverageCmd)

	getLeverageCmd.Flags().IntVar(&levLimit, "limit", 50, "Maximum number of results")
	getLeverageCmd.Flags().IntVar(&levOffset, "offset", 0, "Pagination offset")

	setLeverageCmd.Flags().StringVar(&levSymbol, "symbol", "", "Symbol to set leverage for")
	setLeverageCmd.Flags().Float64Var(&levLeverage, "leverage", 0, "Leverage value (e.g. 10)")

	getAvailableLeverageCmd.Flags().IntVar(&levLimit, "limit", 50, "Maximum number of results")
	getAvailableLeverageCmd.Flags().IntVar(&levOffset, "offset", 0, "Pagination offset")

	cancelOnDisconnectCmd.Flags().Bool("enable", true, "Enable cancel-on-disconnect (use --enable=false to disable)")
}
