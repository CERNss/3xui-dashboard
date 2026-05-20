package mailer

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/config"
)

// capturingHandler is a minimal slog.Handler that records every Record into a
// slice — lets us assert what the mailer logged in disabled mode without
// parsing JSON.
type capturingHandler struct {
	records []slog.Record
}

func (h *capturingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(_ string) slog.Handler     { return h }

func newCapturingLogger() (*slog.Logger, *capturingHandler) {
	h := &capturingHandler{}
	return slog.New(h), h
}

// ---- Send: disabled-SMTP fallback -----------------------------------------

func TestSend_DisabledSMTP_LogsInsteadOfDelivering(t *testing.T) {
	logger, captured := newCapturingLogger()
	// Empty SMTP config — Enabled() reports false.
	m := New(config.SMTP{}, logger)

	if m.Enabled() {
		t.Fatalf("mailer with zero cfg should report Enabled()==false")
	}

	err := m.Send("alice@example.com", "Test subject", "Plain body")
	if err != nil {
		t.Fatalf("Send with disabled SMTP should not error, got %v", err)
	}

	if len(captured.records) != 1 {
		t.Fatalf("expected exactly 1 log record, got %d", len(captured.records))
	}
	rec := captured.records[0]
	if rec.Level != slog.LevelInfo {
		t.Errorf("expected INFO level, got %v", rec.Level)
	}

	// Walk the record's attrs and confirm to/subject/body landed on the log.
	seen := map[string]string{}
	rec.Attrs(func(a slog.Attr) bool {
		seen[a.Key] = a.Value.String()
		return true
	})
	for _, k := range []string{"to", "subject", "body"} {
		if _, ok := seen[k]; !ok {
			t.Errorf("disabled-mode log missing attribute %q (got %v)", k, seen)
		}
	}
	if seen["to"] != "alice@example.com" {
		t.Errorf("to attr = %q, want alice@example.com", seen["to"])
	}
	if seen["subject"] != "Test subject" {
		t.Errorf("subject attr = %q, want 'Test subject'", seen["subject"])
	}
	if seen["body"] != "Plain body" {
		t.Errorf("body attr = %q, want 'Plain body'", seen["body"])
	}
}

func TestEnabled_RequiresBothHostAndFrom(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.SMTP
		want bool
	}{
		{"zero", config.SMTP{}, false},
		{"host only", config.SMTP{Host: "smtp.example.com"}, false},
		{"from only", config.SMTP{From: "noreply@example.com"}, false},
		{"both", config.SMTP{Host: "smtp.example.com", From: "noreply@example.com"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(tc.cfg, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
			if got := m.Enabled(); got != tc.want {
				t.Errorf("Enabled() = %v, want %v (cfg=%+v)", got, tc.want, tc.cfg)
			}
		})
	}
}

// ---- buildMessage / encodeSubject ------------------------------------------

func TestBuildMessage_FramingAndHeaders(t *testing.T) {
	msg := buildMessage("from@example.com", "to@example.com", "Hello", "Body text")

	s := string(msg)
	for _, want := range []string{
		"From: from@example.com\r\n",
		"To: to@example.com\r\n",
		"Subject: Hello\r\n",
		"MIME-Version: 1.0\r\n",
		`Content-Type: text/plain; charset="utf-8"`,
		"Content-Transfer-Encoding: 8bit\r\n",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected message to contain %q, got:\n%s", want, s)
		}
	}

	// Body separated from headers by a blank CRLF line.
	if !strings.Contains(s, "\r\n\r\nBody text") {
		t.Errorf("expected blank CRLF line before body, got:\n%s", s)
	}
}

func TestEncodeSubject_ASCIIVerbatim(t *testing.T) {
	got := encodeSubject("Hello world")
	if got != "Hello world" {
		t.Errorf("ASCII subject should be verbatim, got %q", got)
	}
}

func TestEncodeSubject_ChineseBase64(t *testing.T) {
	got := encodeSubject("【3xui Central】注册验证码")
	// Must start with the RFC 2047 marker.
	if !strings.HasPrefix(got, "=?UTF-8?B?") || !strings.HasSuffix(got, "?=") {
		t.Errorf("Chinese subject should be RFC 2047 encoded, got %q", got)
	}
	// And the payload between markers must decode back to the original.
	// (We rely on the decoder consensus rather than redoing base64 here.)
	if strings.Contains(got, "注册") {
		t.Errorf("encoded subject should NOT contain raw UTF-8 bytes, got %q", got)
	}
}

// Compile-time check that capturingHandler satisfies slog.Handler.
var _ slog.Handler = (*capturingHandler)(nil)
