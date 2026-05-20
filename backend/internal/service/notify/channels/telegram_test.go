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

func TestTelegram_DisabledNoConfig(t *testing.T) {
	c := NewTelegram("", "")
	if c.Enabled() {
		t.Error("empty config should be disabled")
	}
	if err := c.Send(context.Background(), notify.Message{}); err != nil {
		t.Errorf("Send on disabled should no-op, got %v", err)
	}
}

func TestTelegram_SendBuildsRequest(t *testing.T) {
	var capturedBody string
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	t.Cleanup(server.Close)

	c := NewTelegram("TESTTOKEN", "-100123")
	c.SetAPIBase(server.URL)

	err := c.Send(context.Background(), notify.Message{
		Level: notify.LevelError,
		Title: "Node offline",
		Body:  "n1.example.com unreachable",
		Fields: []notify.Field{
			{Key: "Node", Value: "tokyo-1"},
			{Key: "Last seen", Value: "10 min ago"},
		},
		URL: "https://panel.example.com/admin/nodes/1",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if capturedPath != "/botTESTTOKEN/sendMessage" {
		t.Errorf("path = %s", capturedPath)
	}

	var req telegramRequest
	if err := json.Unmarshal([]byte(capturedBody), &req); err != nil {
		t.Fatalf("parse captured body: %v", err)
	}
	if req.ChatID != "-100123" {
		t.Errorf("ChatID = %q", req.ChatID)
	}
	if req.ParseMode != "HTML" {
		t.Errorf("ParseMode = %q", req.ParseMode)
	}
	if !req.DisableWebPagePreview {
		t.Error("DisableWebPagePreview should be true")
	}
	if !strings.Contains(req.Text, "🔴") {
		t.Errorf("text missing error emoji: %s", req.Text)
	}
	if !strings.Contains(req.Text, "<b>Node offline</b>") {
		t.Errorf("text missing bold title: %s", req.Text)
	}
	if !strings.Contains(req.Text, `<a href="https://panel.example.com/admin/nodes/1">查看详情</a>`) {
		t.Errorf("link missing: %s", req.Text)
	}
	if !strings.Contains(req.Text, "Node: tokyo-1") {
		t.Errorf("field missing: %s", req.Text)
	}
}

func TestTelegram_APIErrorSurfaced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	t.Cleanup(server.Close)
	c := NewTelegram("X", "Y")
	c.SetAPIBase(server.URL)

	err := c.Send(context.Background(), notify.Message{Title: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("error: %v", err)
	}
}

func TestTelegram_HTMLEscape(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	c := NewTelegram("X", "Y")
	c.SetAPIBase(server.URL)
	_ = c.Send(context.Background(), notify.Message{
		Title: "<script>alert(1)</script>",
		Body:  "5 < 10 & 10 > 5",
	})
	// Parse the JSON payload + check the text field directly so we're
	// not comparing against JSON-escaped substrings.
	var req telegramRequest
	if err := json.Unmarshal([]byte(capturedBody), &req); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if strings.Contains(req.Text, "<script>") {
		t.Errorf("unescaped <script> in text: %s", req.Text)
	}
	if !strings.Contains(req.Text, "&lt;script&gt;") {
		t.Errorf("escaped form missing in text: %s", req.Text)
	}
	if !strings.Contains(req.Text, "5 &lt; 10 &amp; 10 &gt; 5") {
		t.Errorf("body escaping wrong: %s", req.Text)
	}
}
