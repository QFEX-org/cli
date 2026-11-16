package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRequestBindingVector(t *testing.T) {
	const want = "ca4107bc4b0cea2ef18a44bd9ef40c7f7b5ef04e9e46830b43d8106c8a087d3d"
	if got := signedRequestDigest("POST", "api.qfex.com", "/user/transfer?dst=one", []byte(`{"amount":1}`), "account-one"); got != want {
		t.Fatalf("digest=%s", got)
	}
}

func TestRESTHeadersBindTheExactWireRequestWithoutConsumingBody(t *testing.T) {
	req, err := http.NewRequest("POST", "https://api.qfex.com/user/transfer?dst=one", strings.NewReader(`{"amount":1}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-qfex-requested-account-id", "account-one")
	headers, err := RESTHeaders("public", "secret", req)
	if err != nil {
		t.Fatal(err)
	}
	nonce := headers["x-qfex-nonce"]
	if len(nonce) != 97 || nonce[33:] != "ca4107bc4b0cea2ef18a44bd9ef40c7f7b5ef04e9e46830b43d8106c8a087d3d" {
		t.Fatal("request was not bound")
	}
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(nonce + ":" + headers["x-qfex-timestamp"]))
	if headers["x-qfex-hmac-signature"] != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("invalid signature")
	}
	data, err := io.ReadAll(req.Body)
	if err != nil || string(data) != `{"amount":1}` {
		t.Fatal("signing changed the request body")
	}
	req.Body = io.NopCloser(strings.NewReader("stream"))
	req.GetBody = nil
	if _, err := RESTHeaders("public", "secret", req); err == nil {
		t.Fatal("silently signed an unreadable body as empty")
	}
}

func TestWebSocketProofIsDistinctFromHTTPAndBindsAccount(t *testing.T) {
	creds, err := NewHMACCredentials("public", "secret", "account-one")
	if err != nil {
		t.Fatal(err)
	}
	digest := creds.Nonce[33:]
	if digest != signedRequestDigest("WS_AUTH", "qfex-trade", "", nil, "account-one") {
		t.Fatal("wrong websocket scope")
	}
	if digest == signedRequestDigest("GET", "trade.qfex.com", "/", nil, "account-one") || digest == signedRequestDigest("WS_AUTH", "qfex-trade", "", nil, "account-two") {
		t.Fatal("proof crosses transport/account")
	}
}
