package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/qfex/cli/internal/auth"
)

// apiGet makes a GET request to the REST API and returns the parsed JSON body.
// If needsAuth is true, HMAC auth headers are added.
func apiGet(path string, params url.Values, needsAuth bool) json.RawMessage {
	u := cfg.APIURL() + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Accept", "application/json")
	if needsAuth {
		setAuthHeaders(req)
	}
	return doRequest(req)
}

// apiPost makes an authenticated POST request and returns the parsed JSON body.
func apiPost(path string, payload any) json.RawMessage {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, cfg.APIURL()+path, bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	setAuthHeaders(req)
	return doRequest(req)
}

// apiStream makes a GET request and streams the response body to stdout (for CSV endpoints).
func apiStream(path string, params url.Values, needsAuth bool) {
	u := cfg.APIURL() + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if needsAuth {
		setAuthHeaders(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	io.Copy(os.Stdout, resp.Body)
}

func setAuthHeaders(req *http.Request) {
	headers, err := auth.RESTHeaders(cfg.PublicKey, cfg.SecretKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating auth headers: %v\n", err)
		os.Exit(1)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func doRequest(req *http.Request) json.RawMessage {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "Error %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	return json.RawMessage(body)
}
