// Package stripe implements Stripe Checkout Sessions as a
// payment.Gateway. Pure stdlib — same rationale as alipay (no SDK
// lock-in, audit-friendly). 2 HTTP endpoints + 1 webhook verifier
// is well under the threshold where the stripe-go SDK pays off.
//
// References:
//   - Checkout Sessions API: https://docs.stripe.com/api/checkout/sessions
//   - Webhook signing:        https://docs.stripe.com/webhooks/signatures
package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Pin the API version so Stripe doesn't break us by changing
// defaults underneath. Bump after testing.
const stripeAPIVersion = "2024-06-20"

// Client is a Stripe HTTP client. Thread-safe — all state is set at
// construction and reads only.
type Client struct {
	secretKey            string
	baseURL              string
	currency             string
	successURL           string
	cancelURL            string
	sessionExpiryMinutes int
	http                 *http.Client
	now                  func() time.Time
}

// NewClient builds the client. baseURL is exposed so tests can
// point to a httptest server; production wiring uses the default.
func NewClient(secretKey, currency, successURL, cancelURL string, sessionExpiryMinutes int) *Client {
	if currency == "" {
		currency = "usd"
	}
	if sessionExpiryMinutes <= 0 {
		sessionExpiryMinutes = 30
	}
	return &Client{
		secretKey:            secretKey,
		baseURL:              "https://api.stripe.com",
		currency:             currency,
		successURL:           successURL,
		cancelURL:            cancelURL,
		sessionExpiryMinutes: sessionExpiryMinutes,
		http:                 &http.Client{Timeout: 10 * time.Second},
		now:                  time.Now,
	}
}

// SetBaseURL replaces the API host. Tests use this to point at a
// httptest.NewServer; production never calls it.
func (c *Client) SetBaseURL(u string) { c.baseURL = u }

// SetHTTPClient replaces the HTTP client. Test-only.
func (c *Client) SetHTTPClient(h *http.Client) { c.http = h }

// SetNow injects a clock. Test-only.
func (c *Client) SetNow(now func() time.Time) { c.now = now }

// CheckoutSession is the subset of Stripe's response we care about.
type CheckoutSession struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	ExpiresAt     int64  `json:"expires_at"` // unix seconds
	PaymentStatus string `json:"payment_status"` // "paid" | "unpaid" | "no_payment_required"
	Status        string `json:"status"`         // "open" | "complete" | "expired"
}

// CreateCheckoutSession calls POST /v1/checkout/sessions. The order
// ID is used both as `client_reference_id` (so the webhook payload
// links back to our order) AND as the Idempotency-Key suffix so a
// retried POST returns the same Session URL.
func (c *Client) CreateCheckoutSession(ctx context.Context, orderID int64, priceCents int64, productName string) (*CheckoutSession, error) {
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("client_reference_id", strconv.FormatInt(orderID, 10))
	if c.successURL != "" {
		form.Set("success_url", c.successURL)
	}
	if c.cancelURL != "" {
		form.Set("cancel_url", c.cancelURL)
	}
	form.Set("expires_at", strconv.FormatInt(c.now().Add(time.Duration(c.sessionExpiryMinutes)*time.Minute).Unix(), 10))

	// Stripe's URL-encoded params use bracket notation for nested
	// objects: line_items[0][price_data][unit_amount]=500
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", c.currency)
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(priceCents, 10))
	form.Set("line_items[0][price_data][product_data][name]", productName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("stripe: build request: %w", err)
	}
	c.applyAuth(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", fmt.Sprintf("order-%d-v1", orderID))

	var sess CheckoutSession
	if err := c.do(req, &sess); err != nil {
		return nil, err
	}
	if sess.URL == "" {
		return nil, fmt.Errorf("stripe: session created but url missing")
	}
	return &sess, nil
}

// RetrieveCheckoutSession calls GET /v1/checkout/sessions/{id}.
// Used by the payment-poll job as a failsafe.
func (c *Client) RetrieveCheckoutSession(ctx context.Context, id string) (*CheckoutSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/checkout/sessions/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, fmt.Errorf("stripe: build request: %w", err)
	}
	c.applyAuth(req)
	var sess CheckoutSession
	if err := c.do(req, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// applyAuth sets the Stripe-Version + Bearer headers.
func (c *Client) applyAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Stripe-Version", stripeAPIVersion)
}

// do executes the request, decodes the JSON response into dst, and
// surfaces Stripe error envelopes as typed errors so callers don't
// string-match against bodies. Stripe error envelope shape:
//
//	{ "error": { "type": "...", "message": "...", "code": "..." } }
func (c *Client) do(req *http.Request, dst any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("stripe: http: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("stripe: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		// Try to surface the typed Stripe error if the response
		// looks like one; fall back to status + raw body otherwise.
		var env struct {
			Err *struct {
				Type    string `json:"type"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if jerr := json.Unmarshal(body, &env); jerr == nil && env.Err != nil {
			return fmt.Errorf("stripe: %s (%s): %s", env.Err.Type, env.Err.Code, env.Err.Message)
		}
		return fmt.Errorf("stripe: http %d: %s", resp.StatusCode, body)
	}
	if dst != nil {
		if err := json.Unmarshal(body, dst); err != nil {
			return fmt.Errorf("stripe: parse response: %w", err)
		}
	}
	return nil
}
