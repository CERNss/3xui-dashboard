package webhook

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

func sampleEnvelope() Envelope {
	return Envelope{
		Version:   "v1",
		Event:     "order.completed",
		Timestamp: time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
		Data: map[string]any{
			"order_id":    int64(42),
			"user_id":     int64(7),
			"price_cents": int64(1500),
			"plan_name":   "vip-monthly",
		},
	}
}

func TestRenderWebhookPayload_EmptyTemplateUsesEnvelope(t *testing.T) {
	wh := &model.Webhook{BodyTemplate: ""}
	got, err := renderWebhookPayload(wh, sampleEnvelope())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var decoded Envelope
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("default output isn't JSON envelope: %v\n%s", err, got)
	}
	if decoded.Event != "order.completed" {
		t.Errorf("event = %q, want order.completed", decoded.Event)
	}
}

func TestRenderWebhookPayload_TextTemplate(t *testing.T) {
	wh := &model.Webhook{BodyTemplate: "event={{.Event}} order={{.Data.order_id}} plan={{.Data.plan_name}}"}
	got, err := renderWebhookPayload(wh, sampleEnvelope())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := "event=order.completed order=42 plan=vip-monthly"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderWebhookPayload_JSONTemplate(t *testing.T) {
	// Admin builds a Slack-block-style payload.
	wh := &model.Webhook{BodyTemplate: `{"text":"Order #{{.Data.order_id}} for ¥{{.Data.price_cents}}"}`}
	got, err := renderWebhookPayload(wh, sampleEnvelope())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(got), `"Order #42 for ¥1500"`) {
		t.Errorf("template did not substitute: %s", got)
	}
	var probe map[string]any
	if err := json.Unmarshal(got, &probe); err != nil {
		t.Errorf("rendered output isn't valid JSON: %v\n%s", err, got)
	}
}

func TestRenderWebhookPayload_BadTemplateErrors(t *testing.T) {
	wh := &model.Webhook{BodyTemplate: "{{ .Data.unclosed"}
	if _, err := renderWebhookPayload(wh, sampleEnvelope()); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestRenderWebhookPayload_MissingKeyIsZero(t *testing.T) {
	// "missingkey=zero" option means accessing a nonexistent key
	// emits "<no value>" / 0 rather than erroring — admins won't
	// have their delivery break because an event happens to omit
	// an optional field.
	wh := &model.Webhook{BodyTemplate: "user={{.Data.user_id}} reason={{.Data.reason}}"}
	env := sampleEnvelope()
	// Data has no `reason` key.
	got, err := renderWebhookPayload(wh, env)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.HasPrefix(string(got), "user=7 ") {
		t.Errorf("substitution wrong: %q", got)
	}
}

func TestNormalizeWebhook_DefaultsAndValidation(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		w := &model.Webhook{Method: "", TemplateFormat: ""}
		if err := normalizeWebhook(w); err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if w.Method != "POST" {
			t.Errorf("method = %q, want POST", w.Method)
		}
		if w.TemplateFormat != "json" {
			t.Errorf("template_format = %q, want json", w.TemplateFormat)
		}
	})
	t.Run("uppercases method", func(t *testing.T) {
		w := &model.Webhook{Method: "get"}
		if err := normalizeWebhook(w); err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if w.Method != "GET" {
			t.Errorf("method = %q, want GET", w.Method)
		}
	})
	t.Run("rejects bad method", func(t *testing.T) {
		w := &model.Webhook{Method: "OPTIONS"}
		if err := normalizeWebhook(w); err == nil {
			t.Fatal("expected error for OPTIONS, got nil")
		}
	})
	t.Run("rejects bad template_format", func(t *testing.T) {
		w := &model.Webhook{TemplateFormat: "xml"}
		if err := normalizeWebhook(w); err == nil {
			t.Fatal("expected error for xml, got nil")
		}
	})
	t.Run("rejects malformed template at create time", func(t *testing.T) {
		w := &model.Webhook{BodyTemplate: "{{ .Data."}
		if err := normalizeWebhook(w); err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}
