package alipay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is an alipay 当面付 HTTP client. Thread-safe — all state
// is set at construction and reads only.
type Client struct {
	appID           string
	privateKey      string
	alipayPublicKey string
	gateway         string
	notifyURL       string
	http            *http.Client
	now             func() time.Time // injectable for deterministic tests
}

// NewClient builds a Client. Pass an empty `gateway` to use the
// production URL; the operator sets the sandbox URL via env var.
func NewClient(appID, privateKey, alipayPublicKey, gateway, notifyURL string) *Client {
	if gateway == "" {
		gateway = "https://openapi.alipay.com/gateway.do"
	}
	return &Client{
		appID:           appID,
		privateKey:      privateKey,
		alipayPublicKey: alipayPublicKey,
		gateway:         gateway,
		notifyURL:       notifyURL,
		http:            &http.Client{Timeout: 10 * time.Second},
		now:             time.Now,
	}
}

// SetHTTPClient replaces the default http.Client. Tests use this to
// inject a httptest server's client; production code never calls it.
func (c *Client) SetHTTPClient(h *http.Client) { c.http = h }

// SetNow injects the time source. Tests use this to pin signatures
// against fixture payloads; production code never calls it.
func (c *Client) SetNow(now func() time.Time) { c.now = now }

// PrecreateRequest is the alipay-specific biz_content payload for
// alipay.trade.precreate. JSON-encoded into the outer envelope.
type PrecreateRequest struct {
	OutTradeNo  string `json:"out_trade_no"`     // our order ID, as a string
	TotalAmount string `json:"total_amount"`     // yuan, 2 decimals: "12.34"
	Subject     string `json:"subject"`          // shown in the alipay UI
	TimeExpire  string `json:"time_expire,omitempty"`
}

// PrecreateResponse is the alipay response. QRCode is the URL we
// render as a QR for the user.
type PrecreateResponse struct {
	OutTradeNo string `json:"out_trade_no"`
	QRCode     string `json:"qr_code"`
}

// QueryRequest is the biz_content for alipay.trade.query.
type QueryRequest struct {
	OutTradeNo string `json:"out_trade_no,omitempty"`
	TradeNo    string `json:"trade_no,omitempty"`
}

// QueryResponse is the alipay response from trade.query.
type QueryResponse struct {
	OutTradeNo  string `json:"out_trade_no"`
	TradeNo     string `json:"trade_no"`
	TradeStatus string `json:"trade_status"` // WAIT_BUYER_PAY|TRADE_SUCCESS|TRADE_FINISHED|TRADE_CLOSED
	TotalAmount string `json:"total_amount"`
}

// Precreate calls alipay.trade.precreate. Returns the QR code URL +
// alipay's trade_no.
func (c *Client) Precreate(ctx context.Context, req PrecreateRequest) (*PrecreateResponse, error) {
	bizJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("alipay: marshal precreate biz: %w", err)
	}
	envelope, err := c.do(ctx, "alipay.trade.precreate", string(bizJSON))
	if err != nil {
		return nil, err
	}
	var resp PrecreateResponse
	if err := json.Unmarshal(envelope, &resp); err != nil {
		return nil, fmt.Errorf("alipay: parse precreate response: %w", err)
	}
	if resp.QRCode == "" {
		return nil, errors.New("alipay: precreate succeeded but qr_code missing")
	}
	return &resp, nil
}

// TradeQuery calls alipay.trade.query.
func (c *Client) TradeQuery(ctx context.Context, outTradeNo string) (*QueryResponse, error) {
	bizJSON, err := json.Marshal(QueryRequest{OutTradeNo: outTradeNo})
	if err != nil {
		return nil, fmt.Errorf("alipay: marshal query biz: %w", err)
	}
	envelope, err := c.do(ctx, "alipay.trade.query", string(bizJSON))
	if err != nil {
		return nil, err
	}
	var resp QueryResponse
	if err := json.Unmarshal(envelope, &resp); err != nil {
		return nil, fmt.Errorf("alipay: parse query response: %w", err)
	}
	return &resp, nil
}

// do builds the alipay envelope, signs it, POSTs, parses the JSON
// container, and returns the inner business-response JSON bytes.
// The container shape is:
//
//	{
//	  "alipay_trade_precreate_response": {
//	    "code": "10000",
//	    "msg": "Success",
//	    "out_trade_no": "...",
//	    "qr_code": "https://..."
//	  },
//	  "sign": "...base64..."
//	}
//
// We verify the alipay-platform signature over the
// "alipay_*_response" JSON before returning.
func (c *Client) do(ctx context.Context, method, bizContent string) ([]byte, error) {
	params := map[string]string{
		"app_id":      c.appID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   c.now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": bizContent,
	}
	if c.notifyURL != "" {
		params["notify_url"] = c.notifyURL
	}
	sig, err := SignRSA2(params, c.privateKey)
	if err != nil {
		return nil, err
	}
	params["sign"] = sig

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.gateway, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("alipay: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("alipay: http: %w", err)
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("alipay: read body: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alipay: http %d: %s", httpResp.StatusCode, body)
	}

	// The container is wrapped JSON keyed by the response method:
	// `alipay_trade_precreate_response`. We extract that nested
	// object and verify the outer-envelope sign over its raw JSON.
	respKey := strings.ReplaceAll(method, ".", "_") + "_response"
	innerRaw, signValue, err := splitEnvelope(body, respKey)
	if err != nil {
		return nil, err
	}
	if err := verifyEnvelope(innerRaw, signValue, c.alipayPublicKey); err != nil {
		return nil, err
	}

	// Inner has alipay's universal {code, msg, sub_code, sub_msg, ...}
	// plus the business fields. Surface non-success error codes so
	// callers don't need to know alipay's code semantics.
	var status struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		SubCode string `json:"sub_code"`
		SubMsg  string `json:"sub_msg"`
	}
	if err := json.Unmarshal(innerRaw, &status); err != nil {
		return nil, fmt.Errorf("alipay: parse inner status: %w", err)
	}
	if status.Code != "10000" {
		return nil, fmt.Errorf("alipay: %s (%s): %s — %s", status.Code, status.SubCode, status.Msg, status.SubMsg)
	}
	return innerRaw, nil
}

// splitEnvelope extracts the inner response JSON and the sign field
// from alipay's wrapper. We can't use a typed unmarshal because we
// need the *raw bytes* of the inner object for signature verification
// (any re-marshal could reorder keys and break the signature).
func splitEnvelope(body []byte, respKey string) (innerRaw json.RawMessage, signValue string, err error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, "", fmt.Errorf("alipay: parse envelope: %w", err)
	}
	inner, ok := envelope[respKey]
	if !ok {
		return nil, "", fmt.Errorf("alipay: envelope missing %s", respKey)
	}
	signRaw, ok := envelope["sign"]
	if !ok {
		return inner, "", fmt.Errorf("alipay: %w: envelope missing sign", ErrSignatureFailed)
	}
	if err := json.Unmarshal(signRaw, &signValue); err != nil {
		return inner, "", fmt.Errorf("alipay: parse sign: %w", err)
	}
	return inner, signValue, nil
}

// verifyEnvelope verifies the outer-envelope signature, which is
// computed over the inner-response RAW JSON bytes (NOT the
// canonical-params algorithm — that's only for the request side).
func verifyEnvelope(innerRaw json.RawMessage, signature, pemPublicKey string) error {
	key, err := parsePublicKey(pemPublicKey)
	if err != nil {
		return err
	}
	sig, err := base64Decode(signature)
	if err != nil {
		return fmt.Errorf("%w: base64 decode: %v", ErrSignatureFailed, err)
	}
	if err := rsaVerifySHA256(key, innerRaw, sig); err != nil {
		return fmt.Errorf("%w: %v", ErrSignatureFailed, err)
	}
	return nil
}
