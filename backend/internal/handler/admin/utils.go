package admin

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/curve25519"
)

// UtilsHandler exposes admin-only crypto helpers (currently just
// X25519 keypair generation for Reality inbound configuration).
//
// Reality's privateKey/publicKey pair is whatever the underlying
// xray-core's `xray x25519` command would emit: 32 random bytes for
// the private scalar, the curve25519 base-point multiplied by it for
// the public, both encoded as base64 url-safe without padding (the
// format clients/servers expect in `realitySettings`).
type UtilsHandler struct{}

// NewUtilsHandler constructs the handler. It has no dependencies.
func NewUtilsHandler() *UtilsHandler { return &UtilsHandler{} }

// RegisterRoutes mounts /utils under rg.
func (h *UtilsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/utils")
	g.POST("/x25519", h.X25519)
}

type x25519Response struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

// X25519 returns a fresh X25519 keypair encoded as base64 url-safe
// without padding. Each call yields a fresh keypair; the body is
// ignored.
func (h *UtilsHandler) X25519(c *gin.Context) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "random source unavailable"})
		return
	}
	// Curve25519 expects the scalar to be clamped before use.
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key derivation failed"})
		return
	}

	enc := base64.RawURLEncoding
	c.JSON(http.StatusOK, x25519Response{
		PrivateKey: enc.EncodeToString(priv[:]),
		PublicKey:  enc.EncodeToString(pub),
	})
}
