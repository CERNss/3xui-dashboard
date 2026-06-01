package channels

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/notify"
)

type fakeNotifySettings struct {
	strs    map[string]string
	secrets map[string]string
}

func (f fakeNotifySettings) GetString(_ context.Context, key, fb string) (string, error) {
	if v, ok := f.strs[key]; ok {
		return v, nil
	}
	return fb, nil
}

func (f fakeNotifySettings) GetSecret(_ context.Context, key, fb string) (string, error) {
	if v, ok := f.secrets[key]; ok {
		return v, nil
	}
	return fb, nil
}

func newTestProvider(r SettingsReader, d NotifyDefaults) *SettingsProvider {
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := mailer.New(mailer.NewStaticSource(mailer.SMTPConfig{}), lg)
	return NewSettingsProvider(r, m, d, lg)
}

func notifyChannelByName(chs []notify.Channel, name string) notify.Channel {
	for _, c := range chs {
		if c != nil && c.Name() == name {
			return c
		}
	}
	return nil
}

func TestNotifyProvider_DBOverridesFallback(t *testing.T) {
	r := fakeNotifySettings{
		strs: map[string]string{
			model.SettingNotifyRoutes:         "node.offline:telegram",
			model.SettingNotifyTelegramChatID: "12345",
		},
		secrets: map[string]string{
			model.SettingNotifyTelegramBotToken: "db-bot-token",
		},
	}
	defaults := NotifyDefaults{TelegramBotToken: "env-token", TelegramChatID: "999"}

	router, chs := newTestProvider(r, defaults).Resolve(context.Background())

	if tg := notifyChannelByName(chs, "telegram"); tg == nil || !tg.Enabled() {
		t.Fatal("telegram should be enabled (DB bot token + chat ID)")
	}
	if got := router.Channels("node.offline"); len(got) != 1 || got[0] != "telegram" {
		t.Errorf("route node.offline = %v, want [telegram]", got)
	}
}

func TestNotifyProvider_FallsBackWhenEmpty(t *testing.T) {
	defaults := NotifyDefaults{
		Routes:            "node.offline:discord",
		DiscordWebhookURL: "https://discord.test/webhook/abc",
	}
	router, chs := newTestProvider(fakeNotifySettings{}, defaults).Resolve(context.Background())

	if dc := notifyChannelByName(chs, "discord"); dc == nil || !dc.Enabled() {
		t.Error("discord should be enabled from the fallback webhook URL")
	}
	if tg := notifyChannelByName(chs, "telegram"); tg == nil || tg.Enabled() {
		t.Error("telegram should be disabled with no token / chat")
	}
	if got := router.Channels("node.offline"); len(got) != 1 || got[0] != "discord" {
		t.Errorf("fallback route = %v, want [discord]", got)
	}
}

func TestNotifyProvider_BadStoredRoutesFallBackToDefault(t *testing.T) {
	r := fakeNotifySettings{strs: map[string]string{model.SettingNotifyRoutes: "missing-colon"}}
	defaults := NotifyDefaults{Routes: "node.offline:feishu"}

	router, _ := newTestProvider(r, defaults).Resolve(context.Background())
	if got := router.Channels("node.offline"); len(got) != 1 || got[0] != "feishu" {
		t.Errorf("invalid stored routes should fall back to defaults, got %v", got)
	}
}
