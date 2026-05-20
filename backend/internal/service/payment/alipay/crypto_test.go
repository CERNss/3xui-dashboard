package alipay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
)

// genTestKeypair returns a fresh PEM keypair for one test. RSA-2048
// per the alipay docs minimum; small enough to generate fast.
func genTestKeypair(t *testing.T) (privPEM, pubPEM string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen rsa: %v", err)
	}
	privDer := x509.MarshalPKCS1PrivateKey(priv)
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDer}))

	pubDer, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pubkey: %v", err)
	}
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer}))
	return
}

func TestCanonicalString_SortsAndDropsEmpty(t *testing.T) {
	params := map[string]string{
		"app_id":    "2021000000",
		"sign":      "should-be-dropped",
		"sign_type": "RSA2",
		"empty":     "",
		"method":    "alipay.trade.precreate",
		"charset":   "utf-8",
	}
	got := canonicalString(params)
	want := "app_id=2021000000&charset=utf-8&method=alipay.trade.precreate"
	if got != want {
		t.Errorf("canonicalString = %q\nwant                  %q", got, want)
	}
}

func TestCanonicalString_EmptyInput(t *testing.T) {
	if got := canonicalString(nil); got != "" {
		t.Errorf("nil input: got %q, want empty", got)
	}
	if got := canonicalString(map[string]string{}); got != "" {
		t.Errorf("empty map: got %q, want empty", got)
	}
}

func TestSignVerify_Roundtrip(t *testing.T) {
	priv, pub := genTestKeypair(t)
	params := map[string]string{
		"app_id":    "2021000000",
		"method":    "alipay.trade.precreate",
		"charset":   "utf-8",
		"timestamp": "2026-05-20 14:00:00",
		"version":   "1.0",
	}
	sig, err := SignRSA2(params, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if sig == "" {
		t.Fatal("empty signature")
	}
	if err := VerifyRSA2(params, sig, pub); err != nil {
		t.Errorf("verify roundtrip failed: %v", err)
	}
}

func TestVerify_RejectsTamperedPayload(t *testing.T) {
	priv, pub := genTestKeypair(t)
	params := map[string]string{
		"app_id": "2021000000",
		"method": "alipay.trade.precreate",
	}
	sig, err := SignRSA2(params, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	// Mutate one byte of the payload — verify should fail.
	params["app_id"] = "2021000001"
	if err := VerifyRSA2(params, sig, pub); err == nil {
		t.Fatal("verify accepted tampered payload")
	} else if !errors.Is(err, ErrSignatureFailed) {
		t.Errorf("expected ErrSignatureFailed, got %v", err)
	}
}

func TestVerify_RejectsTamperedSignature(t *testing.T) {
	priv, pub := genTestKeypair(t)
	params := map[string]string{"app_id": "2021000000"}
	sig, err := SignRSA2(params, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	// Flip one byte in the signature.
	tampered := []byte(sig)
	if tampered[0] == 'A' {
		tampered[0] = 'B'
	} else {
		tampered[0] = 'A'
	}
	if err := VerifyRSA2(params, string(tampered), pub); err == nil {
		t.Fatal("verify accepted tampered signature")
	}
}

func TestSign_InvalidPEM(t *testing.T) {
	_, err := SignRSA2(map[string]string{"x": "y"}, "not-a-pem")
	if err == nil {
		t.Fatal("expected error on invalid PEM")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestVerify_InvalidPEM(t *testing.T) {
	err := VerifyRSA2(map[string]string{"x": "y"}, "sig", "not-a-pem")
	if err == nil {
		t.Fatal("expected error on invalid PEM")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestVerify_InvalidBase64Signature(t *testing.T) {
	_, pub := genTestKeypair(t)
	err := VerifyRSA2(map[string]string{"x": "y"}, "not-base64!!!", pub)
	if err == nil {
		t.Fatal("expected error on bad base64")
	}
	if !errors.Is(err, ErrSignatureFailed) {
		t.Errorf("expected ErrSignatureFailed, got %v", err)
	}
}

func TestParsePrivateKey_AcceptsPKCS8(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("pkcs8 marshal: %v", err)
	}
	pemBlock := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	parsed, err := parsePrivateKey(pemBlock)
	if err != nil {
		t.Fatalf("pkcs8 parse: %v", err)
	}
	if parsed.N.Cmp(priv.N) != 0 {
		t.Error("PKCS#8 parse returned wrong modulus")
	}
}
