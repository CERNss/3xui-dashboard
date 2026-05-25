package user

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
)

func newCfg(public bool, allowlist ...string) *config.Config {
	return &config.Config{
		PublicRegistration:   public,
		EmailDomainAllowlist: allowlist,
	}
}

func TestEmailDomainAllowed_EmptyMeansAll(t *testing.T) {
	s := &Service{cfg: newCfg(true)}
	if !s.emailDomainAllowed(nil, "a@anything.example") {
		t.Error("empty allowlist should permit any domain")
	}
}

func TestEmailDomainAllowed_MatchesAllowlist(t *testing.T) {
	s := &Service{cfg: newCfg(true, "example.com", "partner.io")}
	cases := map[string]bool{
		"a@example.com":    true,
		"A@EXAMPLE.COM":    true,
		"a@PARTNER.io":     true,
		"a@notallowed.net": false,
		"no-at-here":       false,
	}
	for email, want := range cases {
		if got := s.emailDomainAllowed(nil, email); got != want {
			t.Errorf("emailDomainAllowed(%q) = %v, want %v", email, got, want)
		}
	}
}

func TestPublicRegistrationGate(t *testing.T) {
	// With nil settings repo, fallback is cfg.PublicRegistration.
	off := &Service{cfg: newCfg(false)}
	if got, _ := off.publicRegistrationEnabled(nil); got {
		t.Error("expected registration disabled when cfg false + no settings")
	}
	on := &Service{cfg: newCfg(true)}
	if got, _ := on.publicRegistrationEnabled(nil); !got {
		t.Error("expected registration enabled when cfg true + no settings")
	}
}

func TestEmailVerificationRequiredFallback(t *testing.T) {
	s := &Service{cfg: newCfg(true)}
	if got, err := s.EmailVerificationRequired(context.Background(), true); err != nil || !got {
		t.Fatalf("fallback true = %v, err=%v", got, err)
	}
	if got, err := s.EmailVerificationRequired(context.Background(), false); err != nil || got {
		t.Fatalf("fallback false = %v, err=%v", got, err)
	}
}

func TestGenerateSubID_Unique(t *testing.T) {
	a, _ := generateSubID()
	b, _ := generateSubID()
	if a == "" || b == "" || a == b {
		t.Errorf("sub_ids not random: a=%q b=%q", a, b)
	}
	if len(a) != 32 {
		t.Errorf("sub_id len = %d, want 32", len(a))
	}
}

func TestRegister_EmailVerifiedFollowsInput(t *testing.T) {
	_, svc := setupOIDCDB(t)
	svc.cfg.PublicRegistration = true
	ctx := context.Background()

	verified, err := svc.Register(ctx, RegisterInput{
		Email:    "verified@example.com",
		Password: "password123",
		Verified: true,
	})
	if err != nil {
		t.Fatalf("register verified: %v", err)
	}
	if !verified.EmailVerified {
		t.Error("expected email_verified=true when RegisterInput.Verified is true")
	}

	unverified, err := svc.Register(ctx, RegisterInput{
		Email:    "dev@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register unverified: %v", err)
	}
	if unverified.EmailVerified {
		t.Error("expected email_verified=false by default")
	}
}

func TestBindEmail_StoresUnverified(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	sub := "bind-email-sub"
	u := &model.User{
		OIDCSubject: &sub,
		SubID:       "bind-email-sub-id",
		Status:      model.UserStatusActive,
	}
	if err := svc.users.Create(ctx, u); err != nil {
		t.Fatalf("seed oidc-only user: %v", err)
	}

	if err := svc.BindEmail(ctx, u.ID, "bind@example.com"); err != nil {
		t.Fatalf("bind email: %v", err)
	}
	got, err := svc.users.Get(ctx, u.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.Email == nil || *got.Email != "bind@example.com" {
		t.Fatalf("email not bound: %v", got.Email)
	}
	if got.EmailVerified {
		t.Error("expected bind email to leave email_verified=false")
	}
}

func TestAdminUpdate_CanUpdateEmailAndPassword(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	u, err := svc.AdminCreate(ctx, AdminCreateInput{
		Email:    "old@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}

	email := "new@example.com"
	password := "newpass123"
	balance := int64(12345)
	verified := false
	updated, err := svc.AdminUpdate(ctx, u.ID, AdminUpdateInput{
		Email:         &email,
		Password:      &password,
		EmailVerified: &verified,
		BalanceCents:  &balance,
	})
	if err != nil {
		t.Fatalf("admin update: %v", err)
	}
	if updated.Email == nil || *updated.Email != "new@example.com" {
		t.Fatalf("email not updated: %v", updated.Email)
	}
	if updated.EmailVerified {
		t.Fatal("email_verified should follow the admin patch")
	}
	if updated.BalanceCents != 12345 {
		t.Fatalf("balance_cents = %d, want 12345", updated.BalanceCents)
	}

	got, err := svc.users.Get(ctx, u.ID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if got.PasswordHash == nil {
		t.Fatal("password_hash not set")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*got.PasswordHash), []byte(password)); err != nil {
		t.Fatalf("password hash does not match new password: %v", err)
	}
}
