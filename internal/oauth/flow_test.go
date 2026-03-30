package oauth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestSubFromTokenReturnsSub(t *testing.T) {
	token := fakeJWT(t, map[string]any{"sub": "user-abc-123", "email": "a@b.com"})
	if got := SubFromToken(token); got != "user-abc-123" {
		t.Fatalf("SubFromToken() = %q, want %q", got, "user-abc-123")
	}
}

func TestEmailFromTokenReturnsEmail(t *testing.T) {
	token := fakeJWT(t, map[string]any{"sub": "user-abc-123", "email": "a@b.com"})
	if got := EmailFromToken(token); got != "a@b.com" {
		t.Fatalf("EmailFromToken() = %q, want %q", got, "a@b.com")
	}
}

func TestSubFromTokenEmptyOnMalformed(t *testing.T) {
	if got := SubFromToken("not-a-jwt"); got != "" {
		t.Fatalf("SubFromToken(malformed) = %q, want empty", got)
	}
}

func TestSubFromTokenEmptyWhenClaimMissing(t *testing.T) {
	token := fakeJWT(t, map[string]any{"email": "a@b.com"})
	if got := SubFromToken(token); got != "" {
		t.Fatalf("SubFromToken(no sub) = %q, want empty", got)
	}
}

func fakeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
