package channels

import (
	"context"
	"log/slog"

	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/notify"
)

// SettingsReader is the subset of repository.SettingRepo the provider
// needs. Declared here so this package doesn't import repository.
type SettingsReader interface {
	GetString(ctx context.Context, key, fallback string) (string, error)
	GetSecret(ctx context.Context, key, fallback string) (string, error)
}

// NotifyDefaults are the first-boot / env fallbacks (from cfg.Notify),
// used for any key the admin hasn't overridden in the panel. Kept as a
// plain struct so this package doesn't import config.
type NotifyDefaults struct {
	Routes             string
	OpsRecipient       string
	TelegramBotToken   string
	TelegramChatID     string
	DiscordWebhookURL  string
	FeishuWebhookURL   string
	FeishuCardTemplate string
}

// SettingsProvider resolves the notify routing + channels from the
// settings table on each dispatch (panel edits are picked up live),
// falling back to the supplied env defaults. It builds the channels
// here — this package already imports notify, so notify can't build
// them itself without an import cycle. Implements notify.ConfigProvider.
type SettingsProvider struct {
	r        SettingsReader
	mailer   *mailer.Mailer
	defaults NotifyDefaults
	log      *slog.Logger
}

// NewSettingsProvider builds the provider. mailer is shared with the
// user-facing messages surface (same SMTP config); it backs the email
// channel here.
func NewSettingsProvider(r SettingsReader, m *mailer.Mailer, d NotifyDefaults, lg *slog.Logger) *SettingsProvider {
	return &SettingsProvider{r: r, mailer: m, defaults: d, log: lg.With(slog.String("component", "notify.provider"))}
}

// Resolve implements notify.ConfigProvider.
func (p *SettingsProvider) Resolve(ctx context.Context) (*notify.Router, []notify.Channel) {
	get := func(key, fb string) string {
		v, _ := p.r.GetString(ctx, key, fb)
		return v
	}
	getSecret := func(key, fb string) string {
		v, err := p.r.GetSecret(ctx, key, fb)
		if err != nil {
			return fb
		}
		return v
	}

	routesRaw := get(model.SettingNotifyRoutes, p.defaults.Routes)
	router, err := notify.ParseRoutes(routesRaw)
	if err != nil {
		// Stored routes are validated on write, so this is unexpected;
		// fall back to the env defaults (then empty) rather than drop
		// every event.
		p.log.Warn("invalid notify routes; using defaults", "value", routesRaw, "err", err.Error())
		if router, err = notify.ParseRoutes(p.defaults.Routes); err != nil {
			router, _ = notify.ParseRoutes("")
		}
	}

	chs := []notify.Channel{
		NewEmail(p.mailer, get(model.SettingNotifyOpsRecipient, p.defaults.OpsRecipient)),
		NewTelegram(
			getSecret(model.SettingNotifyTelegramBotToken, p.defaults.TelegramBotToken),
			get(model.SettingNotifyTelegramChatID, p.defaults.TelegramChatID),
		),
		NewDiscord(getSecret(model.SettingNotifyDiscordWebhookURL, p.defaults.DiscordWebhookURL)),
		NewFeishu(
			getSecret(model.SettingNotifyFeishuWebhookURL, p.defaults.FeishuWebhookURL),
			get(model.SettingNotifyFeishuCardTemplate, p.defaults.FeishuCardTemplate),
		),
	}
	return router, chs
}
