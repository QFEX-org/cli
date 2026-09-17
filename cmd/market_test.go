package cmd

import (
	"strconv"
	"testing"
)

// A flag variable shared by several commands keeps only the default of the last
// registration, so each command must own the variable behind its own default.
func TestMarketLimitFlagDefaults(t *testing.T) {
	cases := []struct {
		name string
		got  int
		flag string
	}{
		{"trades", marketTradesLimit, tradesCmd.Flags().Lookup("limit").DefValue},
		{"settlement-prices", settlementPricesLimit, settlementPricesCmd.Flags().Lookup("limit").DefValue},
	}

	for _, tc := range cases {
		want, err := strconv.Atoi(tc.flag)
		if err != nil {
			t.Fatalf("market %s --limit default %q is not an int: %v", tc.name, tc.flag, err)
		}
		if tc.got != want {
			t.Errorf("market %s --limit = %d, want the documented default %d", tc.name, tc.got, want)
		}
	}
}
