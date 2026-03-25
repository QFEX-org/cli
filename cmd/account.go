package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/config"
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

var subaccountsCmd = &cobra.Command{
	Use:   "subaccounts",
	Short: "Manage account subaccounts",
}

var (
	levSymbol   string
	levLeverage float64
	levLimit    int
	levOffset   int
)

var (
	subaccountTransferFrom   string
	subaccountTransferTo     string
	subaccountTransferAmount float64
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
		printResult(apiGetURL(cfg.BankerURL(), true))
	},
}

var feesCmd = &cobra.Command{
	Use:   "fees",
	Short: "Get your fee tiers",
	Long:  `Get your maker/taker fee rates per symbol. Rates are shown as percentages (e.g. -0.01 = -0.01% maker rebate).`,
	Run: func(cmd *cobra.Command, args []string) {
		raw := apiGet("/user/fees", nil, true)
		var resp struct {
			Fees map[string]struct {
				MakerFee int64 `json:"maker_fee"`
				TakerFee int64 `json:"taker_fee"`
			} `json:"fees"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			printResult(raw)
			return
		}
		type feeEntry struct {
			MakerFee float64 `json:"maker_fee_pct"`
			TakerFee float64 `json:"taker_fee_pct"`
		}
		out := make(map[string]feeEntry, len(resp.Fees))
		for symbol, f := range resp.Fees {
			out[symbol] = feeEntry{
				MakerFee: float64(f.MakerFee) / 1e6,
				TakerFee: float64(f.TakerFee) / 1e6,
			}
		}
		printJSON(mustMarshalJSON(out))
	},
}

func mustMarshalJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return b
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

var listSubaccountsCmd = &cobra.Command{
	Use:   "list",
	Short: "List your subaccounts",
	Run: func(cmd *cobra.Command, args []string) {
		printResult(apiGetWithAccountSelection("/user/subaccounts", nil, true, false))
	},
}

var createSubaccountCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new subaccount",
	Run: func(cmd *cobra.Command, args []string) {
		printResult(apiPostWithQueryAndAccountSelection("/user/subaccounts", nil, struct{}{}, false))
	},
}

var transferSubaccountCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfer funds between your accounts and subaccounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateSubaccountTransferInput(subaccountTransferFrom, subaccountTransferTo, subaccountTransferAmount); err != nil {
			return err
		}
		params := url.Values{}
		params.Set("src_account_id", subaccountTransferFrom)
		params.Set("dst_account_id", subaccountTransferTo)
		printResult(apiPostWithQueryAndAccountSelection("/user/transfer", params, map[string]any{
			"amount": subaccountTransferAmount,
		}, false))
		return nil
	},
}

var currentSubaccountCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the currently selected subaccount",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(currentSubaccountOutput())
	},
}

var selectSubaccountCmd = &cobra.Command{
	Use:   "select <account-id|primary>",
	Short: "Select the active subaccount and restart the daemon if needed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		selected, err := normalizeSelectedSubaccount(args[0])
		if err != nil {
			return err
		}

		if selected != "" {
			subaccounts, err := fetchSubaccountIDs()
			if err != nil {
				return err
			}
			if err := validateSelectableSubaccount(selected, subaccounts); err != nil {
				return err
			}
		}

		if cfg.SelectedSubaccount == selected {
			fmt.Print(currentSubaccountOutput())
			return nil
		}

		cfg.SelectedSubaccount = selected
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		if err := restartDaemonIfRunning(cmd); err != nil {
			return err
		}
		fmt.Print(currentSubaccountOutput())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(accountCmd)
	accountCmd.AddCommand(balanceCmd)
	accountCmd.AddCommand(depositCmd)
	accountCmd.AddCommand(leverageCmd)
	accountCmd.AddCommand(subaccountsCmd)
	accountCmd.AddCommand(cancelOnDisconnectCmd)
	accountCmd.AddCommand(feesCmd)
	accountCmd.AddCommand(pnlCmd)

	leverageCmd.AddCommand(getLeverageCmd)
	leverageCmd.AddCommand(setLeverageCmd)
	leverageCmd.AddCommand(getAvailableLeverageCmd)
	subaccountsCmd.AddCommand(listSubaccountsCmd)
	subaccountsCmd.AddCommand(createSubaccountCmd)
	subaccountsCmd.AddCommand(transferSubaccountCmd)
	subaccountsCmd.AddCommand(currentSubaccountCmd)
	subaccountsCmd.AddCommand(selectSubaccountCmd)

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

	transferSubaccountCmd.Flags().StringVar(&subaccountTransferFrom, "from", "", "Source account ID")
	transferSubaccountCmd.Flags().StringVar(&subaccountTransferTo, "to", "", "Destination account ID")
	transferSubaccountCmd.Flags().Float64Var(&subaccountTransferAmount, "amount", 0, "Amount to transfer")
}

type subaccountsResponse struct {
	Body struct {
		AccountIDs []string `json:"account_ids"`
	} `json:"body"`
}

func fetchSubaccountIDs() ([]string, error) {
	raw := apiGetWithAccountSelection("/user/subaccounts", nil, true, false)
	var resp subaccountsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse subaccounts response: %w", err)
	}
	if resp.Body.AccountIDs == nil {
		return []string{}, nil
	}
	return resp.Body.AccountIDs, nil
}

func currentSubaccountOutput() string {
	if cfg == nil || !cfg.HasSelectedSubaccount() {
		return "primary\n"
	}
	return cfg.SelectedSubaccount + "\n"
}

func validateSubaccountTransferInput(from, to string, amount float64) error {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" || amount <= 0 {
		return fmt.Errorf("required: --from, --to, --amount")
	}
	return nil
}

func validateSelectableSubaccount(selected string, subaccounts []string) error {
	for _, subaccount := range subaccounts {
		if subaccount == selected {
			return nil
		}
	}
	return fmt.Errorf("unknown subaccount: %s", selected)
}

func normalizeSelectedSubaccount(value string) (string, error) {
	selected := strings.TrimSpace(value)
	if selected == "" {
		return "", fmt.Errorf("account ID cannot be empty")
	}
	if selected == "primary" {
		return "", nil
	}
	return selected, nil
}
