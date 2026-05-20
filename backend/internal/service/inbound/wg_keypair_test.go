package inbound

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

func TestEnsureWGSecretKey_FillsEmpty(t *testing.T) {
	in := &runtime.Inbound{
		Protocol: "wireguard",
		Settings: `{"mtu":0,"secretKey":"","peers":[],"noKernelTun":false}`,
	}
	if err := ensureWGSecretKey(in); err != nil {
		t.Fatalf("ensureWGSecretKey: %v", err)
	}
	var s runtime.WGSettings
	if err := json.Unmarshal([]byte(in.Settings), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.SecretKey == "" {
		t.Error("SecretKey still empty after fill")
	}
	if len(s.SecretKey) != 44 {
		t.Errorf("SecretKey length = %d, want 44 (base64 of 32 bytes)", len(s.SecretKey))
	}
	if s.MTU != 1420 {
		t.Errorf("MTU = %d, want 1420 default", s.MTU)
	}
}

func TestEnsureWGSecretKey_PreservesExisting(t *testing.T) {
	in := &runtime.Inbound{
		Protocol: "wireguard",
		Settings: `{"mtu":1280,"secretKey":"existingkey44chars-base64-AAAAAAAAAAAAAA=","peers":[],"noKernelTun":false}`,
	}
	if err := ensureWGSecretKey(in); err != nil {
		t.Fatalf("ensureWGSecretKey: %v", err)
	}
	var s runtime.WGSettings
	_ = json.Unmarshal([]byte(in.Settings), &s)
	if !strings.HasPrefix(s.SecretKey, "existingkey") {
		t.Errorf("existing SecretKey overwritten: %q", s.SecretKey)
	}
	if s.MTU != 1280 {
		t.Errorf("existing MTU overwritten: %d", s.MTU)
	}
}

func TestEnsureWGSecretKey_EmptySettingsString(t *testing.T) {
	in := &runtime.Inbound{
		Protocol: "wireguard",
		Settings: "",
	}
	if err := ensureWGSecretKey(in); err != nil {
		t.Fatalf("ensureWGSecretKey: %v", err)
	}
	if in.Settings == "" {
		t.Fatal("Settings still empty after fill")
	}
	var s runtime.WGSettings
	if err := json.Unmarshal([]byte(in.Settings), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.SecretKey == "" || s.MTU != 1420 {
		t.Errorf("got %+v", s)
	}
}

func TestEnsureWGSecretKey_MalformedJSONErrors(t *testing.T) {
	in := &runtime.Inbound{
		Protocol: "wireguard",
		Settings: "{not valid json",
	}
	if err := ensureWGSecretKey(in); err == nil {
		t.Fatal("expected error on malformed Settings, got nil")
	}
}
