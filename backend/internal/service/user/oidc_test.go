package user

import (
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestOIDCSessions_PutTake(t *testing.T) {
	s := newOIDCSessions()
	s.put("state-a", &oidcState{
		verifier:  "v-a",
		expiresAt: time.Now().Add(5 * time.Minute),
	})

	got := s.take("state-a")
	if got == nil || got.verifier != "v-a" {
		t.Fatalf("take returned %+v, want verifier=v-a", got)
	}
	// Second take of the same state must miss (replay defense).
	if again := s.take("state-a"); again != nil {
		t.Errorf("replay take returned %+v, want nil", again)
	}
}

func TestOIDCSessions_TakeExpired(t *testing.T) {
	s := newOIDCSessions()
	s.put("expired", &oidcState{
		verifier:  "v",
		expiresAt: time.Now().Add(-1 * time.Second),
	})
	if got := s.take("expired"); got != nil {
		t.Errorf("expected nil for expired entry, got %+v", got)
	}
}

func TestRandomURLString_Format(t *testing.T) {
	s, err := randomURLString(32)
	if err != nil {
		t.Fatalf("randomURLString: %v", err)
	}
	// Base64URL-encoded 32 bytes is 43 chars (no padding).
	if len(s) != 43 {
		t.Errorf("len=%d, want 43 (base64url of 32 bytes)", len(s))
	}
	// Sanity: alphabet is URL-safe (no + / =).
	for _, r := range s {
		if r == '+' || r == '/' || r == '=' {
			t.Errorf("unexpected char %q in %q (should be URL-safe)", r, s)
		}
	}
}

func TestJWKToRSAPublicKey_RoundtripDecode(t *testing.T) {
	// Synthetic JWK using a known small RSA key (NOT for crypto use —
	// just to verify n/e decode path).
	//
	// e=65537 → "AQAB" base64url
	// n is a 256-bit number "FACE…" → base64url-encoded
	nRaw := make([]byte, 256)
	nRaw[0] = 0xFA
	nRaw[1] = 0xCE
	nB64 := base64.RawURLEncoding.EncodeToString(nRaw)

	key := jwksKey{
		Kid: "test-1",
		Kty: "RSA",
		Alg: "RS256",
		N:   nB64,
		E:   "AQAB",
	}
	pub, err := jwkToRSAPublicKey(key)
	if err != nil {
		t.Fatalf("jwkToRSAPublicKey: %v", err)
	}
	if pub.E != 65537 {
		t.Errorf("e = %d, want 65537", pub.E)
	}
	if pub.N == nil || pub.N.Sign() <= 0 {
		t.Errorf("n is zero/negative: %v", pub.N)
	}
	// Verify decoded n bytes start with FA CE
	roundtrip := new(big.Int).SetBytes(nRaw)
	if pub.N.Cmp(roundtrip) != 0 {
		t.Errorf("n roundtrip mismatch")
	}
}

func TestJWKToRSAPublicKey_RejectsBadN(t *testing.T) {
	if _, err := jwkToRSAPublicKey(jwksKey{Kty: "RSA", N: "not!base64", E: "AQAB"}); err == nil {
		t.Error("expected error on malformed N")
	}
}

func TestJWKToRSAPublicKey_RejectsZeroE(t *testing.T) {
	if _, err := jwkToRSAPublicKey(jwksKey{Kty: "RSA", N: "AAAA", E: "AAAA"}); err == nil {
		t.Error("expected error on zero E")
	}
}

func TestSnippet(t *testing.T) {
	short := []byte("short")
	if got := snippet(short); got != "short" {
		t.Errorf("snippet(short) = %q, want short", got)
	}
	long := []byte(strings.Repeat("x", 500))
	got := snippet(long)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("snippet(long) should end with …, got %q", got[len(got)-3:])
	}
	if len(got) > 250 {
		t.Errorf("snippet returned %d bytes, expected ≤ ~200+ellipsis", len(got))
	}
}
