package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HMACCredentials holds the fields needed for QFEX HMAC auth.
type HMACCredentials struct {
	PublicKey string `json:"public_key"`
	Nonce     string `json:"nonce"`
	UnixTS    int64  `json:"unix_ts"`
	Signature string `json:"signature"`
}

// NewHMACCredentials generates fresh HMAC credentials from a public/secret key pair.
func NewHMACCredentials(publicKey, secretKey, accountID string) (*HMACCredentials, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes) + "." + signedRequestDigest("WS_AUTH", "qfex-trade", "", nil, accountID)

	ts := time.Now().Unix()
	msg := fmt.Sprintf("%s:%d", nonce, ts)

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(msg))
	sig := hex.EncodeToString(mac.Sum(nil))

	return &HMACCredentials{
		PublicKey: publicKey,
		Nonce:     nonce,
		UnixTS:    ts,
		Signature: sig,
	}, nil
}

// RESTHeaders returns the HTTP headers needed to authenticate a REST request.
// The nonce binds method, authority, raw path/query, body and selected account.
func RESTHeaders(publicKey, secretKey string, req *http.Request) (map[string]string, error) {
	var body []byte
	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody == nil {
			return nil, fmt.Errorf("request body cannot be replayed for signing")
		}
		reader, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		body, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, err
		}
	}
	authority := req.Host
	if authority == "" {
		authority = req.URL.Host
	}

	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes) + "." + signedRequestDigest(req.Method, authority, req.URL.RequestURI(), body, req.Header.Get("x-qfex-requested-account-id"))
	ts := fmt.Sprintf("%d", time.Now().Unix())

	msg := nonce + ":" + ts
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(msg))
	sig := hex.EncodeToString(mac.Sum(nil))

	return map[string]string{
		"x-qfex-public-key":     publicKey,
		"x-qfex-hmac-signature": sig,
		"x-qfex-nonce":          nonce,
		"x-qfex-timestamp":      ts,
	}, nil
}

func signedRequestDigest(method, authority, path string, body []byte, accountID string) string {
	bodyHash := sha256.Sum256(body)
	canonical := strings.Join([]string{"QFEX-HMAC-V1", strings.ToUpper(method), strings.ToLower(authority), path, hex.EncodeToString(bodyHash[:]), accountID}, "\n")
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}
