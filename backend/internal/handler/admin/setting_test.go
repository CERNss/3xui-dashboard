package admin

import (
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestValidateNewUserSettings(t *testing.T) {
	if err := validate(model.SettingNewUserInitialBalanceCents, "1234"); err != nil {
		t.Fatalf("initial balance cents rejected: %v", err)
	}
	if err := validate(model.SettingNewUserInitialBalanceCents, "-1"); err == nil {
		t.Fatal("negative initial balance cents should be rejected")
	}
	if err := validate(model.SettingNewUserPlanIDs, "1, 2, 99"); err != nil {
		t.Fatalf("starter plan ids rejected: %v", err)
	}
	if err := validate(model.SettingNewUserPlanIDs, "1, nope"); err == nil {
		t.Fatal("bad starter plan id should be rejected")
	}
}

func TestValidateRegistrationSettings(t *testing.T) {
	if err := validate(model.SettingEmailVerificationRequired, "true"); err != nil {
		t.Fatalf("email verification bool rejected: %v", err)
	}
	if err := validate(model.SettingEmailVerificationRequired, "sometimes"); err == nil {
		t.Fatal("bad email verification bool should be rejected")
	}
}

func TestValidateOIDCSettings(t *testing.T) {
	if err := validate(model.SettingOIDCIssuer, "https://auth.example.com"); err != nil {
		t.Fatalf("issuer URL rejected: %v", err)
	}
	if err := validate(model.SettingOIDCRedirectURL, "http://localhost:8080/oidc/callback"); err != nil {
		t.Fatalf("redirect URL rejected: %v", err)
	}
	if err := validate(model.SettingOIDCIconURL, "ftp://auth.example.com/icon.svg"); err == nil {
		t.Fatal("non-http icon URL should be rejected")
	}
	if err := validate(model.SettingOIDCIssuer, "not a url"); err == nil {
		t.Fatal("bad issuer URL should be rejected")
	}
	if err := validate(model.SettingOIDCScopes, "openid,profile,email"); err != nil {
		t.Fatalf("scopes rejected: %v", err)
	}
	if err := validate(model.SettingOIDCScopes, "openid, bad scope"); err == nil {
		t.Fatal("scope with internal whitespace should be rejected")
	}
}
