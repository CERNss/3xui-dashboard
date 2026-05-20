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

func TestFeishu_DisabledNoConfig(t *testing.T) {
	c := NewFeishu("", "")
	if c.Enabled() {
		t.Error("empty webhook should be disabled")
	}
	if err := c.Send(context.Background(), notify.Message{}); err != nil {
		t.Errorf("Send on disabled should no-op, got %v", err)
	}
}

func TestFeishu_SendBuildsCard(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	t.Cleanup(server.Close)

	c := NewFeishu(server.URL, "")
	err := c.Send(context.Background(), notify.Message{
		Level: notify.LevelWarn,
		Title: "Pending order expired",
		Body:  "Order #42 wasn't paid within 15 minutes.",
		Fields: []notify.Field{
			{Key: "User", Value: "alice@example.com"},
			{Key: "Plan", Value: "Pro 30d"},
		},
		URL: "https://panel.example.com/admin/orders/42",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var msg feishuMessage
	if err := json.Unmarshal([]byte(capturedBody), &msg); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if msg.MsgType != "interactive" {
		t.Errorf("msg_type = %q", msg.MsgType)
	}
	if msg.Card.Header.Template != "yellow" {
		t.Errorf("header template = %q, want yellow for warn", msg.Card.Header.Template)
	}
	if msg.Card.Header.Title.Content != "Pending order expired" {
		t.Errorf("header title = %q", msg.Card.Header.Title.Content)
	}
	// Body element + hr + fields + action = 4 elements
	if len(msg.Card.Elements) != 4 {
		t.Errorf("elements count = %d, want 4 (body + hr + fields + action)", len(msg.Card.Elements))
	}
	// Find the action element to verify URL
	var foundAction bool
	for _, el := range msg.Card.Elements {
		if el.Tag == "action" && len(el.Actions) > 0 && el.Actions[0].URL != "" {
			foundAction = true
			if el.Actions[0].URL != "https://panel.example.com/admin/orders/42" {
				t.Errorf("action URL = %q", el.Actions[0].URL)
			}
		}
	}
	if !foundAction {
		t.Errorf("no action element in card: %s", capturedBody)
	}
}

func TestFeishu_APIErrorSurfaced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":9499,"msg":"Bad Request"}`))
	}))
	t.Cleanup(server.Close)

	c := NewFeishu(server.URL, "")
	err := c.Send(context.Background(), notify.Message{Title: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "9499") {
		t.Errorf("error missing code: %v", err)
	}
	if !strings.Contains(err.Error(), "Bad Request") {
		t.Errorf("error missing msg: %v", err)
	}
}

func TestFeishu_TemplateByLevel(t *testing.T) {
	cases := []struct {
		level notify.Level
		want  string
	}{
		{notify.LevelInfo, "green"},
		{notify.LevelWarn, "yellow"},
		{notify.LevelError, "red"},
	}
	for _, c := range cases {
		if got := c.level.FeishuTemplate(); got != c.want {
			t.Errorf("%s.FeishuTemplate() = %q, want %q", c.level, got, c.want)
		}
	}
}
