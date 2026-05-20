// Package config loads runtime configuration from environment variables
// (and an optional .env file) into a typed Config struct. Required keys
// are validated up front so the process fails fast on startup.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
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
	Alipay Alipay
	Stripe Stripe

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

	// AllowedOrigins gates CORS. Empty list (the default) means
	// "permissive" — echo the request's Origin back, no credentials.
	// A comma-separated list of exact origins narrows to a closed
	// set and enables Access-Control-Allow-Credentials. The literal
	// "*" anywhere in the list also means "permissive".
	AllowedOrigins []string
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

	// UI hints — surfaced to the login page so the operator can brand the
	// "使用 X 登录" button. Both default to empty; frontend falls back to
	// the issuer hostname / a generic globe icon when missing.
	DisplayName string // e.g. "集换社"
	IconURL     string // e.g. "https://cdn.example.com/logos/jihuanshe.svg"
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

// Alipay configures the 当面付 gateway. PrivateKey + AlipayPublicKey
// are PEM blocks the operator pastes from the alipay open-platform
// console (RSA2 keypair: we sign with PrivateKey, verify alipay's
// signatures with AlipayPublicKey).
type Alipay struct {
	AppID           string
	PrivateKey      string // PEM, our RSA2 private key
	AlipayPublicKey string // PEM, alipay's platform public key
	Gateway         string // defaults to https://openapi.alipay.com/gateway.do
	NotifyURL       string // public URL alipay POSTs to; e.g. https://panel.example.com/api/public/payment/alipay/notify
	ReturnURL       string // optional; only used by the WAP flow which we don't implement (#future)
}

// Enabled reports whether the alipay gateway is fully configured.
// Notify URL is technically optional (the poll job covers the gap)
// but strongly recommended — without it the user waits up to 30s for
// the success flip even on a clean payment.
func (a Alipay) Enabled() bool {
	return a.AppID != "" && a.PrivateKey != "" && a.AlipayPublicKey != ""
}

// Stripe configures the Checkout Sessions gateway. Stripe distinguishes
// test vs live by the secret key prefix (sk_test_... vs sk_live_...),
// so there's no separate sandbox URL — the API host is always
// api.stripe.com.
type Stripe struct {
	SecretKey            string // sk_live_... or sk_test_...
	WebhookSecret        string // whsec_... — used to HMAC-verify inbound webhooks
	Currency             string // ISO 4217 lowercase, defaults to "usd"
	SuccessURL           string // where Stripe redirects after success — usually /portal/orders?stripe=ok
	CancelURL            string // where Stripe redirects on cancel — usually /portal/plans?stripe=cancel
	SessionExpiryMinutes int    // 0 → 30 minutes (Stripe's API default is 24h but we expire eagerly)
}

// Enabled reports whether the stripe gateway is fully configured.
func (s Stripe) Enabled() bool {
	return s.SecretKey != "" && s.WebhookSecret != ""
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
	v.SetDefault("ALIPAY_GATEWAY", "https://openapi.alipay.com/gateway.do")
	v.SetDefault("STRIPE_CURRENCY", "usd")
	v.SetDefault("STRIPE_SESSION_EXPIRY_MINUTES", 30)

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
			AllowedOrigins:  splitCSV(v.GetString("ALLOWED_ORIGINS")),
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
			DisplayName:  v.GetString("OIDC_DISPLAY_NAME"),
			IconURL:      v.GetString("OIDC_ICON_URL"),
		},
		SMTP: SMTP{
			Host:     v.GetString("SMTP_HOST"),
			Port:     v.GetInt("SMTP_PORT"),
			Username: v.GetString("SMTP_USERNAME"),
			Password: v.GetString("SMTP_PASSWORD"),
			From:     v.GetString("SMTP_FROM"),
			UseTLS:   v.GetBool("SMTP_USE_TLS"),
		},
		Alipay: Alipay{
			AppID:           v.GetString("ALIPAY_APP_ID"),
			PrivateKey:      v.GetString("ALIPAY_PRIVATE_KEY"),
			AlipayPublicKey: v.GetString("ALIPAY_PUBLIC_KEY"),
			Gateway:         v.GetString("ALIPAY_GATEWAY"),
			NotifyURL:       v.GetString("ALIPAY_NOTIFY_URL"),
			ReturnURL:       v.GetString("ALIPAY_RETURN_URL"),
		},
		Stripe: Stripe{
			SecretKey:            v.GetString("STRIPE_SECRET_KEY"),
			WebhookSecret:        v.GetString("STRIPE_WEBHOOK_SECRET"),
			Currency:             v.GetString("STRIPE_CURRENCY"),
			SuccessURL:           v.GetString("STRIPE_SUCCESS_URL"),
			CancelURL:            v.GetString("STRIPE_CANCEL_URL"),
			SessionExpiryMinutes: v.GetInt("STRIPE_SESSION_EXPIRY_MINUTES"),
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

	// Bootstrap admin password — if operator left ADMIN_PASSWORD blank,
	// generate a fresh random one and print it to stderr so they can
	// find it in the first-boot logs. On subsequent boots the value must
	// be set in .env (or env) for the same credentials to keep working.
	if cfg.Admin.Password == "" {
		pw, err := generateAdminPassword()
		if err != nil {
			return nil, fmt.Errorf("generate admin password: %w", err)
		}
		cfg.Admin.Password = pw
		fmt.Fprintf(os.Stderr,
			"\n"+
				"============================================================\n"+
				"  ADMIN_PASSWORD was not set in env / .env file.\n"+
				"  Generated a fresh random one for this boot:\n"+
				"\n"+
				"      ADMIN_PASSWORD=%s\n"+
				"\n"+
				"  Save it into your .env file to keep the same credentials\n"+
				"  across restarts; otherwise a new password is generated on\n"+
				"  every startup and previous tokens stay valid until expiry.\n"+
				"============================================================\n\n",
			pw,
		)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// generateAdminPassword returns a 24-char URL-safe random password.
// Uses crypto/rand so it's safe to publish into logs as a one-shot
// bootstrap secret — operator is expected to copy it into .env.
func generateAdminPassword() (string, error) {
	var b [18]byte // 18 bytes → 24 base64 chars, no padding
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
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
	// ADMIN_PASSWORD is no longer required — Load() auto-generates one if
	// blank and prints it to stderr. We still leave the validate() hook in
	// place so future required fields slot in here.

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
