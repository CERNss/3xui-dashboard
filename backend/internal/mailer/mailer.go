// Package mailer wraps SMTP sending into a single Send call. Stdlib only —
// no external mail SDK. SMTP config is read from a ConfigSource per send,
// so an admin editing it in the panel takes effect without a restart. If
// SMTP is not configured, Send falls back to a no-op that logs the
// would-be message to stderr so dev workflows can still verify codes
// without real email.
package mailer

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"time"
)

// SMTPConfig is the resolved configuration a single send uses — a
// snapshot read from the ConfigSource at send time.
type SMTPConfig struct {
	Host     string
	Port     int
	From     string
	Username string
	Password string
}

// Enabled reports whether the snapshot is usable for real delivery.
func (c SMTPConfig) Enabled() bool { return c.Host != "" && c.From != "" }

// ConfigSource supplies the current SMTP config. Read per send so panel
// edits are picked up live; implementations should be cheap (a settings
// point-lookup) since this runs on every send and Enabled() probe.
type ConfigSource interface {
	SMTPConfig(ctx context.Context) (SMTPConfig, error)
}

// Mailer sends emails over SMTP. Zero-value is unusable — construct via New().
type Mailer struct {
	src    ConfigSource
	logger *slog.Logger
}

// New constructs a Mailer that reads its SMTP config from src on every
// send, so runtime panel edits take effect immediately.
func New(src ConfigSource, logger *slog.Logger) *Mailer {
	return &Mailer{src: src, logger: logger}
}

// Enabled reports whether real SMTP delivery is currently available. Uses
// a background context — it's a cheap status probe, not request-scoped I/O.
func (m *Mailer) Enabled() bool {
	cfg, err := m.src.SMTPConfig(context.Background())
	return err == nil && cfg.Enabled()
}

// Send delivers a UTF-8 plain-text email. When SMTP is disabled (or its
// config can't be read), the message is logged instead — sufficient for
// dev where the operator can copy verification codes from stderr.
func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	cfg, err := m.src.SMTPConfig(ctx)
	if err != nil {
		m.logger.Error("mailer: could not read SMTP config — dropping message",
			"to", to, "subject", subject, "err", err.Error())
		return nil
	}
	if !cfg.Enabled() {
		m.logger.Info("mailer: SMTP disabled — pretending to send",
			"to", to, "subject", subject, "body", body)
		return nil
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	msg := buildMessage(cfg.From, to, subject, body)

	// Connection: STARTTLS or implicit TLS based on port.
	// Most providers use 587 with STARTTLS; 465 wants implicit TLS.
	if cfg.Port == 465 {
		return m.sendImplicitTLS(cfg, addr, to, msg)
	}
	return m.sendSTARTTLS(cfg, addr, to, msg)
}

func (m *Mailer) sendSTARTTLS(cfg SMTPConfig, addr, to string, msg []byte) error {
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	// stdlib smtp.SendMail handles STARTTLS upgrade transparently
	// when the server advertises it.
	if err := smtp.SendMail(addr, auth, cfg.From, []string{to}, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

func (m *Mailer) sendImplicitTLS(cfg SMTPConfig, addr, to string, msg []byte) error {
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = c.Quit() }()

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(cfg.From); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return nil
}

// buildMessage produces an RFC 5322 message. Body is plain UTF-8.
func buildMessage(from, to, subject, body string) []byte {
	header := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + encodeSubject(subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n" +
		"\r\n"
	return []byte(header + body)
}

// encodeSubject RFC 2047-encodes a header value if it contains non-ASCII.
// Most subjects we send are mixed Chinese/English, so we encode unconditionally
// when any byte is high-bit set.
func encodeSubject(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			// =?UTF-8?B?...?= base64-encoded form
			return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
		}
	}
	return s
}

// ErrSMTPNotConfigured signals callers that real delivery is unavailable.
// Mailer.Send itself does NOT return this — it silently logs instead — but
// service-level callers may want to check Enabled() and surface this when
// SMTP is required but missing.
var ErrSMTPNotConfigured = errors.New("smtp not configured")
