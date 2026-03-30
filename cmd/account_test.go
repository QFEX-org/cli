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

func TestResolveAccountIDPrimaryReturnsUserID(t *testing.T) {
	cfg = &config.Config{UserID: "user-abc-123"}
	got, err := resolveAccountID("primary")
	if err != nil {
		t.Fatalf("resolveAccountID(\"primary\") error = %v", err)
	}
	if got != "user-abc-123" {
		t.Fatalf("resolveAccountID(\"primary\") = %q, want %q", got, "user-abc-123")
	}
}

func TestResolveAccountIDPrimaryCaseInsensitive(t *testing.T) {
	cfg = &config.Config{UserID: "user-abc-123"}
	got, err := resolveAccountID("Primary")
	if err != nil {
		t.Fatalf("resolveAccountID(\"Primary\") error = %v", err)
	}
	if got != "user-abc-123" {
		t.Fatalf("resolveAccountID(\"Primary\") = %q, want %q", got, "user-abc-123")
	}
}

func TestResolveAccountIDPrimaryErrorsWhenNoUserID(t *testing.T) {
	cfg = &config.Config{}
	_, err := resolveAccountID("primary")
	if err == nil {
		t.Fatalf("resolveAccountID(\"primary\") error = nil, want error")
	}
}

func TestResolveAccountIDPassesThroughUUID(t *testing.T) {
	cfg = &config.Config{}
	id := "f0ed8fbd-5a60-46fe-b112-e8887ea02824"
	got, err := resolveAccountID(id)
	if err != nil {
		t.Fatalf("resolveAccountID(%q) error = %v", id, err)
	}
	if got != id {
		t.Fatalf("resolveAccountID(%q) = %q, want %q", id, got, id)
	}
}
