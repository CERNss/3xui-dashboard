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
	// ADMIN_PASSWORD is intentionally excluded — Load() now auto-generates it
	// when blank (printed to stderr for first-boot bootstrap).
	required := []string{"DATABASE_URL", "JWT_SECRET", "ADMIN_USERNAME"}
	for _, k := range required {
		os.Unsetenv(k)
	}
	os.Unsetenv("ADMIN_PASSWORD")
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error on missing required keys, got nil")
	}
	got := err.Error()
	for _, k := range required {
		if !strings.Contains(got, k) {
			t.Errorf("error %q should mention %s", got, k)
		}
	}
	if strings.Contains(got, "ADMIN_PASSWORD") {
		t.Errorf("ADMIN_PASSWORD should auto-generate now, not error: %v", err)
	}
}

func TestLoad_GeneratesAdminPasswordWhenBlank(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":   "postgres://x@x/x",
		"JWT_SECRET":     "secret",
		"ADMIN_USERNAME": "admin@example.com",
		// ADMIN_PASSWORD intentionally absent
		"ENV": "dev",
	}, func() {
		os.Unsetenv("ADMIN_PASSWORD")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Admin.Password == "" {
			t.Error("expected auto-generated password, got empty")
		}
		if len(cfg.Admin.Password) < 16 {
			t.Errorf("auto-generated password too short: %d chars", len(cfg.Admin.Password))
		}
	})
}

func TestLoad_FullEnvLoadsCleanly(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":   "postgres://x@x/x",
		"JWT_SECRET":     "secret",
		"ADMIN_USERNAME": "admin",
		"ADMIN_PASSWORD": "pw",
		"ENV":            "dev",
		"LOG_FORMAT":     "",
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
		if cfg.Bootstrap.NodesJSON != "" {
			t.Errorf("Bootstrap.NodesJSON = %q, want empty default", cfg.Bootstrap.NodesJSON)
		}
	})
}

func TestLoad_BootstrapNodesJSON(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":         "postgres://x@x/x",
		"JWT_SECRET":           "secret",
		"ADMIN_USERNAME":       "admin",
		"ADMIN_PASSWORD":       "pw",
		"BOOTSTRAP_NODES_JSON": `[{"name":"edge","access_url":"https://node.example.com/panel","api_token":"tok"}]`,
	}, func() {
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Bootstrap.NodesJSON == "" {
			t.Fatal("Bootstrap.NodesJSON should load from env")
		}
	})
}

func TestLoad_PublicRegistrationDefaultsEnabled(t *testing.T) {
	prev, hadPrev := os.LookupEnv("PUBLIC_REGISTRATION")
	os.Unsetenv("PUBLIC_REGISTRATION")
	defer func() {
		if hadPrev {
			os.Setenv("PUBLIC_REGISTRATION", prev)
		} else {
			os.Unsetenv("PUBLIC_REGISTRATION")
		}
	}()

	withEnv(t, map[string]string{
		"DATABASE_URL":   "postgres://x@x/x",
		"JWT_SECRET":     "secret",
		"ADMIN_USERNAME": "admin",
		"ADMIN_PASSWORD": "pw",
	}, func() {
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !cfg.PublicRegistration {
			t.Fatal("PUBLIC_REGISTRATION should default to true")
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
