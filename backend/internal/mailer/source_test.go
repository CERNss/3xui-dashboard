package mailer

import (
	"context"
	"errors"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

// fakeReader returns canned values per key; missing keys yield the fallback,
// mirroring the typed getters on repository.SettingRepo.
type fakeReader struct {
	strs    map[string]string
	ints    map[string]int64
	secrets map[string]string
	secErr  error
}

func (f fakeReader) GetString(_ context.Context, key, fb string) (string, error) {
	if v, ok := f.strs[key]; ok {
		return v, nil
	}
	return fb, nil
}

func (f fakeReader) GetInt(_ context.Context, key string, fb int64) (int64, error) {
	if v, ok := f.ints[key]; ok {
		return v, nil
	}
	return fb, nil
}

func (f fakeReader) GetSecret(_ context.Context, key, fb string) (string, error) {
	if f.secErr != nil {
		return fb, f.secErr
	}
	if v, ok := f.secrets[key]; ok {
		return v, nil
	}
	return fb, nil
}

func TestSettingsSource_DBOverridesFallback(t *testing.T) {
	r := fakeReader{
		strs:    map[string]string{model.SettingSMTPHost: "db.example.com", model.SettingSMTPFrom: "db@x.com"},
		ints:    map[string]int64{model.SettingSMTPPort: 465},
		secrets: map[string]string{model.SettingSMTPPassword: "db-pass"},
	}
	fb := SMTPConfig{Host: "env.example.com", Port: 587, From: "env@x.com", Username: "envuser", Password: "env-pass"}

	got, err := NewSettingsSource(r, fb).SMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("SMTPConfig: %v", err)
	}
	if got.Host != "db.example.com" {
		t.Errorf("Host = %q, want the DB override", got.Host)
	}
	if got.Port != 465 {
		t.Errorf("Port = %d, want 465", got.Port)
	}
	if got.From != "db@x.com" {
		t.Errorf("From = %q, want the DB override", got.From)
	}
	if got.Username != "envuser" {
		t.Errorf("Username = %q, want the env fallback (no DB row)", got.Username)
	}
	if got.Password != "db-pass" {
		t.Errorf("Password = %q, want the DB secret", got.Password)
	}
	if !got.Enabled() {
		t.Error("config with host + from should be Enabled()")
	}
}

func TestSettingsSource_FallsBackWhenEmpty(t *testing.T) {
	fb := SMTPConfig{Host: "env.example.com", Port: 587, From: "env@x.com", Password: "env-pass"}
	got, err := NewSettingsSource(fakeReader{}, fb).SMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("SMTPConfig: %v", err)
	}
	if got != fb {
		t.Errorf("an empty reader should yield the fallback verbatim, got %+v", got)
	}
}

func TestSettingsSource_SecretErrorFallsBackToEnvPassword(t *testing.T) {
	r := fakeReader{secErr: errors.New("encrypted value but no cipher configured")}
	fb := SMTPConfig{Host: "h", From: "f", Password: "env-pass"}

	got, err := NewSettingsSource(r, fb).SMTPConfig(context.Background())
	if err != nil {
		t.Fatalf("a secret read error should not fail the whole resolve, got %v", err)
	}
	if got.Password != "env-pass" {
		t.Errorf("Password = %q, want the env fallback when the secret read errors", got.Password)
	}
}

func TestStaticSource_ReturnsItsConfig(t *testing.T) {
	cfg := SMTPConfig{Host: "h", Port: 25, From: "f"}
	got, err := NewStaticSource(cfg).SMTPConfig(context.Background())
	if err != nil || got != cfg {
		t.Fatalf("StaticSource = (%+v, %v), want (%+v, nil)", got, err, cfg)
	}
}
