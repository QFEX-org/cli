package daemon

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QFEX-org/cli/internal/config"
	"github.com/gorilla/websocket"
)

func TestSendAuthDoesNotLogCredentials(t *testing.T) {
	for _, cfg := range []*config.Config{
		{AccessToken: "private-jwt-token"},
		{PublicKey: "private-public-key", SecretKey: "private-secret-key"},
	} {
		received := make(chan map[string]any, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			var msg map[string]any
			if conn.ReadJSON(&msg) == nil {
				received <- msg
			}
		}))
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		var output bytes.Buffer
		client := &TradeWS{cfg: cfg, log: log.New(&output, "", 0)}
		if err := client.sendAuth(conn); err != nil {
			t.Fatal(err)
		}
		message := <-received
		params := message["params"].(map[string]any)
		secrets := []string{cfg.AccessToken, cfg.PublicKey, cfg.SecretKey}
		if hmac, ok := params["hmac"].(map[string]any); ok {
			for _, field := range []string{"signature", "nonce"} {
				secrets = append(secrets, hmac[field].(string))
			}
		}
		raw, _ := json.Marshal(params)
		if len(params) == 0 || strings.Contains(output.String(), string(raw)) {
			t.Fatal("missing wire credentials or auth payload logged")
		}
		for _, secret := range secrets {
			if secret != "" && strings.Contains(output.String(), secret) {
				t.Fatal("credential appeared in logs")
			}
		}
		conn.Close()
		server.Close()
	}
}
