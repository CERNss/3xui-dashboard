package stripe

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVerifyWebhook_Roundtrip(t *testing.T) {
	body := []byte(`{"id":"evt_test_123","type":"checkout.session.completed"}`)
	secret := "whsec_supersecret"
	now := time.Now()
	sig := SignForTest(body, secret, now)
	if err := VerifyWebhook(body, sig, secret, now); err != nil {
		t.Errorf("roundtrip failed: %v", err)
	}
}

func TestVerifyWebhook_TamperedBody(t *testing.T) {
	body := []byte(`{"id":"evt_test_123"}`)
	secret := "whsec_x"
	now := time.Now()
	sig := SignForTest(body, secret, now)
	tampered := []byte(`{"id":"evt_test_124"}`) // one char different
	err := VerifyWebhook(tampered, sig, secret, now)
	if err == nil {
		t.Fatal("verify accepted tampered body")
	}
	if !errors.Is(err, ErrSignatureFailed) {
		t.Errorf("want ErrSignatureFailed, got %v", err)
	}
}

func TestVerifyWebhook_TamperedSignature(t *testing.T) {
	body := []byte(`{"id":"evt_test_123"}`)
	secret := "whsec_x"
	now := time.Now()
	sig := SignForTest(body, secret, now)
	// Flip last char of hex sig
	idx := len(sig) - 1
	tampered := []byte(sig)
	if tampered[idx] == 'a' {
		tampered[idx] = 'b'
	} else {
		tampered[idx] = 'a'
	}
	if err := VerifyWebhook(body, string(tampered), secret, now); err == nil {
		t.Fatal("verify accepted tampered sig")
	}
}

func TestVerifyWebhook_ReplayRejected(t *testing.T) {
	body := []byte(`{"id":"evt_test_123"}`)
	secret := "whsec_x"
	// Sign with a timestamp 10 minutes old
	oldTime := time.Now().Add(-10 * time.Minute)
	sig := SignForTest(body, secret, oldTime)
	err := VerifyWebhook(body, sig, secret, time.Now())
	if err == nil {
		t.Fatal("verify accepted replay")
	}
	if !errors.Is(err, ErrReplay) {
		t.Errorf("want ErrReplay, got %v", err)
	}
}

func TestVerifyWebhook_MissingT(t *testing.T) {
	body := []byte(`{}`)
	err := VerifyWebhook(body, "v1=abc", "whsec_x", time.Now())
	if err == nil || !errors.Is(err, ErrSignatureFailed) {
		t.Errorf("want ErrSignatureFailed on missing t, got %v", err)
	}
}

func TestVerifyWebhook_MissingV1(t *testing.T) {
	body := []byte(`{}`)
	err := VerifyWebhook(body, "t=1700000000", "whsec_x", time.Now())
	if err == nil || !errors.Is(err, ErrSignatureFailed) {
		t.Errorf("want ErrSignatureFailed on missing v1, got %v", err)
	}
}

func TestVerifyWebhook_EmptySecret(t *testing.T) {
	body := []byte(`{}`)
	now := time.Now()
	sig := "t=" + strconv.FormatInt(now.Unix(), 10) + ",v1=deadbeef"
	err := VerifyWebhook(body, sig, "", now)
	if err == nil || !errors.Is(err, ErrSignatureFailed) {
		t.Errorf("want ErrSignatureFailed on empty secret, got %v", err)
	}
}

func TestVerifyWebhook_MultipleV1Entries(t *testing.T) {
	// During secret rotation Stripe sends BOTH old + new signatures
	// in the same Stripe-Signature header. We must accept any match.
	body := []byte(`{"x":1}`)
	secret := "whsec_real"
	now := time.Now()
	realSig := SignForTest(body, secret, now)
	// realSig is "t=...,v1=..." — inject a fake v1 in front
	parts := strings.SplitN(realSig, ",v1=", 2)
	composed := parts[0] + ",v1=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef,v1=" + parts[1]
	if err := VerifyWebhook(body, composed, secret, now); err != nil {
		t.Errorf("multi-v1 acceptance failed: %v", err)
	}
}

func TestSignForTest_Deterministic(t *testing.T) {
	body := []byte("hello")
	secret := "whsec_x"
	now := time.Unix(1700000000, 0)
	a := SignForTest(body, secret, now)
	b := SignForTest(body, secret, now)
	if a != b {
		t.Errorf("SignForTest not deterministic: %q vs %q", a, b)
	}
}
