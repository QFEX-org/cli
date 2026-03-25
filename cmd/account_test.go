package cmd

import (
	"testing"

	"github.com/qfex/cli/internal/config"
)

func TestCurrentSubaccountOutputPrimaryWhenUnset(t *testing.T) {
	cfg = &config.Config{}

	if got := currentSubaccountOutput(); got != "primary\n" {
		t.Fatalf("currentSubaccountOutput() = %q, want %q", got, "primary\n")
	}
}

func TestCurrentSubaccountOutputSelectedSubaccount(t *testing.T) {
	cfg = &config.Config{SelectedSubaccount: "sub-123"}

	if got := currentSubaccountOutput(); got != "sub-123\n" {
		t.Fatalf("currentSubaccountOutput() = %q, want %q", got, "sub-123\n")
	}
}

func TestValidateSubaccountTransferInputRequiresFromToAndAmount(t *testing.T) {
	if err := validateSubaccountTransferInput("", "", 0); err == nil {
		t.Fatalf("validateSubaccountTransferInput() error = nil, want error")
	}
}

func TestValidateSelectableSubaccountRejectsUnknownID(t *testing.T) {
	err := validateSelectableSubaccount("sub-404", []string{"sub-1", "sub-2"})
	if err == nil {
		t.Fatalf("validateSelectableSubaccount() error = nil, want error")
	}
}
