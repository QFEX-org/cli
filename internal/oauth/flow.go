package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	callbackPath = "/callback"

	// CallbackPort is the fixed local port for the OAuth redirect.
	// Both the prod and UAT Supabase OAuth clients must have
	// http://localhost:57423/callback registered as an allowed redirect URI.
	CallbackPort = 57423

	// ProdSupabaseURL is the Supabase project URL for the production environment.
	ProdSupabaseURL = "https://verify.qfex.com"

	// UATSupabaseURL is the Supabase project URL for the UAT environment.
	// TODO: set to your UAT Supabase project URL.
	UATSupabaseURL = "https://verify.qfex.io"

	// ProdClientID is the Supabase OAuth client ID for the CLI in production.
	// TODO: set to your prod CLI OAuth app client_id.
	ProdClientID = "885ac1cf-8cc1-4878-8ece-5d7c379ea4d7"

	// UATClientID is the Supabase OAuth client ID for the CLI in UAT.
	// TODO: set to your UAT CLI OAuth app client_id.
	UATClientID = "6557526d-5249-44f0-992e-791942c0b3d4"
)

// Config holds the OAuth parameters for a browser login flow.
type Config struct {
	SupabaseURL string
	ClientID    string
	// Scopes is optional; leave nil for default Supabase OIDC scopes.
	Scopes []string
}

// Tokens holds the credentials returned after a successful browser login.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	// ExpiresIn is the lifetime of AccessToken in seconds.
	ExpiresIn int
}

// RunBrowserFlow performs a PKCE authorization-code flow against Supabase.
//
// It:
//  1. Generates a PKCE state + code verifier/challenge.
//  2. Starts a local HTTP server on CallbackPort.
//  3. Prints the authorization URL and tries to open the user's browser.
//  4. Waits up to 5 minutes for the OAuth callback.
//  5. Exchanges the authorization code for access + refresh tokens.
func RunBrowserFlow(ctx context.Context, cfg Config) (Tokens, error) {
	if cfg.SupabaseURL == "" {
		return Tokens{}, fmt.Errorf("Supabase URL not configured for this environment — " +
			"set UATSupabaseURL in cli/internal/oauth/flow.go")
	}
	if cfg.ClientID == "" {
		return Tokens{}, fmt.Errorf("OAuth client ID not configured — " +
			"set ProdClientID / UATClientID in cli/internal/oauth/flow.go")
	}

	state, verifier, challenge, err := generatePKCE()
	if err != nil {
		return Tokens{}, fmt.Errorf("generate PKCE: %w", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", CallbackPort))
	if err != nil {
		return Tokens{}, fmt.Errorf(
			"cannot listen on port %d (is another login in progress?): %w", CallbackPort, err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d%s", CallbackPort, callbackPath)
	authURL := buildAuthURL(cfg, state, challenge, redirectURI)

	codeCh := make(chan string, 1)
	failCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if oauthErr := q.Get("error"); oauthErr != "" {
			desc := q.Get("error_description")
			select {
			case failCh <- fmt.Errorf("login denied: %s: %s", oauthErr, desc):
			default:
			}
			renderPage(w, http.StatusBadRequest, pageData{
				PageTitle:   "QFEX CLI login failed",
				Title:       "Login failed",
				Description: oauthErr,
				Footer:      "You may close this tab and retry in your terminal.",
			})
			return
		}
		if q.Get("state") != state {
			select {
			case failCh <- fmt.Errorf("state mismatch — possible CSRF, please retry"):
			default:
			}
			renderPage(w, http.StatusBadRequest, pageData{
				PageTitle:   "QFEX CLI login failed",
				Title:       "Login failed",
				Description: "Invalid state parameter — possible CSRF attack. Please retry.",
				Footer:      "You may close this tab.",
			})
			return
		}
		code := q.Get("code")
		if code == "" {
			select {
			case failCh <- fmt.Errorf("no authorization code in callback"):
			default:
			}
			renderPage(w, http.StatusBadRequest, pageData{
				PageTitle:   "QFEX CLI login failed",
				Title:       "Login failed",
				Description: "No authorization code returned.",
				Footer:      "You may close this tab and retry in your terminal.",
			})
			return
		}
		select {
		case codeCh <- code:
		default:
		}
		renderPage(w, http.StatusOK, pageData{
			PageTitle:   "QFEX CLI login successful",
			Title:       "Login successful",
			Description: "Your QFEX CLI is now authenticated.",
			Footer:      "You may close this tab and return to your terminal.",
		})
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	fmt.Printf("Opening browser for QFEX login...\n\n")
	fmt.Printf("If the browser does not open, visit:\n  %s\n\n", authURL)
	openBrowser(authURL)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	select {
	case code := <-codeCh:
		return exchangeCode(ctx, cfg, code, verifier, redirectURI)
	case err := <-failCh:
		return Tokens{}, err
	case <-ctx.Done():
		return Tokens{}, fmt.Errorf("login timed out (5 minutes)")
	}
}

func generatePKCE() (state, verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return
	}
	state = base64.RawURLEncoding.EncodeToString(buf)

	if _, err = rand.Read(buf); err != nil {
		return
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)

	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return
}

func buildAuthURL(cfg Config, state, challenge, redirectURI string) string {
	params := url.Values{
		"client_id":             {cfg.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	if len(cfg.Scopes) > 0 {
		params.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	return cfg.SupabaseURL + "/auth/v1/oauth/authorize?" + params.Encode()
}

func exchangeCode(ctx context.Context, cfg Config, code, verifier, redirectURI string) (Tokens, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
		"client_id":     {cfg.ClientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.SupabaseURL+"/auth/v1/oauth/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Tokens{}, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Tokens{}, fmt.Errorf("decode token response (HTTP %d): %w", resp.StatusCode, err)
	}
	if payload.Error != "" {
		return Tokens{}, fmt.Errorf("token exchange failed: %s: %s", payload.Error, payload.ErrorDesc)
	}
	if payload.AccessToken == "" {
		return Tokens{}, fmt.Errorf("no access_token in response (HTTP %d)", resp.StatusCode)
	}
	return Tokens{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresIn:    payload.ExpiresIn,
	}, nil
}

type pageData struct {
	PageTitle   string
	Title       string
	Description string
	Footer      string
}

func renderPage(w http.ResponseWriter, statusCode int, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	callbackPageTemplate.Execute(w, data) //nolint:errcheck
}

var callbackPageTemplate = template.Must(template.New("callback-page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.PageTitle}}</title>
  <style>
    body {
      margin: 0;
      min-height: 100vh;
      box-sizing: border-box;
      display: grid;
      place-items: center;
      padding: 24px;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,sans-serif;
    }
    main {
      width: min(100%, 560px);
      text-align: center;
    }
    h1 {font-weight: 500; }
  </style>
</head>
<body>
  <main>
    <h1>{{.Title}}</h1>
    <p>{{.Description}}</p>
    <p>{{.Footer}}</p>
  </main>
</body>
</html>`))

// SupabaseURLForEnv returns the Supabase project URL for the given environment.
func SupabaseURLForEnv(env string) string {
	if env == "uat" {
		return UATSupabaseURL
	}
	return ProdSupabaseURL
}

// IsTokenExpired returns true when the JWT access token is expired or within
// 60 seconds of expiring, or if it cannot be parsed.
func IsTokenExpired(accessToken string) bool {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return true
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return true
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return true
	}
	return time.Now().Add(60 * time.Second).Unix() >= claims.Exp
}

// RefreshTokens exchanges a refresh token for a new access + refresh token pair.
func RefreshTokens(ctx context.Context, supabaseURL, refreshToken string) (Tokens, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		supabaseURL+"/auth/v1/token?grant_type=refresh_token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Tokens{}, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Tokens{}, fmt.Errorf("decode refresh response (HTTP %d): %w", resp.StatusCode, err)
	}
	if payload.Error != "" {
		return Tokens{}, fmt.Errorf("token refresh failed: %s: %s", payload.Error, payload.ErrorDesc)
	}
	if payload.AccessToken == "" {
		return Tokens{}, fmt.Errorf("no access_token in refresh response (HTTP %d)", resp.StatusCode)
	}
	return Tokens{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresIn:    payload.ExpiresIn,
	}, nil
}

func openBrowser(u string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{u}
	case "linux":
		cmd, args = "xdg-open", []string{u}
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", u}
	default:
		return
	}
	exec.Command(cmd, args...).Start() //nolint:errcheck
}
