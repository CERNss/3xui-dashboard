package wgcrypto

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// devKey is a test-only hex master key. Real deployments draw one
// via `openssl rand -hex 32` and store it in WG_MASTER_KEY.
const devKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestGenerateKeypair_Format(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if got := len(kp.Private); got != 44 {
		t.Errorf("private key length = %d, want 44 (base64 of 32 bytes)", got)
	}
	if got := len(kp.Public); got != 44 {
		t.Errorf("public key length = %d, want 44", got)
	}
	if kp.Private == kp.Public {
		t.Error("private and public keys are identical — generation is broken")
	}
	raw, err := base64.StdEncoding.DecodeString(kp.Private)
	if err != nil {
		t.Fatalf("private not valid base64: %v", err)
	}
	// RFC 7748 §5 clamp checks: bit 0..2 cleared, bit 254 set, bit 255 cleared.
	if raw[0]&0b00000111 != 0 {
		t.Errorf("private byte 0 low 3 bits not cleared: %08b", raw[0])
	}
	if raw[31]&0b10000000 != 0 {
		t.Errorf("private byte 31 high bit not cleared: %08b", raw[31])
	}
	if raw[31]&0b01000000 == 0 {
		t.Errorf("private byte 31 bit 6 not set: %08b", raw[31])
	}
}

func TestGenerateKeypair_Unique(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 10; i++ {
		kp, err := GenerateKeypair()
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if _, dup := seen[kp.Private]; dup {
			t.Fatalf("iter %d: duplicate private key — rand.Read is suspect", i)
		}
		seen[kp.Private] = struct{}{}
	}
}

func TestDerivePublic_MatchesGenerate(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	derived, err := DerivePublic(kp.Private)
	if err != nil {
		t.Fatalf("DerivePublic: %v", err)
	}
	if derived != kp.Public {
		t.Errorf("DerivePublic = %q, want %q", derived, kp.Public)
	}
}

func TestDerivePublic_RejectsBadInput(t *testing.T) {
	if _, err := DerivePublic("not base64!!"); err == nil {
		t.Error("expected error on non-base64 input")
	}
	if _, err := DerivePublic(base64.StdEncoding.EncodeToString([]byte("too short"))); err == nil {
		t.Error("expected error on wrong-length key")
	}
}

func TestCipher_Roundtrip(t *testing.T) {
	c, err := NewCipherFromHexKey(devKey)
	if err != nil {
		t.Fatalf("NewCipherFromHexKey: %v", err)
	}
	plain := []byte("wg-private-key-bytes")
	sealed, err := c.Seal(plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(sealed, plain) {
		t.Error("sealed blob contains plaintext — encryption is broken")
	}
	got, err := c.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("roundtrip = %q, want %q", got, plain)
	}
}

func TestCipher_TamperedRejected(t *testing.T) {
	c, _ := NewCipherFromHexKey(devKey)
	sealed, _ := c.Seal([]byte("payload"))
	// Flip a tag byte.
	sealed[len(sealed)-1] ^= 0xff
	if _, err := c.Open(sealed); err == nil {
		t.Fatal("expected Open to reject tampered ciphertext")
	}
}

func TestCipher_NonceVariesAcrossSeals(t *testing.T) {
	c, _ := NewCipherFromHexKey(devKey)
	a, _ := c.Seal([]byte("same"))
	b, _ := c.Seal([]byte("same"))
	if bytes.Equal(a, b) {
		t.Error("Seal produced identical ciphertext for same plaintext — nonce reuse")
	}
}

func TestNewCipherFromHexKey_Validation(t *testing.T) {
	cases := []struct {
		name string
		hex  string
		err  string
	}{
		{"empty", "", "is empty"},
		{"bad hex", "zz", "decode"},
		{"wrong length", "00", "length = 1, want 32"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCipherFromHexKey(tc.hex)
			if err == nil {
				t.Fatalf("expected error for %q", tc.hex)
			}
			if !strings.Contains(err.Error(), tc.err) {
				t.Errorf("error %q does not mention %q", err, tc.err)
			}
		})
	}
}
