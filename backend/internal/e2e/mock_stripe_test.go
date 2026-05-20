package e2e

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

// mockStripe stands in for api.stripe.com. Two routes:
//   POST /v1/checkout/sessions     → returns a session JSON
//   GET  /v1/checkout/sessions/:id → returns session status
//
// The webhook signing secret is exposed via WebhookSecret() so tests
// can sign payloads with the same HMAC the dashboard verifies against.
type mockStripe struct {
	server        *httptest.Server
	webhookSecret string

	mu     sync.Mutex
	nextID int
	// sessions[id].status — "paid" | "unpaid" | etc
	sessions map[string]string
}

func newMockStripe(t *testing.T) *mockStripe {
	t.Helper()
	m := &mockStripe{
		webhookSecret: "whsec_test_e2e_secret_supersecret",
		sessions:      map[string]string{},
	}
	m.server = httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(m.server.Close)
	return m
}

func (m *mockStripe) URL() string           { return m.server.URL }
func (m *mockStripe) WebhookSecret() string { return m.webhookSecret }

// SetPaid marks a session as paid for trade.query polling tests.
func (m *mockStripe) SetPaid(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = "paid"
}

func (m *mockStripe) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/checkout/sessions":
		m.mu.Lock()
		m.nextID++
		id := fmt.Sprintf("cs_test_e2e_%d", m.nextID)
		m.sessions[id] = "unpaid"
		m.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":             id,
			"url":            "https://checkout.stripe.com/c/pay/" + id,
			"expires_at":     time.Now().Add(30 * time.Minute).Unix(),
			"payment_status": "unpaid",
			"status":         "open",
		})
	case r.Method == http.MethodGet:
		// /v1/checkout/sessions/{id}
		const prefix = "/v1/checkout/sessions/"
		if len(r.URL.Path) <= len(prefix) {
			http.Error(w, "no session id", 400)
			return
		}
		id := r.URL.Path[len(prefix):]
		m.mu.Lock()
		status := m.sessions[id]
		m.mu.Unlock()
		if status == "" {
			http.Error(w, `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"No such checkout session"}}`, 404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":             id,
			"payment_status": status,
			"status":         "complete",
		})
	default:
		http.Error(w, "not implemented", 404)
	}
}

// SignWebhook builds the Stripe-Signature header for `body` using the
// mock's webhook secret. `t` lets us pin the timestamp for replay
// tests; pass time.Now() for the happy-path.
func (m *mockStripe) SignWebhook(body []byte, signedAt time.Time) string {
	ts := strconv.FormatInt(signedAt.Unix(), 10)
	signed := ts + "." + string(body)
	mac := hmac.New(sha256.New, []byte(m.webhookSecret))
	mac.Write([]byte(signed))
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

