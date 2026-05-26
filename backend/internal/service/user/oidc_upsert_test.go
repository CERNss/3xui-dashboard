// Integration tests for the P5 OIDC account-identity contract.
package user

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

func setupOIDCDB(t *testing.T) (*gorm.DB, *Service) {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping oidc upsert DB tests")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{DB: config.DB{URL: dbURL, MaxOpenConns: 5, MaxIdleConns: 2}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := repository.Open(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if err := repository.MigrateUp(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close(db) })

	userRepo := repository.NewUserRepo(db)
	settingRepo := repository.NewSettingRepo(db)
	bus := event.New()
	svc := New(userRepo, settingRepo, bus, &config.Config{}, logger)
	svc.cfg.OIDC = config.OIDC{
		Issuer:       "https://idp.example.com",
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURL:  "https://dashboard.example.com/oidc/callback",
		Scopes:       []string{"openid", "profile", "email"},
		DisplayName:  "Example SSO",
	}
	return db, svc
}

func seedEmailUser(t *testing.T, svc *Service, email string, password string) *model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &model.User{
		Status:        model.UserStatusActive,
		SubID:         "seed-" + email,
		Email:         &email,
		PasswordHash:  string(hash),
		EmailVerified: true,
	}
	if err := svc.users.Create(context.Background(), u); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func seedOIDCIdentity(t *testing.T, svc *Service, userID int64, providerKey, subject, providerEmail string) {
	t.Helper()
	if err := svc.users.UpsertOIDCProvider(context.Background(), &model.OIDCProvider{
		ProviderKey:  providerKey,
		DisplayName:  "Provider " + providerKey,
		Issuer:       "https://" + providerKey + ".example.com",
		ClientID:     "client-" + providerKey,
		ClientSecret: "secret",
		RedirectURL:  "https://dashboard.example.com/oidc/callback",
		Scopes:       model.StringSlice{"openid", "email", "profile"},
		Enabled:      true,
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	if err := svc.users.LinkOIDCIdentityToUser(context.Background(), &model.UserOIDCIdentity{
		UserID:                userID,
		ProviderKey:           providerKey,
		Subject:               subject,
		ProviderEmail:         providerEmail,
		ProviderEmailVerified: true,
	}); err != nil {
		t.Fatalf("seed identity: %v", err)
	}
}

func requireIdentity(t *testing.T, svc *Service, providerKey, subject string) *model.UserOIDCIdentity {
	t.Helper()
	identity, err := svc.users.FindOIDCIdentity(context.Background(), providerKey, subject)
	if err != nil {
		t.Fatalf("find identity: %v", err)
	}
	if identity == nil {
		t.Fatalf("missing identity %s/%s", providerKey, subject)
	}
	return identity
}

func TestEffectiveOIDC_UsesRuntimeSettings(t *testing.T) {
	_, svc := setupOIDCDB(t)
	svc.cfg.OIDC = config.OIDC{
		Issuer:       "https://env.example.com",
		ClientID:     "env-client",
		ClientSecret: "env-secret",
		RedirectURL:  "https://env.example.com/callback",
		Scopes:       []string{"openid", "email"},
		DisplayName:  "Env SSO",
	}
	ctx := context.Background()
	if err := svc.settings.Set(ctx, model.SettingOIDCIssuer, "https://runtime.example.com"); err != nil {
		t.Fatalf("set issuer: %v", err)
	}
	if err := svc.settings.Set(ctx, model.SettingOIDCClientID, "runtime-client"); err != nil {
		t.Fatalf("set client id: %v", err)
	}
	if err := svc.settings.Set(ctx, model.SettingOIDCScopes, "openid,profile,email"); err != nil {
		t.Fatalf("set scopes: %v", err)
	}
	if err := svc.settings.Set(ctx, model.SettingOIDCDisplayName, ""); err != nil {
		t.Fatalf("set blank display name: %v", err)
	}

	got, err := svc.effectiveOIDC(ctx)
	if err != nil {
		t.Fatalf("effectiveOIDC: %v", err)
	}
	if got.Issuer != "https://runtime.example.com" {
		t.Fatalf("issuer = %q", got.Issuer)
	}
	if got.ClientID != "runtime-client" {
		t.Fatalf("client id = %q", got.ClientID)
	}
	if got.ClientSecret != "env-secret" {
		t.Fatalf("client secret fallback = %q", got.ClientSecret)
	}
	if got.DisplayName != "Env SSO" {
		t.Fatalf("display name fallback = %q", got.DisplayName)
	}
	if len(got.Scopes) != 3 || got.Scopes[1] != "profile" {
		t.Fatalf("scopes = %#v", got.Scopes)
	}
}

func TestUpsertOIDC_UnlinkedSubjectReturnsPendingCreate(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()

	res, err := svc.upsertOIDCUser(ctx, "sub-new-1", "provider@example.com", true, "/portal")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if res.User != nil {
		t.Fatalf("unlinked OIDC callback should not auto-create user: %+v", res.User)
	}
	if res.Pending == nil || res.Pending.ExistingUser {
		t.Fatalf("expected create-account pending response, got %+v", res.Pending)
	}
	if res.Pending.ProviderKey != defaultOIDCProviderKey {
		t.Fatalf("provider key = %q", res.Pending.ProviderKey)
	}
	if res.Pending.Email != "provider@example.com" || !res.Pending.EmailVerified {
		t.Fatalf("provider email not preserved: %+v", res.Pending)
	}
}

func TestOIDCCreateAccount_CreatesLocalUserAndIdentityWithDifferentEmail(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	res, err := svc.upsertOIDCUser(ctx, "sub-new-2", "provider@example.com", true, "/portal/profile")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	u, redirectAfter, err := svc.OIDCCreateAccount(ctx, OIDCCreateAccountInput{
		PendingToken: res.Pending.Token,
		DisplayName:  "Local Alice",
		Email:        "local@example.com",
		Password:     "password123",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if redirectAfter != "/portal/profile" {
		t.Fatalf("redirect_after = %q", redirectAfter)
	}
	if u.Email == nil || *u.Email != "local@example.com" {
		t.Fatalf("local email = %v", u.Email)
	}
	if u.DisplayName != "Local Alice" {
		t.Fatalf("display_name = %q", u.DisplayName)
	}
	if !u.EmailVerified {
		t.Fatal("local email should be verified by completion")
	}
	if u.PasswordHash == "" || u.PasswordHash == model.DisabledPasswordHash {
		t.Fatalf("password_hash must be a real local credential")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("password123")); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
	identity := requireIdentity(t, svc, defaultOIDCProviderKey, "sub-new-2")
	if identity.UserID != u.ID {
		t.Fatalf("identity user_id = %d, want %d", identity.UserID, u.ID)
	}
	if identity.ProviderEmail != "provider@example.com" || !identity.ProviderEmailVerified {
		t.Fatalf("provider email not stored: %+v", identity)
	}
}

func TestUpsertOIDC_LinkedIdentityLogsInAndRefreshesProviderEmail(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	u := seedEmailUser(t, svc, "alice@example.com", "password123")
	seedOIDCIdentity(t, svc, u.ID, defaultOIDCProviderKey, "sub-linked", "old-provider@example.com")

	res, err := svc.upsertOIDCUser(ctx, "sub-linked", "new-provider@example.com", true, "/portal/orders")
	if err != nil {
		t.Fatalf("upsert linked: %v", err)
	}
	if res.User == nil || res.User.ID != u.ID {
		t.Fatalf("expected linked user login, got %+v", res)
	}
	if res.RedirectAfter != "/portal/orders" {
		t.Fatalf("redirect_after = %q", res.RedirectAfter)
	}
	identity := requireIdentity(t, svc, defaultOIDCProviderKey, "sub-linked")
	if identity.ProviderEmail != "new-provider@example.com" {
		t.Fatalf("provider email not refreshed: %+v", identity)
	}
}

func TestOIDCBindExisting_RequiresPasswordThenLinks(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	seeded := seedEmailUser(t, svc, "alice@example.com", "password123")

	res, err := svc.upsertOIDCUser(ctx, "sub-link", "alice@example.com", true, "/portal/subscription")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if res.Pending == nil || !res.Pending.ExistingUser {
		t.Fatalf("expected existing-user pending decision, got %+v", res.Pending)
	}

	_, _, err = svc.OIDCBindExisting(ctx, OIDCBindExistingInput{
		PendingToken: res.Pending.Token,
		Password:     "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password should be rejected, got %v", err)
	}

	got, redirectAfter, err := svc.OIDCBindExisting(ctx, OIDCBindExistingInput{
		PendingToken: res.Pending.Token,
		Password:     "password123",
	})
	if err != nil {
		t.Fatalf("bind existing: %v", err)
	}
	if redirectAfter != "/portal/subscription" {
		t.Fatalf("redirect_after = %q", redirectAfter)
	}
	if got.ID != seeded.ID {
		t.Fatalf("expected linked to user %d, got %d", seeded.ID, got.ID)
	}
	identity := requireIdentity(t, svc, defaultOIDCProviderKey, "sub-link")
	if identity.UserID != seeded.ID {
		t.Fatalf("identity user_id = %d, want %d", identity.UserID, seeded.ID)
	}
}

func TestUpsertOIDC_RejectsUnverifiedOrMissingProviderEmail(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()

	_, err := svc.upsertOIDCUser(ctx, "sub-noemail", "", false, "")
	if !errors.Is(err, ErrOIDCEmailRequired) {
		t.Fatalf("want ErrOIDCEmailRequired, got %v", err)
	}
	_, err = svc.upsertOIDCUser(ctx, "sub-unverified", "alice@example.com", false, "")
	if !errors.Is(err, ErrOIDCEmailUnverified) {
		t.Fatalf("want ErrOIDCEmailUnverified, got %v", err)
	}
}

func TestUpsertOIDC_RejectsDisallowedProviderEmail(t *testing.T) {
	_, svc := setupOIDCDB(t)
	svc.cfg.EmailDomainAllowlist = []string{"example.com"}
	ctx := context.Background()

	_, err := svc.upsertOIDCUser(ctx, "sub-new-deny", "alice@blocked.test", true, "")
	if !errors.Is(err, ErrDomainNotAllowed) {
		t.Fatalf("want ErrDomainNotAllowed, got %v", err)
	}
}

func TestLinkOIDCToUser_AllowsDifferentProviderEmail(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	seeded := seedEmailUser(t, svc, "alice@example.com", "password123")

	res, err := svc.linkOIDCToUser(ctx, seeded.ID, "sub-profile-link", "provider@example.com", true, "/portal/profile")
	if err != nil {
		t.Fatalf("link oidc: %v", err)
	}
	if res.User == nil || res.User.ID != seeded.ID {
		t.Fatalf("expected same user, got %+v", res.User)
	}
	identity := requireIdentity(t, svc, defaultOIDCProviderKey, "sub-profile-link")
	if identity.ProviderEmail != "provider@example.com" {
		t.Fatalf("provider email = %q", identity.ProviderEmail)
	}
}

func TestLinkOIDCToUser_RejectsSubjectOwnedByAnotherUser(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	target := seedEmailUser(t, svc, "alice@example.com", "password123")
	owner := seedEmailUser(t, svc, "bob@example.com", "password123")
	seedOIDCIdentity(t, svc, owner.ID, defaultOIDCProviderKey, "sub-owned", "bob@example.com")

	_, err := svc.linkOIDCToUser(ctx, target.ID, "sub-owned", "alice@example.com", true, "")
	if !errors.Is(err, ErrOIDCEmailConflict) {
		t.Fatalf("want ErrOIDCEmailConflict, got %v", err)
	}
}

func TestListOIDCProviders_ReturnsLinkedStateForMultipleProviders(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	u := seedEmailUser(t, svc, "alice@example.com", "password123")
	for _, key := range []string{"google", "github"} {
		if err := svc.users.UpsertOIDCProvider(ctx, &model.OIDCProvider{
			ProviderKey:  key,
			DisplayName:  key,
			Issuer:       "https://" + key + ".example.com",
			ClientID:     "client-" + key,
			ClientSecret: "secret",
			RedirectURL:  "https://dashboard.example.com/oidc/callback",
			Scopes:       model.StringSlice{"openid", "email"},
			Enabled:      true,
		}); err != nil {
			t.Fatalf("provider %s: %v", key, err)
		}
	}
	seedOIDCIdentity(t, svc, u.ID, "google", "sub-google", "alice@gmail.example")

	providers, err := svc.ListOIDCProviders(ctx, u.ID)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	linked := map[string]bool{}
	for _, provider := range providers {
		linked[provider.ProviderKey] = provider.Linked
	}
	if !linked["google"] {
		t.Fatalf("google should be linked: %+v", providers)
	}
	if linked["github"] {
		t.Fatalf("github should not be linked: %+v", providers)
	}
}

func TestAdminList_MarksOIDCLinkedUsers(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	emailOnly := seedEmailUser(t, svc, "email-only@example.com", "password123")
	oidcUser := seedEmailUser(t, svc, "oidc-linked@example.com", "password123")
	seedOIDCIdentity(t, svc, oidcUser.ID, defaultOIDCProviderKey, "sub-admin-list", "provider@example.com")

	rows, err := svc.AdminList(ctx, 100, 0)
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	linkedByID := map[int64]bool{}
	for _, row := range rows {
		linkedByID[row.ID] = row.OIDCLinked
	}
	if linkedByID[emailOnly.ID] {
		t.Fatalf("email-only user marked OIDC-linked: %+v", linkedByID)
	}
	if !linkedByID[oidcUser.ID] {
		t.Fatalf("OIDC-linked user not marked: %+v", linkedByID)
	}
}
