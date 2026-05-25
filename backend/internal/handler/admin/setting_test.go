package admin

import (
	"context"
	"strings"
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

func TestValidateOpsCollectionSettings(t *testing.T) {
	if err := validate(model.SettingOpsCollectEnabled, "true"); err != nil {
		t.Fatalf("ops collection bool rejected: %v", err)
	}
	if err := validate(model.SettingOpsCollectIntervalSeconds, "5"); err != nil {
		t.Fatalf("minimum ops interval rejected: %v", err)
	}
	if err := validate(model.SettingOpsCollectIntervalSeconds, "4"); err == nil {
		t.Fatal("ops interval below 5 seconds should be rejected")
	}
	if err := validate(model.SettingOpsCollectConcurrency, "8"); err != nil {
		t.Fatalf("ops concurrency rejected: %v", err)
	}
	if err := validate(model.SettingOpsCollectConcurrency, "0"); err == nil {
		t.Fatal("ops concurrency below 1 should be rejected")
	}
	if err := validate(model.SettingOpsCollectTimeoutSeconds, "12"); err != nil {
		t.Fatalf("ops timeout rejected: %v", err)
	}
	if err := validate(model.SettingOpsCollectTimeoutSeconds, "301"); err == nil {
		t.Fatal("ops timeout above 300 seconds should be rejected")
	}
	if err := validate(model.SettingOpsCollectRetryAttempts, "1"); err != nil {
		t.Fatalf("ops retry attempts rejected: %v", err)
	}
	if err := validate(model.SettingOpsCollectRetryAttempts, "-1"); err == nil {
		t.Fatal("ops retry attempts below 0 should be rejected")
	}
	if err := validate(model.SettingOpsRetentionSeconds, "21600"); err != nil {
		t.Fatalf("ops retention rejected: %v", err)
	}
	if err := validate(model.SettingOpsRetentionSeconds, "-1"); err == nil {
		t.Fatal("negative ops retention should be rejected")
	}
	if err := validate(model.SettingTrafficCollectEnabled, "true"); err != nil {
		t.Fatalf("traffic collection bool rejected: %v", err)
	}
	if err := validate(model.SettingTrafficCollectIntervalSecs, "5"); err != nil {
		t.Fatalf("minimum traffic interval rejected: %v", err)
	}
	if err := validate(model.SettingTrafficCollectIntervalSecs, "4"); err == nil {
		t.Fatal("traffic interval below 5 seconds should be rejected")
	}
	if err := validate(model.SettingTrafficCollectConcurrency, "8"); err != nil {
		t.Fatalf("traffic concurrency rejected: %v", err)
	}
	if err := validate(model.SettingTrafficCollectConcurrency, "65"); err == nil {
		t.Fatal("traffic concurrency above 64 should be rejected")
	}
	if err := validate(model.SettingTrafficCollectTimeoutSecs, "30"); err != nil {
		t.Fatalf("traffic timeout rejected: %v", err)
	}
	if err := validate(model.SettingTrafficCollectTimeoutSecs, "0"); err == nil {
		t.Fatal("traffic timeout below 1 second should be rejected")
	}
	if err := validate(model.SettingTrafficCollectRetryAttempts, "5"); err != nil {
		t.Fatalf("traffic retry attempts rejected: %v", err)
	}
	if err := validate(model.SettingTrafficCollectRetryAttempts, "6"); err == nil {
		t.Fatal("traffic retry attempts above 5 should be rejected")
	}
	if err := validate(model.SettingTrafficRetentionSeconds, "0"); err != nil {
		t.Fatalf("traffic retention disabled value rejected: %v", err)
	}
	if err := validate(model.SettingTrafficRetentionSeconds, "-1"); err == nil {
		t.Fatal("negative traffic retention should be rejected")
	}
}

func TestValidateCollectionTimeoutWithinInterval(t *testing.T) {
	h := &SettingHandler{}
	ctx := context.Background()

	if err := h.validateCollectionTimeoutWithinInterval(ctx,
		model.SettingOpsCollectTimeoutSeconds,
		"12",
		model.SettingOpsCollectIntervalSeconds,
		model.SettingOpsCollectTimeoutSeconds,
		60,
		12,
	); err != nil {
		t.Fatalf("timeout within interval rejected: %v", err)
	}
	if err := h.validateCollectionTimeoutWithinInterval(ctx,
		model.SettingOpsCollectTimeoutSeconds,
		"61",
		model.SettingOpsCollectIntervalSeconds,
		model.SettingOpsCollectTimeoutSeconds,
		60,
		12,
	); err == nil {
		t.Fatal("timeout greater than interval should be rejected")
	}
	if err := h.validateCollectionTimeoutWithinInterval(ctx,
		model.SettingOpsCollectIntervalSeconds,
		"5",
		model.SettingOpsCollectIntervalSeconds,
		model.SettingOpsCollectTimeoutSeconds,
		60,
		12,
	); err == nil {
		t.Fatal("interval below current timeout should be rejected")
	}
}

func TestValidateBrandSettings(t *testing.T) {
	if err := validate(model.SettingBrandTitle, "Acme Network"); err != nil {
		t.Fatalf("brand title rejected: %v", err)
	}
	if err := validate(model.SettingBrandFooter, "© 2026 Acme Network"); err != nil {
		t.Fatalf("brand footer rejected: %v", err)
	}
	if err := validate(model.SettingBrandTitle, strings.Repeat("x", 81)); err == nil {
		t.Fatal("overlong brand title should be rejected")
	}
}

func TestValidateSubscriptionTemplatePlaceholders(t *testing.T) {
	clash := "mixed-port: 7890\nproxies:\n  ${proxies}\nproxy-groups:\n  - name: 节点选择\n    type: select\n    proxies: [${proxy_names}]\nrules:\n  - MATCH,节点选择\n"
	if err := validate(model.SettingClashTemplateYAML, clash); err != nil {
		t.Fatalf("clash template with placeholders rejected: %v", err)
	}

	singbox := `{"outbounds":[${proxies},{"type":"direct","tag":"direct"}],"route":{"final":"select","tags":[${proxy_names}]}}`
	if err := validate(model.SettingSingBoxTemplateJSON, singbox); err != nil {
		t.Fatalf("singbox template with placeholders rejected: %v", err)
	}

	if err := validate(model.SettingSingBoxTemplateJSON, `{"outbounds":[]}`); err == nil {
		t.Fatal("singbox template without ${proxies} should be rejected")
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
