package cmd

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/qfex/cli/internal/config"
)

func TestSetAuthHeadersAddsRequestedAccountIDWhenSelectedSubaccountPresent(t *testing.T) {
	cfg = &config.Config{
		AccessToken:        testJWT(t, time.Now().Add(time.Hour).Unix()),
		SelectedSubaccount: "sub-123",
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	setAuthHeaders(req, true)

	if got := req.Header.Get("x-qfex-requested-account-id"); got != "sub-123" {
		t.Fatalf("requested account header = %q, want %q", got, "sub-123")
	}
}

func TestSetAuthHeadersOmitsRequestedAccountIDWhenNoSelection(t *testing.T) {
	cfg = &config.Config{
		AccessToken: testJWT(t, time.Now().Add(time.Hour).Unix()),
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	setAuthHeaders(req, true)

	if got := req.Header.Get("x-qfex-requested-account-id"); got != "" {
		t.Fatalf("requested account header = %q, want empty", got)
	}
}

func TestSetAuthHeadersOmitsRequestedAccountIDWhenSelectionDisabled(t *testing.T) {
	cfg = &config.Config{
		AccessToken:        testJWT(t, time.Now().Add(time.Hour).Unix()),
		SelectedSubaccount: "sub-123",
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	setAuthHeaders(req, false)

	if got := req.Header.Get("x-qfex-requested-account-id"); got != "" {
		t.Fatalf("requested account header = %q, want empty", got)
	}
}

func testJWT(t *testing.T, exp int64) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(map[string]any{"exp": exp})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".signature"
}
