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

func TestDiscord_DisabledNoConfig(t *testing.T) {
	c := NewDiscord("")
	if c.Enabled() {
		t.Error("empty webhook should be disabled")
	}
	if err := c.Send(context.Background(), notify.Message{}); err != nil {
		t.Errorf("Send on disabled should no-op, got %v", err)
	}
}

func TestDiscord_SendBuildsEmbed(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.WriteHeader(http.StatusNoContent) // Discord returns 204
	}))
	t.Cleanup(server.Close)

	c := NewDiscord(server.URL)
	err := c.Send(context.Background(), notify.Message{
		Level:     notify.LevelError,
		Title:     "Node offline",
		Body:      "tokyo-1 unreachable for 5 minutes",
		EventType: "node.offline",
		Fields: []notify.Field{
			{Key: "Node", Value: "tokyo-1"},
			{Key: "Last seen", Value: "10 min ago"},
		},
		URL: "https://panel.example.com/admin/nodes/1",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload discordWebhook
	if err := json.Unmarshal([]byte(capturedBody), &payload); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if len(payload.Embeds) != 1 {
		t.Fatalf("embeds count = %d", len(payload.Embeds))
	}
	em := payload.Embeds[0]
	if em.Title != "Node offline" {
		t.Errorf("title = %q", em.Title)
	}
	if em.Color != 0xE74C3C {
		t.Errorf("color = 0x%X, want red 0xE74C3C", em.Color)
	}
	if em.URL != "https://panel.example.com/admin/nodes/1" {
		t.Errorf("url = %q", em.URL)
	}
	if len(em.Fields) != 2 {
		t.Errorf("fields count = %d", len(em.Fields))
	}
	if em.Footer == nil || em.Footer.Text != "node.offline" {
		t.Errorf("footer = %+v", em.Footer)
	}
}

func TestDiscord_HTTPErrorSurfaced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Invalid Webhook Token","code":50027}`))
	}))
	t.Cleanup(server.Close)
	c := NewDiscord(server.URL)
	err := c.Send(context.Background(), notify.Message{Title: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error missing status: %v", err)
	}
}

func TestDiscord_ColorByLevel(t *testing.T) {
	cases := []struct {
		level notify.Level
		want  int
	}{
		{notify.LevelInfo, 0x2ECC71},
		{notify.LevelWarn, 0xF1C40F},
		{notify.LevelError, 0xE74C3C},
	}
	for _, c := range cases {
		if got := c.level.DiscordColor(); got != c.want {
			t.Errorf("%s.DiscordColor() = 0x%X, want 0x%X", c.level, got, c.want)
		}
	}
}
