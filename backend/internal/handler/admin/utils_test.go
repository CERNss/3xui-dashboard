package admin

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/curve25519"
)

func TestX25519GeneratesValidKeypair(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewUtilsHandler().RegisterRoutes(r.Group("/api/admin"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/utils/x25519", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		PrivateKey string `json:"privateKey"`
		PublicKey  string `json:"publicKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	priv, err := base64.RawURLEncoding.DecodeString(body.PrivateKey)
	if err != nil || len(priv) != 32 {
		t.Fatalf("private key not 32 raw-url-base64 bytes: err=%v len=%d", err, len(priv))
	}
	pub, err := base64.RawURLEncoding.DecodeString(body.PublicKey)
	if err != nil || len(pub) != 32 {
		t.Fatalf("public key not 32 raw-url-base64 bytes: err=%v len=%d", err, len(pub))
	}

	// Re-derive the public key from the returned private key and
	// confirm it matches what the handler emitted.
	derived, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		t.Fatalf("derive public from returned private: %v", err)
	}
	if base64.RawURLEncoding.EncodeToString(derived) != body.PublicKey {
		t.Fatal("returned public key does not match base-point * private")
	}
}

func TestX25519YieldsFreshKeypairs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewUtilsHandler().RegisterRoutes(r.Group("/api/admin"))

	call := func() (string, string) {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/utils/x25519", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		var body struct {
			PrivateKey string `json:"privateKey"`
			PublicKey  string `json:"publicKey"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		return body.PrivateKey, body.PublicKey
	}

	priv1, pub1 := call()
	priv2, pub2 := call()
	if priv1 == priv2 || pub1 == pub2 {
		t.Fatal("two consecutive calls produced identical keypair — random source not used")
	}
}
