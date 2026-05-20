// Package mailer wraps SMTP sending into a single Send call. Stdlib only —
// no external mail SDK. If SMTP is not configured (cfg.SMTP.Enabled() ==
// false), Send falls back to a no-op that logs the would-be message to
// stderr so dev workflows can still verify codes without real email.
package mailer

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/cern/3xui-dashboard/internal/config"
)

// Mailer sends emails over SMTP. Zero-value is unusable — construct via New().
type Mailer struct {
	cfg    config.SMTP
	logger *slog.Logger
}

// New constructs a Mailer. The cfg is captured by value, so subsequent
// changes to the Config struct don't take effect mid-process.
func New(cfg config.SMTP, logger *slog.Logger) *Mailer {
	return &Mailer{cfg: cfg, logger: logger}
}

// Enabled reports whether real SMTP delivery is available.
func (m *Mailer) Enabled() bool { return m.cfg.Enabled() }

// Send delivers a UTF-8 plain-text email. When SMTP is disabled, the
// message is logged at INFO level instead — sufficient for dev where
// the operator can copy verification codes from stderr.
func (m *Mailer) Send(to, subject, body string) error {
	if !m.cfg.Enabled() {
		m.logger.Info("mailer: SMTP disabled — pretending to send",
			"to", to, "subject", subject, "body", body)
		return nil
	}

	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	msg := buildMessage(m.cfg.From, to, subject, body)

	// Connection: STARTTLS or implicit TLS based on port + flag.
	// Most providers use 587 with STARTTLS; 465 wants implicit TLS.
	if m.cfg.Port == 465 {
		return m.sendImplicitTLS(addr, to, msg)
	}
	return m.sendSTARTTLS(addr, to, msg)
}

func (m *Mailer) sendSTARTTLS(addr, to string, msg []byte) error {
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	// stdlib smtp.SendMail handles STARTTLS upgrade transparently
	// when the server advertises it.
	if err := smtp.SendMail(addr, auth, m.cfg.From, []string{to}, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

func (m *Mailer) sendImplicitTLS(addr, to string, msg []byte) error {
	tlsCfg := &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = c.Quit() }()

	if m.cfg.Username != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(m.cfg.From); err != nil {
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
