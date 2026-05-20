package channels

import (
	"context"
	"fmt"
	"strings"

	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/service/notify"
)

// Email wraps the existing mailer.Mailer as a notify.Channel.
// `opsRecipient` is the fallback address ops alerts (events
// without a per-user Recipient) get sent to. Empty fallback means
// the channel silently drops ops messages — operators see a
// startup warning when EMAIL is routed but MAIL_OPS_ADDRESS isn't
// set.
type Email struct {
	mailer       *mailer.Mailer
	opsRecipient string
}

// NewEmail builds the adapter. If `mailer` is nil OR not configured,
// the returned channel reports Enabled()=false and Send is a no-op.
func NewEmail(m *mailer.Mailer, opsRecipient string) *Email {
	return &Email{mailer: m, opsRecipient: opsRecipient}
}

func (e *Email) Name() string { return "email" }

func (e *Email) Enabled() bool {
	return e.mailer != nil && e.mailer.Enabled()
}

func (e *Email) Send(_ context.Context, msg notify.Message) error {
	if !e.Enabled() {
		return nil
	}
	to := msg.Recipient
	if to == "" {
		to = e.opsRecipient
	}
	if to == "" {
		// Routed here but no recipient configured. Don't error —
		// the operator's choice — just skip.
		return nil
	}

	// Subject = "[Level] Title" so a quick mailbox scan sorts by
	// severity. The plain-text body adds fields as "key: value" lines.
	subject := fmt.Sprintf("[%s] %s", strings.ToUpper(msg.Level.String()), msg.Title)
	var b strings.Builder
	b.WriteString(msg.Body)
	if len(msg.Fields) > 0 {
		b.WriteString("\n\n")
		for _, f := range msg.Fields {
			b.WriteString(f.Key)
			b.WriteString(": ")
			b.WriteString(f.Value)
			b.WriteByte('\n')
		}
	}
	if msg.URL != "" {
		b.WriteString("\n")
		b.WriteString(msg.URL)
		b.WriteByte('\n')
	}
	return e.mailer.Send(to, subject, b.String())
}
