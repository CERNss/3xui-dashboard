package channels

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/service/notify"
)

func TestFeishu_DefaultCardWhenNoTemplate(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		receivedBody, _ = io.ReadAll(req.Body)
		w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer srv.Close()

	c := NewFeishu(srv.URL, "") // no template → default card
	err := c.Send(context.Background(), notify.Message{
		Title: "node offline",
		Body:  "node-1 stopped responding",
		Level: notify.LevelError,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	// Default card wraps as {msg_type:"interactive", card:{...}}.
	var probe struct {
		MsgType string `json:"msg_type"`
	}
	_ = json.Unmarshal(receivedBody, &probe)
	if probe.MsgType != "interactive" {
		t.Errorf("default path should produce msg_type=interactive, got %q (body=%s)", probe.MsgType, receivedBody)
	}
}

func TestFeishu_CustomTemplateRendersAndSends(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		receivedBody, _ = io.ReadAll(req.Body)
		w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	// Admin wants a simple text-card layout with title interpolated.
	tmpl := `{"msg_type":"text","content":{"text":"[{{.Level}}] {{.Title}} — {{.Body}}"}}`
	c := NewFeishu(srv.URL, tmpl)
	err := c.Send(context.Background(), notify.Message{
		Title: "order completed",
		Body:  "user@example.com bought vip-monthly",
		Level: notify.LevelInfo,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	want := `{"msg_type":"text","content":{"text":"[info] order completed — user@example.com bought vip-monthly"}}`
	if string(receivedBody) != want {
		t.Errorf("template output mismatch:\ngot  %s\nwant %s", receivedBody, want)
	}
}

func TestFeishu_TemplateMissingKeyIsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	// Reference a nonexistent field — should render as empty,
	// NOT crash the dashboard's notification dispatch loop.
	tmpl := `{"text":"{{.Body}} {{.NotARealField}}"}`
	c := NewFeishu(srv.URL, tmpl)
	if err := c.Send(context.Background(), notify.Message{Body: "x"}); err != nil {
		t.Errorf("missing key should render to zero, got error: %v", err)
	}
}

func TestFeishu_BadTemplatePanicsAtConstruction(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on malformed template")
		}
		if !strings.Contains(r.(string), "FEISHU_CARD_TEMPLATE") {
			t.Errorf("panic should mention FEISHU_CARD_TEMPLATE for grep-ability, got %v", r)
		}
	}()
	_ = NewFeishu("https://example.com", "{{ .Body")
}
