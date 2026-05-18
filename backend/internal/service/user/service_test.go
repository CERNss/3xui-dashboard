package user

import (
	"testing"

	"github.com/cern/3xui-dashboard/internal/config"
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
		"a@example.com":   true,
		"A@EXAMPLE.COM":   true,
		"a@PARTNER.io":    true,
		"a@notallowed.net": false,
		"no-at-here":      false,
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
