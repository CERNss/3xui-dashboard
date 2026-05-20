package stripe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newStripeFake(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := NewClient("sk_test_dummy", "usd", "https://example.com/ok", "https://example.com/cancel", 30)
	c.SetBaseURL(server.URL)
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) })
	return c, server
}

func TestCreateCheckoutSession_Success(t *testing.T) {
	var capturedAuth, capturedVersion, capturedIdempotency, capturedBody string
	c, _ := newStripeFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/checkout/sessions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		capturedAuth = r.Header.Get("Authorization")
		capturedVersion = r.Header.Get("Stripe-Version")
		capturedIdempotency = r.Header.Get("Idempotency-Key")
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		capturedBody = string(b)
		_ = json.NewEncoder(w).Encode(CheckoutSession{
			ID:        "cs_test_a1B2c3",
			URL:       "https://checkout.stripe.com/c/pay/cs_test_a1B2c3",
			ExpiresAt: time.Now().Add(30 * time.Minute).Unix(),
		})
	})

	sess, err := c.CreateCheckoutSession(context.Background(), 42, 500, "Pro Plan")
	if err != nil {
		t.Fatalf("CreateCheckoutSession: %v", err)
	}
	if sess.ID != "cs_test_a1B2c3" {
		t.Errorf("ID = %q", sess.ID)
	}
	if sess.URL != "https://checkout.stripe.com/c/pay/cs_test_a1B2c3" {
		t.Errorf("URL = %q", sess.URL)
	}
	if capturedAuth != "Bearer sk_test_dummy" {
		t.Errorf("Auth header = %q", capturedAuth)
	}
	if capturedVersion != stripeAPIVersion {
		t.Errorf("Stripe-Version = %q, want %q", capturedVersion, stripeAPIVersion)
	}
	if capturedIdempotency != "order-42-v1" {
		t.Errorf("Idempotency-Key = %q", capturedIdempotency)
	}
	// Body has the expected fields. The brackets in line_items[0]
	// are URL-encoded as %5B%5D so we check for the encoded form.
	if !strings.Contains(capturedBody, "client_reference_id=42") {
		t.Errorf("body missing client_reference_id: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "unit_amount%5D=500") {
		t.Errorf("body missing unit_amount=500: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "currency%5D=usd") {
		t.Errorf("body missing currency=usd: %s", capturedBody)
	}
}

func TestCreateCheckoutSession_StripeError(t *testing.T) {
	c, _ := newStripeFake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","code":"parameter_missing","message":"line_items required"}}`))
	})
	_, err := c.CreateCheckoutSession(context.Background(), 42, 500, "Pro")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid_request_error") {
		t.Errorf("error missing stripe type: %v", err)
	}
	if !strings.Contains(err.Error(), "parameter_missing") {
		t.Errorf("error missing stripe code: %v", err)
	}
}

func TestCreateCheckoutSession_HTTPError(t *testing.T) {
	c, _ := newStripeFake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	})
	_, err := c.CreateCheckoutSession(context.Background(), 42, 500, "Pro")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error missing status: %v", err)
	}
}

func TestRetrieveCheckoutSession_Paid(t *testing.T) {
	c, _ := newStripeFake(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/checkout/sessions/cs_test_") {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(CheckoutSession{
			ID:            "cs_test_x",
			PaymentStatus: "paid",
			Status:        "complete",
		})
	})
	sess, err := c.RetrieveCheckoutSession(context.Background(), "cs_test_x")
	if err != nil {
		t.Fatalf("RetrieveCheckoutSession: %v", err)
	}
	if sess.PaymentStatus != "paid" {
		t.Errorf("PaymentStatus = %q", sess.PaymentStatus)
	}
}

func TestRetrieveCheckoutSession_Unpaid(t *testing.T) {
	c, _ := newStripeFake(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(CheckoutSession{ID: "cs_x", PaymentStatus: "unpaid", Status: "open"})
	})
	sess, err := c.RetrieveCheckoutSession(context.Background(), "cs_x")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if sess.PaymentStatus != "unpaid" {
		t.Errorf("PaymentStatus = %q", sess.PaymentStatus)
	}
}

func TestRetrieveCheckoutSession_NotFound(t *testing.T) {
	c, _ := newStripeFake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","code":"resource_missing","message":"No such checkout session"}}`))
	})
	_, err := c.RetrieveCheckoutSession(context.Background(), "cs_missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "resource_missing") {
		t.Errorf("error: %v", err)
	}
}
