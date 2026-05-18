// Package config loads runtime configuration from environment variables
// (and an optional .env file) into a typed Config struct. Required keys
// are validated up front so the process fails fast on startup.
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	Env    string // "dev" | "prod"
	Server Server
	DB     DB
	Auth   Auth
	Admin  Admin
	OIDC   OIDC
	SMTP   SMTP

	PublicRegistration   bool
	EmailDomainAllowlist []string
}

type Server struct {
	ListenAddr      string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	LogLevel        string // debug|info|warn|error
	LogFormat       string // json|text
}

type DB struct {
	URL          string // postgres://user:pass@host:5432/dbname?sslmode=disable
	MaxOpenConns int
	MaxIdleConns int
	MigrateOnBoot bool
}

type Auth struct {
	JWTSecret      string
	AccessTokenTTL time.Duration
}

type Admin struct {
	Username string
	Password string
}

// OIDC is fully optional. Enabled is computed: true iff Issuer + ClientID + ClientSecret + RedirectURL are all set.
type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string

	// Optional explicit endpoint overrides (skip discovery when set).
	AuthURL  string
	TokenURL string
	JWKSURL  string
	UserURL  string
}

// Enabled reports whether OIDC is configured.
func (o OIDC) Enabled() bool {
	return o.Issuer != "" && o.ClientID != "" && o.ClientSecret != "" && o.RedirectURL != ""
}

// SMTP is fully optional. Enabled is computed: true iff Host + From are both set.
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	UseTLS   bool
}

// Enabled reports whether SMTP is configured.
func (s SMTP) Enabled() bool {
	return s.Host != "" && s.From != ""
}

// Load reads configuration. If envFile is non-empty it is loaded as a
// dotenv-format file; values still get overridden by real environment
// variables (process env wins). Missing required keys produce a single
// aggregated error so operators see every problem at once.
func Load(envFile string) (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Defaults.
	v.SetDefault("ENV", "prod")
	v.SetDefault("LISTEN_ADDR", ":8080")
	v.SetDefault("READ_TIMEOUT", "15s")
	v.SetDefault("WRITE_TIMEOUT", "30s")
	v.SetDefault("SHUTDOWN_TIMEOUT", "20s")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "")  // resolved from ENV below
	v.SetDefault("DB_MAX_OPEN_CONNS", 25)
	v.SetDefault("DB_MAX_IDLE_CONNS", 5)
	v.SetDefault("DB_MIGRATE_ON_BOOT", true)
	v.SetDefault("ACCESS_TOKEN_TTL", "24h")
	v.SetDefault("OIDC_SCOPES", "openid,profile,email")
	v.SetDefault("SMTP_PORT", 587)
	v.SetDefault("SMTP_USE_TLS", true)
	v.SetDefault("PUBLIC_REGISTRATION", false)
	v.SetDefault("EMAIL_DOMAIN_ALLOWLIST", "")

	if envFile != "" {
		v.SetConfigFile(envFile)
		v.SetConfigType("env")
		// Missing file is not fatal: operators may rely on real env vars.
		if err := v.ReadInConfig(); err != nil {
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) {
				return nil, fmt.Errorf("read env file %q: %w", envFile, err)
			}
		}
	}

	cfg := &Config{
		Env: v.GetString("ENV"),
		Server: Server{
			ListenAddr:      v.GetString("LISTEN_ADDR"),
			ReadTimeout:     v.GetDuration("READ_TIMEOUT"),
			WriteTimeout:    v.GetDuration("WRITE_TIMEOUT"),
			ShutdownTimeout: v.GetDuration("SHUTDOWN_TIMEOUT"),
			LogLevel:        v.GetString("LOG_LEVEL"),
			LogFormat:       v.GetString("LOG_FORMAT"),
		},
		DB: DB{
			URL:           v.GetString("DATABASE_URL"),
			MaxOpenConns:  v.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:  v.GetInt("DB_MAX_IDLE_CONNS"),
			MigrateOnBoot: v.GetBool("DB_MIGRATE_ON_BOOT"),
		},
		Auth: Auth{
			JWTSecret:      v.GetString("JWT_SECRET"),
			AccessTokenTTL: v.GetDuration("ACCESS_TOKEN_TTL"),
		},
		Admin: Admin{
			Username: v.GetString("ADMIN_USERNAME"),
			Password: v.GetString("ADMIN_PASSWORD"),
		},
		OIDC: OIDC{
			Issuer:       v.GetString("OIDC_ISSUER"),
			ClientID:     v.GetString("OIDC_CLIENT_ID"),
			ClientSecret: v.GetString("OIDC_CLIENT_SECRET"),
			RedirectURL:  v.GetString("OIDC_REDIRECT_URL"),
			Scopes:       splitCSV(v.GetString("OIDC_SCOPES")),
			AuthURL:      v.GetString("OIDC_AUTH_URL"),
			TokenURL:     v.GetString("OIDC_TOKEN_URL"),
			JWKSURL:      v.GetString("OIDC_JWKS_URL"),
			UserURL:      v.GetString("OIDC_USERINFO_URL"),
		},
		SMTP: SMTP{
			Host:     v.GetString("SMTP_HOST"),
			Port:     v.GetInt("SMTP_PORT"),
			Username: v.GetString("SMTP_USERNAME"),
			Password: v.GetString("SMTP_PASSWORD"),
			From:     v.GetString("SMTP_FROM"),
			UseTLS:   v.GetBool("SMTP_USE_TLS"),
		},
		PublicRegistration:   v.GetBool("PUBLIC_REGISTRATION"),
		EmailDomainAllowlist: splitCSV(v.GetString("EMAIL_DOMAIN_ALLOWLIST")),
	}

	// LOG_FORMAT defaults to text in dev, json in prod.
	if cfg.Server.LogFormat == "" {
		if cfg.Env == "dev" {
			cfg.Server.LogFormat = "text"
		} else {
			cfg.Server.LogFormat = "json"
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// MustLoad is a convenience for main(): it Loads and panics on error.
// The returned config is non-nil.
func MustLoad(envFile string) *Config {
	cfg, err := Load(envFile)
	if err != nil {
		panic(err)
	}
	return cfg
}

func (c *Config) validate() error {
	var missing []string
	require := func(key, value string) {
		if value == "" {
			missing = append(missing, key)
		}
	}
	require("DATABASE_URL", c.DB.URL)
	require("JWT_SECRET", c.Auth.JWTSecret)
	require("ADMIN_USERNAME", c.Admin.Username)
	require("ADMIN_PASSWORD", c.Admin.Password)

	// Partial OIDC config is a misconfiguration — either all four or none.
	oidcSet := []string{
		c.OIDC.Issuer, c.OIDC.ClientID, c.OIDC.ClientSecret, c.OIDC.RedirectURL,
	}
	oidcNames := []string{
		"OIDC_ISSUER", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL",
	}
	var oidcCount int
	for _, v := range oidcSet {
		if v != "" {
			oidcCount++
		}
	}
	if oidcCount > 0 && oidcCount < len(oidcSet) {
		var partial []string
		for i, v := range oidcSet {
			if v == "" {
				partial = append(partial, oidcNames[i])
			}
		}
		missing = append(missing, partial...)
	}

	if len(missing) > 0 {
		return fmt.Errorf("config: missing required key(s): %s", strings.Join(missing, ", "))
	}

	switch c.Server.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: invalid LOG_LEVEL %q (want debug|info|warn|error)", c.Server.LogLevel)
	}
	switch c.Server.LogFormat {
	case "json", "text":
	default:
		return fmt.Errorf("config: invalid LOG_FORMAT %q (want json|text)", c.Server.LogFormat)
	}
	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
