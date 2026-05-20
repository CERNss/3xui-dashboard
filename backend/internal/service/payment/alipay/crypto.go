// Package alipay implements Alipay's 当面付 (face-to-face / QR-based)
// payment integration as a self-contained gateway.
//
// Crypto here is pure stdlib — no third-party SDK — so we can audit
// every byte against alipay's published canonical-string algorithm
// without chasing transitive deps. The canonical algorithm:
//
//  1. Drop empty values + the keys "sign" and "sign_type"
//  2. Sort remaining keys alphabetically
//  3. Join as `k1=v1&k2=v2&...` (no URL-encoding — alipay specifies
//     plain `&` joining for the signing payload)
//  4. RSA2 (SHA256withRSA) sign / verify with base64 encoding
//
// References: https://opendocs.alipay.com/open/204/105301
package alipay

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrSignatureFailed wraps any RSA verification failure. Callers
// should NOT branch on this error to skip verification — it's
// exported only so logs/responses can distinguish "bad signature"
// from "malformed PEM" or "missing field".
var ErrSignatureFailed = errors.New("alipay: signature verification failed")

// ErrInvalidKey wraps PEM/parse failures on a key passed in from
// configuration. Sign / verify both surface this so an operator
// pasting a corrupt key sees a clear error at boot or first request.
var ErrInvalidKey = errors.New("alipay: invalid PEM key")

// canonicalString returns the alipay-spec signing string: keys
// sorted, empty values dropped, "sign" + "sign_type" dropped, joined
// k=v with `&`. Used by both Sign and Verify.
func canonicalString(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v == "" || k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	return b.String()
}

// parsePrivateKey accepts either PKCS#1 (RSA PRIVATE KEY) or PKCS#8
// (PRIVATE KEY) PEM blocks. Alipay's open-platform UI hands operators
// PKCS#8 by default, but tooling-generated keys are often PKCS#1 —
// we accept both so the operator doesn't have to convert.
func parsePrivateKey(pemBlock string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemBlock))
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block", ErrInvalidKey)
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: PKCS#8 key is not RSA", ErrInvalidKey)
	}
	return key, nil
}

// parsePublicKey accepts PKIX (PUBLIC KEY) — alipay's open-platform
// downloads PEMs in this format. PKCS#1 (RSA PUBLIC KEY) is also
// accepted as a courtesy for the rare operator who used openssl
// directly.
func parsePublicKey(pemBlock string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemBlock))
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block", ErrInvalidKey)
	}
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		key, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("%w: PKIX key is not RSA", ErrInvalidKey)
		}
		return key, nil
	}
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}
	return key, nil
}

// SignRSA2 signs the canonical-string form of params with the given
// PEM private key. Returns the base64-encoded signature ready to be
// attached as the `sign` parameter on the outbound request.
func SignRSA2(params map[string]string, pemPrivateKey string) (string, error) {
	key, err := parsePrivateKey(pemPrivateKey)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(canonicalString(params)))
	sig, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("alipay: rsa sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyRSA2 verifies the base64 `signature` against the canonical
// form of params using the PEM public key (alipay's platform key).
// Returns nil on success, wraps ErrSignatureFailed on bad signature.
func VerifyRSA2(params map[string]string, signature, pemPublicKey string) error {
	key, err := parsePublicKey(pemPublicKey)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("%w: base64 decode: %v", ErrSignatureFailed, err)
	}
	digest := sha256.Sum256([]byte(canonicalString(params)))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return fmt.Errorf("%w: %v", ErrSignatureFailed, err)
	}
	return nil
}

// base64Decode is a thin wrapper kept as a package-internal helper
// so client.go reads the same way without re-importing the base64
// package. Production callers should use VerifyRSA2 (which expects
// param-canonical strings) — base64Decode is only used by the
// envelope verifier in client.go which signs over a raw JSON blob.
func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// rsaVerifySHA256 verifies a SHA256-with-RSA signature directly over
// `data`, no canonical-string transformation. Used for verifying the
// outer envelope on alipay responses where the signature covers the
// raw inner-JSON bytes verbatim.
func rsaVerifySHA256(key *rsa.PublicKey, data, signature []byte) error {
	digest := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature)
}
