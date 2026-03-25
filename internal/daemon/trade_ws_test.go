package daemon

import (
	"testing"

	"github.com/qfex/cli/internal/config"
)

func TestBuildAuthMessageWithJWTIncludesAccountIDWhenSelectedSubaccountSet(t *testing.T) {
	tws := &TradeWS{
		cfg: &config.Config{
			AccessToken:        "jwt-token",
			SelectedSubaccount: "sub-123",
		},
	}

	msg, err := tws.buildAuthMessage()
	if err != nil {
		t.Fatalf("buildAuthMessage() error = %v", err)
	}

	params, ok := msg["params"].(map[string]any)
	if !ok {
		t.Fatalf("params has type %T, want map[string]any", msg["params"])
	}

	if got := params["account_id"]; got != "sub-123" {
		t.Fatalf("account_id = %v, want %q", got, "sub-123")
	}
}

func TestBuildAuthMessageWithoutSelectionOmitsAccountID(t *testing.T) {
	tws := &TradeWS{
		cfg: &config.Config{
			AccessToken: "jwt-token",
		},
	}

	msg, err := tws.buildAuthMessage()
	if err != nil {
		t.Fatalf("buildAuthMessage() error = %v", err)
	}

	params, ok := msg["params"].(map[string]any)
	if !ok {
		t.Fatalf("params has type %T, want map[string]any", msg["params"])
	}

	if _, ok := params["account_id"]; ok {
		t.Fatalf("account_id unexpectedly present: %#v", params["account_id"])
	}
}

func TestBuildAuthMessageWithHMACIncludesAccountIDWhenSelectedSubaccountSet(t *testing.T) {
	tws := &TradeWS{
		cfg: &config.Config{
			PublicKey:          "public-key",
			SecretKey:          "secret-key",
			SelectedSubaccount: "sub-123",
		},
	}

	msg, err := tws.buildAuthMessage()
	if err != nil {
		t.Fatalf("buildAuthMessage() error = %v", err)
	}

	params, ok := msg["params"].(map[string]any)
	if !ok {
		t.Fatalf("params has type %T, want map[string]any", msg["params"])
	}

	if got := params["account_id"]; got != "sub-123" {
		t.Fatalf("account_id = %v, want %q", got, "sub-123")
	}

	if _, ok := params["hmac"]; !ok {
		t.Fatalf("hmac auth payload missing")
	}
}
