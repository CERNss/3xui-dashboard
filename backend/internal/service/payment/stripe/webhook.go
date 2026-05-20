package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrSignatureFailed wraps any HMAC verification failure. Callers
// should never branch on this to skip verification.
var ErrSignatureFailed = errors.New("stripe: webhook signature failed")

// ErrReplay is returned when the `t=` timestamp in Stripe-Signature
// is more than maxAge old. Distinct from ErrSignatureFailed so the
// log line tells operators what actually happened.
var ErrReplay = errors.New("stripe: webhook timestamp too old (possible replay)")

// maxAge for webhook timestamps. 5 minutes matches Stripe's
// published recommendation.
const maxAge = 5 * time.Minute

// VerifyWebhook checks the Stripe-Signature header against the raw
// body. The body MUST be exactly what Stripe POSTed — Gin's JSON
// binding mutates whitespace, so the handler reads with io.ReadAll
// FIRST and passes the raw bytes here.
//
// Stripe-Signature shape:  t=1492774577,v1=5257a869e7e..,v0=...
//
// Verification:
//   1. Parse t and (one or more) v1 values.
//   2. Reject if t is older than maxAge.
//   3. Compute HMAC-SHA256("${t}.${rawBody}", secret).
//   4. Constant-time compare against each v1 — Stripe rotates keys
//      via a multi-secret window so multiple v1 entries are valid
//      during rotation.
func VerifyWebhook(rawBody []byte, sigHeader, secret string, now time.Time) error {
	if secret == "" {
		return fmt.Errorf("%w: webhook secret not configured", ErrSignatureFailed)
	}
	t, v1s, err := parseSignature(sigHeader)
	if err != nil {
		return err
	}
	if now.Sub(t) > maxAge {
		return ErrReplay
	}
	signed := strconv.FormatInt(t.Unix(), 10) + "." + string(rawBody)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signed))
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range v1s {
		if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) == 1 {
			return nil
		}
	}
	return fmt.Errorf("%w: no v1 matched", ErrSignatureFailed)
}

// parseSignature splits "t=...,v1=...,v0=..." into the timestamp +
// every v1 value. Multiple v1 entries are legal during webhook
// secret rotation; we accept any match.
func parseSignature(header string) (time.Time, []string, error) {
	if header == "" {
		return time.Time{}, nil, fmt.Errorf("%w: empty header", ErrSignatureFailed)
	}
	var tStr string
	var v1s []string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			tStr = kv[1]
		case "v1":
			v1s = append(v1s, kv[1])
		}
	}
	if tStr == "" {
		return time.Time{}, nil, fmt.Errorf("%w: missing t", ErrSignatureFailed)
	}
	if len(v1s) == 0 {
		return time.Time{}, nil, fmt.Errorf("%w: missing v1", ErrSignatureFailed)
	}
	ts, err := strconv.ParseInt(tStr, 10, 64)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("%w: bad t: %v", ErrSignatureFailed, err)
	}
	return time.Unix(ts, 0), v1s, nil
}

// SignForTest is a test helper exported for the handler test
// package — it produces the same Stripe-Signature header the real
// Stripe servers would send. Not used in production code.
func SignForTest(rawBody []byte, secret string, t time.Time) string {
	signed := strconv.FormatInt(t.Unix(), 10) + "." + string(rawBody)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signed))
	return fmt.Sprintf("t=%d,v1=%s", t.Unix(), hex.EncodeToString(mac.Sum(nil)))
}
