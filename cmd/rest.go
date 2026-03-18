package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/qfex/cli/internal/auth"
	"github.com/qfex/cli/internal/build"
)

// apiGetURL makes a GET request to an arbitrary URL and returns the parsed JSON body.
func apiGetURL(u string, needsAuth bool) json.RawMessage {
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
	req.Header.Set("User-Agent", build.UserAgent())
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

// fetchSymbols returns all active symbol names from /refdata.
// Used for shell tab-completion in commands that accept a <symbol> argument.
func fetchSymbols() []string {
	data := apiGet("/refdata", nil, false)
	var resp struct {
		Data []struct {
			Symbol string `json:"symbol"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	symbols := make([]string, 0, len(resp.Data))
	for _, s := range resp.Data {
		if s.Status == "ACTIVE" {
			symbols = append(symbols, s.Symbol)
		}
	}
	return symbols
}

// symbolCompletion is a Cobra ValidArgsFunction that completes symbol names.
func symbolCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return fetchSymbols(), cobra.ShellCompDirectiveNoFileComp
}

func doRequest(req *http.Request) json.RawMessage {
	req.Header.Set("User-Agent", build.UserAgent())
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
