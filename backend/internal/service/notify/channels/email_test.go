package channels

import (
	"io"
	"log/slog"
	"testing"

	"github.com/cern/3xui-dashboard/internal/mailer"
)

func TestEmailChannel_EnabledRequiresMailerAndRecipient(t *testing.T) {
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	enabled := mailer.New(mailer.NewStaticSource(mailer.SMTPConfig{Host: "smtp.test", From: "ops@test"}), lg)
	disabled := mailer.New(mailer.NewStaticSource(mailer.SMTPConfig{}), lg) // no host/from → not enabled

	cases := []struct {
		name      string
		mailer    *mailer.Mailer
		recipient string
		want      bool
	}{
		{"mailer + recipient", enabled, "ops@example.com", true},
		{"mailer, no recipient", enabled, "", false}, // can't deliver → disabled, not a silent no-op
		{"disabled mailer", disabled, "ops@example.com", false},
		{"nil mailer", nil, "ops@example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewEmail(tc.mailer, tc.recipient).Enabled(); got != tc.want {
				t.Errorf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}
