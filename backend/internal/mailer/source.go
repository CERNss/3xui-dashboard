package mailer

import (
	"context"

	"github.com/cern/3xui-dashboard/internal/model"
)

// SettingsReader is the subset of repository.SettingRepo the
// settings-backed source needs. Declared here so the mailer package
// doesn't import the repository package (which would invert the layering).
type SettingsReader interface {
	GetString(ctx context.Context, key, fallback string) (string, error)
	GetInt(ctx context.Context, key string, fallback int64) (int64, error)
	GetSecret(ctx context.Context, key, fallback string) (string, error)
}

// SettingsSource resolves SMTP config from the settings table, falling
// back to the supplied snapshot (env / config.yaml) for any key without
// a panel override. The password is read via GetSecret so an encrypted
// row is transparently decrypted.
type SettingsSource struct {
	r        SettingsReader
	fallback SMTPConfig
}

// NewSettingsSource builds a ConfigSource backed by r. fallback supplies
// first-boot/env defaults for keys the admin hasn't set in the panel.
func NewSettingsSource(r SettingsReader, fallback SMTPConfig) *SettingsSource {
	return &SettingsSource{r: r, fallback: fallback}
}

// StaticSource is a ConfigSource that always returns the same snapshot —
// useful for tests and for callers with a fixed (non-runtime) config.
type StaticSource struct{ cfg SMTPConfig }

// NewStaticSource returns a ConfigSource that always yields cfg.
func NewStaticSource(cfg SMTPConfig) StaticSource { return StaticSource{cfg: cfg} }

// SMTPConfig implements ConfigSource.
func (s StaticSource) SMTPConfig(context.Context) (SMTPConfig, error) { return s.cfg, nil }

// SMTPConfig implements ConfigSource. Read errors degrade to the fallback
// value for that field rather than failing the whole send — the typed
// getters already fold "absent" into the fallback, so only a malformed
// row or DB error lands here, and a dropped notification beats a hard
// failure in a verification flow.
func (s *SettingsSource) SMTPConfig(ctx context.Context) (SMTPConfig, error) {
	host, _ := s.r.GetString(ctx, model.SettingSMTPHost, s.fallback.Host)
	port, _ := s.r.GetInt(ctx, model.SettingSMTPPort, int64(s.fallback.Port))
	from, _ := s.r.GetString(ctx, model.SettingSMTPFrom, s.fallback.From)
	user, _ := s.r.GetString(ctx, model.SettingSMTPUsername, s.fallback.Username)
	pass, err := s.r.GetSecret(ctx, model.SettingSMTPPassword, s.fallback.Password)
	if err != nil {
		pass = s.fallback.Password
	}
	return SMTPConfig{
		Host:     host,
		Port:     int(port),
		From:     from,
		Username: user,
		Password: pass,
	}, nil
}
