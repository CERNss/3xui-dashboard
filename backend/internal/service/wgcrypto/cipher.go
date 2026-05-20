package wgcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// Cipher wraps an AES-256-GCM AEAD with a fixed master key. The
// nonce is generated per-Seal and stored as a prefix on the
// returned ciphertext, so the on-disk bytea is self-contained
// (nonce || ciphertext || tag).
type Cipher struct {
	aead cipher.AEAD
}

// NewCipherFromHexKey decodes a 64-char hex string into a 32-byte
// AES-256 key and returns a Cipher. The expected input format
// matches `openssl rand -hex 32`.
func NewCipherFromHexKey(hexKey string) (*Cipher, error) {
	if hexKey == "" {
		return nil, errors.New("wgcrypto: WG_MASTER_KEY is empty — generate one via 'openssl rand -hex 32'")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("wgcrypto: decode WG_MASTER_KEY hex: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("wgcrypto: WG_MASTER_KEY decoded length = %d, want 32 (64 hex chars)", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("wgcrypto: aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("wgcrypto: cipher.NewGCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext with a freshly-generated nonce and
// returns nonce||ciphertext||tag suitable for storage.
func (c *Cipher) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("wgcrypto: read nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a nonce||ciphertext||tag blob produced by Seal.
// Returns an error on tag mismatch (tampered ciphertext) or
// truncated input.
func (c *Cipher) Open(sealed []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(sealed) < ns {
		return nil, fmt.Errorf("wgcrypto: sealed blob length %d < nonce size %d", len(sealed), ns)
	}
	nonce, ciphertext := sealed[:ns], sealed[ns:]
	pt, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("wgcrypto: gcm.Open (tampered or wrong key): %w", err)
	}
	return pt, nil
}
