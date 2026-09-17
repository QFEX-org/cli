package cmd

import "testing"

// A flag variable shared by several commands keeps only the default of the last
// registration, so each command must own the variable behind its own default.
func TestOrderTypeFlagDefaults(t *testing.T) {
	cases := []struct {
		name string
		got  string
		flag string
	}{
		{"place", placeOrderType, placeOrderCmd.Flags().Lookup("type").DefValue},
		{"modify", modifyOrderType, modifyOrderCmd.Flags().Lookup("type").DefValue},
	}

	for _, tc := range cases {
		if tc.got != tc.flag {
			t.Errorf("order %s --type = %q, want the documented default %q", tc.name, tc.got, tc.flag)
		}
	}
}
