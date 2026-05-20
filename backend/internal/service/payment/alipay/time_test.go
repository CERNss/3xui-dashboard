package alipay

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFormatBeijing_ShiftsUTCByEightHours(t *testing.T) {
	// 2026-05-20 06:00:00 UTC = 2026-05-20 14:00:00 Beijing
	utc := time.Date(2026, 5, 20, 6, 0, 0, 0, time.UTC)
	got := formatBeijing(utc)
	want := "2026-05-20 14:00:00"
	if got != want {
		t.Errorf("formatBeijing(UTC 06:00) = %q, want %q", got, want)
	}
}

func TestFormatBeijing_PreservesBeijingTime(t *testing.T) {
	// If we feed in a Beijing time, it stays the same wall-clock.
	bj := time.Date(2026, 5, 20, 14, 0, 0, 0, beijingTZ)
	got := formatBeijing(bj)
	want := "2026-05-20 14:00:00"
	if got != want {
		t.Errorf("formatBeijing(Beijing 14:00) = %q, want %q", got, want)
	}
}

// TestTimestampInRequest_IsBeijing asserts the `timestamp` form
// param sent to alipay is the Beijing wall-clock for the injected
// `now`, not UTC. This is the test that would have caught the
// production bug: a UTC server posting a `timestamp` 8 hours older
// than alipay expects gets rejected.
func TestTimestampInRequest_IsBeijing(t *testing.T) {
	f := newFakeAlipay(t)
	var capturedTimestamp string
	f.handler = func(_ string, params map[string]string) ([]byte, string) {
		capturedTimestamp = params["timestamp"]
		return []byte(`{"code":"10000","msg":"Success","out_trade_no":"42","qr_code":"https://x"}`), "10000"
	}
	priv, _ := clientKeypair(t)
	c := NewClient("appid", priv, f.serverPub, f.server.URL, "")
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 6, 0, 0, 0, time.UTC) })

	_, err := c.Precreate(context.Background(), PrecreateRequest{
		OutTradeNo: "42", TotalAmount: "1.00", Subject: "x",
	})
	if err != nil {
		t.Fatalf("Precreate: %v", err)
	}
	if capturedTimestamp != "2026-05-20 14:00:00" {
		t.Errorf("timestamp = %q, want 2026-05-20 14:00:00 (UTC 06:00 + 8h)", capturedTimestamp)
	}
}

// TestTimeExpire_IsBeijing checks the gateway-level wrapper renders
// TimeExpire in Beijing TZ too. Catches the parallel bug in
// gateway.go where TimeExpire was hand-formatted with the wrong TZ.
func TestTimeExpire_IsBeijing(t *testing.T) {
	f := newFakeAlipay(t)
	var capturedBiz string
	f.handler = func(_ string, params map[string]string) ([]byte, string) {
		capturedBiz = params["biz_content"]
		return []byte(`{"code":"10000","msg":"Success","out_trade_no":"42","qr_code":"https://x"}`), "10000"
	}
	priv, _ := clientKeypair(t)
	c := NewClient("appid", priv, f.serverPub, f.server.URL, "")
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 6, 0, 0, 0, time.UTC) })

	_, err := c.Precreate(context.Background(), PrecreateRequest{
		OutTradeNo:  "42",
		TotalAmount: "1.00",
		Subject:     "x",
		TimeExpire:  formatBeijing(time.Date(2026, 5, 20, 6, 15, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("Precreate: %v", err)
	}
	// biz_content is captured already-URL-decoded by httptest.
	if !strings.Contains(capturedBiz, `"time_expire":"2026-05-20 14:15:00"`) {
		t.Errorf("time_expire wrong in biz_content: %s", capturedBiz)
	}
	// JSON-parse for the strict assertion too.
	var biz PrecreateRequest
	_ = json.Unmarshal([]byte(capturedBiz), &biz)
	if biz.TimeExpire != "2026-05-20 14:15:00" {
		t.Errorf("parsed time_expire = %q", biz.TimeExpire)
	}
}

