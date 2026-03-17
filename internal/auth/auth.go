package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
func NewHMACCredentials(publicKey, secretKey string) (*HMACCredentials, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

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
// The signature is HMAC-SHA256(secretKey, nonce + ":" + unix_ts).
func RESTHeaders(publicKey, secretKey string) (map[string]string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
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
