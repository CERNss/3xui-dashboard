package user

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/middleware"
	"github.com/cern/3xui-dashboard/internal/service/billing"
	"github.com/cern/3xui-dashboard/internal/service/payment"
)

// BillingHandler serves /api/user/plans, /api/user/orders, /api/user/purchase.
type BillingHandler struct{ svc *billing.Service }

// NewBillingHandler wires the handler.
func NewBillingHandler(svc *billing.Service) *BillingHandler { return &BillingHandler{svc: svc} }

// RegisterRoutes mounts the endpoints under rg (RequireUser).
func (h *BillingHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/plans", h.ListPlans)
	rg.GET("/orders", h.ListOrders)
	rg.GET("/orders/:id", h.GetOrder)
	rg.GET("/payment-methods", h.PaymentMethods)
	rg.POST("/purchase", h.Purchase)
	rg.POST("/purchase/:provider", h.PurchaseViaPayment)
}

// PaymentMethods returns the providers the user can pick from on the
// portal. Always includes "balance"; additionally lists any payment
// gateway registered at boot.
func (h *BillingHandler) PaymentMethods(c *gin.Context) {
	reg := h.svc.Gateways()
	if reg == nil {
		c.JSON(http.StatusOK, gin.H{"methods": []string{"balance"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"methods": reg.EnabledProviders()})
}

// GetOrder returns one order by ID, scoped to the authenticated
// user — portal pollers use this to flip the QR modal to "支付成功"
// when status advances to completed.
func (h *BillingHandler) GetOrder(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad order id"})
		return
	}
	order, err := h.svc.GetOrderForUser(c.Request.Context(), userID, id)
	if err != nil {
		if errors.Is(err, billing.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

// PurchaseViaPayment serves POST /api/user/billing/purchase/:provider.
// Provider currently supported: "alipay" (others register the same
// way as we add Stripe / WeChat Pay).
func (h *BillingHandler) PurchaseViaPayment(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	provider := c.Param("provider")
	var req purchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = randHex(16)
	}
	order, err := h.svc.PurchaseViaPayment(c.Request.Context(), billing.PurchaseViaPaymentInput{
		UserID:         userID,
		PlanID:         req.PlanID,
		IdempotencyKey: req.IdempotencyKey,
		NodeID:         req.NodeID,
		InboundTag:     req.InboundTag,
		Provider:       provider,
	})
	if err != nil {
		switch {
		case errors.Is(err, payment.ErrUnknownProvider):
			c.JSON(http.StatusNotFound, gin.H{"error": "payment provider not configured"})
		case errors.Is(err, billing.ErrPlanNotFound), errors.Is(err, billing.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, billing.ErrPlanDisabled):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, billing.ErrNoProvisioningTarget):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, billing.ErrIdempotencyConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, billing.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "order": order})
		}
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *BillingHandler) ListPlans(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	rows, err := h.svc.ListPlansForUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": rows})
}

func (h *BillingHandler) ListOrders(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, err := h.svc.ListOrdersByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": rows, "limit": limit, "offset": offset})
}

type purchaseRequest struct {
	PlanID         int64  `json:"plan_id" binding:"required"`
	IdempotencyKey string `json:"idempotency_key"`
	NodeID         int64  `json:"node_id"`
	InboundTag     string `json:"inbound_tag"`
}

func (h *BillingHandler) Purchase(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	var req purchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if req.IdempotencyKey == "" {
		// Generate one server-side so retries from a flaky client
		// don't accidentally double-charge. The client can pass its
		// own to make retries safe across reconnects.
		req.IdempotencyKey = randHex(16)
	}
	order, err := h.svc.Purchase(c.Request.Context(), billing.PurchaseInput{
		UserID:         userID,
		PlanID:         req.PlanID,
		IdempotencyKey: req.IdempotencyKey,
		NodeID:         req.NodeID,
		InboundTag:     req.InboundTag,
	})
	if err != nil {
		switch {
		case errors.Is(err, billing.ErrInsufficientBalance):
			c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error(), "order": order})
		case errors.Is(err, billing.ErrPlanNotFound), errors.Is(err, billing.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, billing.ErrPlanDisabled):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, billing.ErrNoProvisioningTarget):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "order": order})
		case errors.Is(err, billing.ErrIdempotencyConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, billing.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "order": order})
		}
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *BillingHandler) subject(c *gin.Context) (int64, bool) {
	claims := middleware.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return 0, false
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return 0, false
	}
	return id, true
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}
