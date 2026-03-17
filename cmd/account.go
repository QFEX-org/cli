package cmd

import (
	"fmt"
	"net/url"
	"strconv"

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

var depositCmd = &cobra.Command{
	Use:   "deposit",
	Short: "Get the USDC deposit address",
	Long: `Fetch the wallet address to deposit USDC (Arbitrum) into your QFEX account.

Also returns available_allowance — the remaining deposit capacity on your account.`,
	Run: func(cmd *cobra.Command, args []string) {
		requireDaemon()
		requireAuth()
		sendAndPrint(protocol.CmdGetDepositAddress, nil)
	},
}

var feesCmd = &cobra.Command{
	Use:   "fees",
	Short: "Get your fee tiers",
	Run: func(cmd *cobra.Command, args []string) {
		printResult(apiGet("/user/fees", nil, true))
	},
}

var (
	pnlSymbol     string
	pnlStart      string
	pnlEnd        string
	pnlLimitHours int64
)

var pnlCmd = &cobra.Command{
	Use:   "pnl",
	Short: "Get hourly PnL approximations",
	Long:  `Get hourly PnL data. Maximum 2160 hours (90 days).`,
	Run: func(cmd *cobra.Command, args []string) {
		params := url.Values{}
		if pnlSymbol != "" {
			params.Set("symbol", pnlSymbol)
		}
		if pnlStart != "" {
			params.Set("start", pnlStart)
		}
		if pnlEnd != "" {
			params.Set("end", pnlEnd)
		}
		if pnlLimitHours > 0 {
			params.Set("limit_hours", strconv.FormatInt(pnlLimitHours, 10))
		}
		printResult(apiGet("/pnl", params, true))
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
	accountCmd.AddCommand(depositCmd)
	accountCmd.AddCommand(leverageCmd)
	accountCmd.AddCommand(cancelOnDisconnectCmd)
	accountCmd.AddCommand(feesCmd)
	accountCmd.AddCommand(pnlCmd)

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

	pnlCmd.Flags().StringVar(&pnlSymbol, "symbol", "", "Filter by symbol")
	pnlCmd.Flags().StringVar(&pnlStart, "start", "", "Start time in ISO 8601")
	pnlCmd.Flags().StringVar(&pnlEnd, "end", "", "End time in ISO 8601")
	pnlCmd.Flags().Int64Var(&pnlLimitHours, "limit-hours", 168, "Hours of history (default 168, max 2160)")
}
