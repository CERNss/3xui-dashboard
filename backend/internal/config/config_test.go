package config

import (
	"os"
	"strings"
	"testing"
)

// withEnv runs fn with the given environment, then restores.
func withEnv(t *testing.T, env map[string]string, fn func()) {
	t.Helper()
	prev := map[string]string{}
	clear := func() {
		for k := range env {
			if v, ok := prev[k]; ok {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}
	for k, v := range env {
		if pv, ok := os.LookupEnv(k); ok {
			prev[k] = pv
		}
		os.Setenv(k, v)
	}
	defer clear()
	fn()
}

func TestLoad_FailsOnMissingRequired(t *testing.T) {
	// Wipe every required key so Load reports them all at once.
	for _, k := range []string{"DATABASE_URL", "JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD"} {
		os.Unsetenv(k)
	}
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error on missing required keys, got nil")
	}
	got := err.Error()
	for _, k := range []string{"DATABASE_URL", "JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD"} {
		if !strings.Contains(got, k) {
			t.Errorf("error %q should mention %s", got, k)
		}
	}
}

func TestLoad_FullEnvLoadsCleanly(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":   "postgres://x@x/x",
		"JWT_SECRET":     "secret",
		"ADMIN_USERNAME": "admin",
		"ADMIN_PASSWORD": "pw",
		"ENV":            "dev",
	}, func() {
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.DB.URL == "" || cfg.Auth.JWTSecret == "" {
			t.Errorf("Load returned zero-valued fields: %+v", cfg)
		}
		if cfg.Server.LogFormat != "text" {
			t.Errorf("dev should default LogFormat to text, got %q", cfg.Server.LogFormat)
		}
	})
}

func TestLoad_PartialOIDCIsAnError(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":   "x",
		"JWT_SECRET":     "x",
		"ADMIN_USERNAME": "x",
		"ADMIN_PASSWORD": "x",
		"OIDC_ISSUER":    "https://idp.example.com",
		// CLIENT_ID, CLIENT_SECRET, REDIRECT_URL intentionally absent.
	}, func() {
		_, err := Load("")
		if err == nil {
			t.Fatal("partial OIDC config should fail; got nil")
		}
		if !strings.Contains(err.Error(), "OIDC_CLIENT_ID") {
			t.Errorf("error should name the missing OIDC fields: %v", err)
		}
	})
}
