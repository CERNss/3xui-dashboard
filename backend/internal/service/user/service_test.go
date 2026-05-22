package user

import (
	"context"
	"testing"

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
