package repository

import (
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/service/wgcrypto"
)

func testCipher(t *testing.T, hexByte string) Cipher {
	t.Helper()
	c, err := wgcrypto.NewCipherFromHexKey(strings.Repeat(hexByte, 32)) // 32 bytes = 64 hex chars
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return c
}

func TestSealOpenSecret_RoundTrip(t *testing.T) {
	c := testCipher(t, "ab")
	stored, err := sealSecret(c, "smtp-password-123")
	if err != nil {
		t.Fatalf("sealSecret: %v", err)
	}
	if !strings.HasPrefix(stored, secretPrefix) {
		t.Errorf("stored value missing %q prefix: %q", secretPrefix, stored)
	}
	if strings.Contains(stored, "smtp-password-123") {
		t.Errorf("plaintext leaked into the stored value: %q", stored)
	}
	got, err := openSecret(c, stored)
	if err != nil {
		t.Fatalf("openSecret: %v", err)
	}
	if got != "smtp-password-123" {
		t.Errorf("round-trip = %q, want smtp-password-123", got)
	}
}

func TestOpenSecret_PlaintextPassthrough(t *testing.T) {
	c := testCipher(t, "ab")
	got, err := openSecret(c, "not-encrypted-value")
	if err != nil {
		t.Fatalf("openSecret plaintext: %v", err)
	}
	if got != "not-encrypted-value" {
		t.Errorf("a value without the prefix should pass through, got %q", got)
	}
}

func TestOpenSecret_WrongKeyFails(t *testing.T) {
	stored, err := sealSecret(testCipher(t, "ab"), "secret")
	if err != nil {
		t.Fatalf("sealSecret: %v", err)
	}
	if _, err := openSecret(testCipher(t, "cd"), stored); err == nil {
		t.Fatal("decrypting with the wrong key should fail (GCM auth)")
	}
}
