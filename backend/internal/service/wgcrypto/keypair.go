// Package wgcrypto provides the keypair generation and at-rest
// encryption primitives the WireGuard provisioning flow needs.
//
// Two responsibilities, intentionally tiny:
//
//   - keypair.go: curve25519 keypair generation matching the
//     wg(8) / wireguard-tools wire format (base64-encoded
//     32-byte big-endian) so the produced strings are pasteable
//     into [Peer]/[Interface] config blocks unchanged.
//   - cipher.go: AES-256-GCM authenticated encryption of peer
//     private keys before they hit the wg_peers.private_key_encrypted
//     bytea column. The master key comes from WG_MASTER_KEY env;
//     rotation is out-of-scope for v1 and the docs spell that out.
package wgcrypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// Keypair holds a curve25519 keypair in the wg(8) text form:
// 32 raw bytes base64-encoded with standard alphabet + padding.
// Both Private and Public are 44 characters when encoded.
type Keypair struct {
	Private string
	Public  string
}

// GenerateKeypair produces a new curve25519 keypair. The private
// key is drawn from crypto/rand and clamped per RFC 7748 §5; the
// public key is X25519(priv, base point). Both are returned in
// the standard wg(8) base64 encoding.
func GenerateKeypair() (Keypair, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return Keypair{}, fmt.Errorf("wgcrypto: read random: %w", err)
	}
	// RFC 7748 §5: clamp the private scalar.
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return Keypair{}, fmt.Errorf("wgcrypto: derive public: %w", err)
	}
	return Keypair{
		Private: base64.StdEncoding.EncodeToString(priv[:]),
		Public:  base64.StdEncoding.EncodeToString(pub),
	}, nil
}

// DerivePublic returns the base64-encoded public key for the given
// base64-encoded private key. Used in tests + reconciliation paths
// where only the private key is known.
func DerivePublic(privateB64 string) (string, error) {
	priv, err := base64.StdEncoding.DecodeString(privateB64)
	if err != nil {
		return "", fmt.Errorf("wgcrypto: decode private key: %w", err)
	}
	if len(priv) != 32 {
		return "", fmt.Errorf("wgcrypto: private key length = %d, want 32", len(priv))
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("wgcrypto: derive public: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}
