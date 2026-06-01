package admin

import (
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

// Secret settings must be flagged so List masks them and Put encrypts
// them (SettingRepo.SetSecret). Forgetting the flag silently leaks the
// value into the List response and stores it in plaintext.
func TestSecretSettings_AreFlaggedSecret(t *testing.T) {
	for _, key := range []string{
		model.SettingSMTPPassword,
		model.SettingOIDCClientSecret,
	} {
		d, ok := descriptorFor(key)
		if !ok {
			t.Errorf("%s is not registered in knownSettings", key)
			continue
		}
		if !d.Secret {
			t.Errorf("%s must be marked Secret (masked in List, encrypted on Put)", key)
		}
	}
}

func TestMaskSet(t *testing.T) {
	if got := maskSet(""); got != "" {
		t.Errorf("maskSet(empty) = %q, want empty", got)
	}
	if got := maskSet("super-secret"); got == "super-secret" {
		t.Error("maskSet must not return the raw secret")
	}
	if got := maskSet("super-secret"); got != "********" {
		t.Errorf("maskSet(non-empty) = %q, want the fixed marker", got)
	}
}
