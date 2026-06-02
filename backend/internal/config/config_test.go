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
	_, err := Load("", "")
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

func TestLoad_ConfigYAMLBaseEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	yamlPath := dir + "/config.yaml"
	// read_timeout proves the yaml layer is read; listen_addr is set in
	// yaml too but a real env var below must override it.
	if err := os.WriteFile(yamlPath, []byte("read_timeout: 5s\nlisten_addr: \":9090\"\n"), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	withEnv(t, map[string]string{
		"DATABASE_URL":   "postgres://x@x/x",
		"JWT_SECRET":     "secret",
		"ADMIN_USERNAME": "admin@example.com",
		"ADMIN_PASSWORD": "pw",
		"ENV":            "dev",
		"LISTEN_ADDR":    ":7000", // real env overrides yaml's :9090
	}, func() {
		cfg, err := Load("", yamlPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Server.ReadTimeout.String() != "5s" {
			t.Errorf("read_timeout from yaml = %v, want 5s", cfg.Server.ReadTimeout)
		}
		if cfg.Server.ListenAddr != ":7000" {
			t.Errorf("listen_addr = %q, want :7000 (real env must override yaml)", cfg.Server.ListenAddr)
		}
	})
}

func TestLoad_MissingConfigYAMLIsNotFatal(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":   "postgres://x@x/x",
		"JWT_SECRET":     "secret",
		"ADMIN_USERNAME": "admin@example.com",
		"ADMIN_PASSWORD": "pw",
		"ENV":            "dev",
	}, func() {
		if _, err := Load("", "/nonexistent/path/config.yaml"); err != nil {
			t.Fatalf("missing config.yaml should not be fatal: %v", err)
		}
	})
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
		cfg, err := Load("", "")
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
		"DATABASE_URL":                 "postgres://x@x/x",
		"JWT_SECRET":                   "secret",
		"ADMIN_USERNAME":               "admin",
		"ADMIN_PASSWORD":               "pw",
		"ENV":                          "dev",
		"LOG_FORMAT":                   "",
		"SUBSCRIPTION_PUBLIC_BASE_URL": "https://sub.example.com/panel/",
	}, func() {
		cfg, err := Load("", "")
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
		if cfg.Subscription.PublicBaseURL != "https://sub.example.com/panel" {
			t.Errorf("Subscription.PublicBaseURL = %q, want trimmed public URL", cfg.Subscription.PublicBaseURL)
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
		cfg, err := Load("", "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Bootstrap.NodesJSON == "" {
			t.Fatal("Bootstrap.NodesJSON should load from env")
		}
	})
}

func TestLoad_PublicRegistrationDefaultsDisabled(t *testing.T) {
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
		cfg, err := Load("", "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.PublicRegistration {
			t.Fatal("PUBLIC_REGISTRATION should default to false (safer; admin enables it in the panel)")
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
		_, err := Load("", "")
		if err == nil {
			t.Fatal("partial OIDC config should fail; got nil")
		}
		if !strings.Contains(err.Error(), "OIDC_CLIENT_ID") {
			t.Errorf("error should name the missing OIDC fields: %v", err)
		}
	})
}
