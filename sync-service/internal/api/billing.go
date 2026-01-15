package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/internal/admin"
	"github.com/aspect-code/codeswitch/sync-service/pkg/models"
	"github.com/gin-gonic/gin"
)

// BillingHandlers handles billing API requests
type BillingHandlers struct {
	billingService *admin.BillingService
	auditService   *admin.AuditService
}

// NewBillingHandlers creates billing handlers
func NewBillingHandlers(billingService *admin.BillingService, auditService *admin.AuditService) *BillingHandlers {
	return &BillingHandlers{
		billingService: billingService,
		auditService:   auditService,
	}
}

// RegisterBillingRoutes registers billing routes
func (h *BillingHandlers) RegisterBillingRoutes(router *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Billing routes (require authentication)
	billingGroup := router.Group("/billing")
	billingGroup.Use(authMiddleware)
	{
		// Plans (public within authenticated users)
		billingGroup.GET("/plans", h.listPlans)

		// Subscriptions
		billingGroup.GET("/subscriptions", h.listSubscriptions)
		billingGroup.GET("/subscriptions/:id", h.getSubscription)
		billingGroup.POST("/subscriptions", h.createSubscription)
		billingGroup.POST("/subscriptions/:id/cancel", h.cancelSubscription)
		billingGroup.POST("/subscriptions/:id/reactivate", h.reactivateSubscription)

		// Wallets
		billingGroup.GET("/wallets", h.listWallets)
		billingGroup.GET("/wallets/:id", h.getWallet)
		billingGroup.POST("/wallets", h.createWallet)
		billingGroup.POST("/wallets/:id/top-up", h.topUpWallet)
		billingGroup.GET("/wallets/:id/transactions", h.getWalletTransactions)

		// Payments
		billingGroup.GET("/payments", h.listPayments)
		billingGroup.GET("/payments/:id", h.getPayment)
		billingGroup.POST("/payments", h.createPayment)
		billingGroup.POST("/payments/:id/refund", h.refundPayment)
		billingGroup.POST("/payments/:id/confirm", h.confirmPayment)

		// Configuration (admin only)
		billingGroup.GET("/config", h.adminOnly(), h.getConfig)
		billingGroup.PUT("/config", h.adminOnly(), h.updateConfig)
		billingGroup.GET("/status", h.getStatus)

		// Connection tests (admin only)
		billingGroup.POST("/test/casdoor", h.adminOnly(), h.testCasdoor)
		billingGroup.POST("/test/lago", h.adminOnly(), h.testLago)
		billingGroup.POST("/test/payment/:method", h.adminOnly(), h.testPayment)

		// User quick queries
		billingGroup.GET("/users/:id/subscription-status", h.getUserSubscriptionStatus)
		billingGroup.GET("/users/:id/balance", h.getUserBalance)
	}
}

// adminOnly middleware for admin-only routes
func (h *BillingHandlers) adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("is_admin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ===== Plans =====

func (h *BillingHandlers) listPlans(c *gin.Context) {
	plans := h.billingService.ListPlans()
	c.JSON(http.StatusOK, plans)
}

// ===== Subscriptions =====

func (h *BillingHandlers) listSubscriptions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userID := c.Query("user_id")
	status := c.Query("status")
	planCode := c.Query("plan_code")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	result := h.billingService.ListSubscriptions(page, pageSize, userID, status, planCode)
	c.JSON(http.StatusOK, result)
}

func (h *BillingHandlers) getSubscription(c *gin.Context) {
	id := c.Param("id")
	sub, err := h.billingService.GetSubscription(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *BillingHandlers) createSubscription(c *gin.Context) {
	var req models.CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.billingService.CreateSubscription(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Audit log
	h.logAction(c, "subscription.create", "subscription", sub.ID, "success", map[string]interface{}{
		"user_id":   req.UserID,
		"plan_code": req.PlanCode,
	})

	c.JSON(http.StatusCreated, sub)
}

func (h *BillingHandlers) cancelSubscription(c *gin.Context) {
	id := c.Param("id")
	if err := h.billingService.CancelSubscription(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logAction(c, "subscription.cancel", "subscription", id, "success", nil)
	c.JSON(http.StatusOK, models.MessageResponse{Message: "Subscription canceled"})
}

func (h *BillingHandlers) reactivateSubscription(c *gin.Context) {
	id := c.Param("id")
	if err := h.billingService.ReactivateSubscription(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logAction(c, "subscription.reactivate", "subscription", id, "success", nil)
	c.JSON(http.StatusOK, models.MessageResponse{Message: "Subscription reactivated"})
}

// ===== Wallets =====

func (h *BillingHandlers) listWallets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userID := c.Query("user_id")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	result := h.billingService.ListWallets(page, pageSize, userID, status)
	c.JSON(http.StatusOK, result)
}

func (h *BillingHandlers) getWallet(c *gin.Context) {
	id := c.Param("id")
	wallet, err := h.billingService.GetWallet(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, wallet)
}

func (h *BillingHandlers) createWallet(c *gin.Context) {
	var req models.CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wallet, err := h.billingService.CreateWallet(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logAction(c, "wallet.create", "wallet", wallet.ID, "success", map[string]interface{}{
		"user_id":  req.UserID,
		"currency": req.Currency,
	})

	c.JSON(http.StatusCreated, wallet)
}

func (h *BillingHandlers) topUpWallet(c *gin.Context) {
	id := c.Param("id")
	var req models.TopUpWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.billingService.TopUpWallet(id, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logAction(c, "wallet.topup", "wallet", id, "success", map[string]interface{}{
		"paid_credits":    req.PaidCredits,
		"granted_credits": req.GrantedCredits,
	})

	c.JSON(http.StatusOK, models.MessageResponse{Message: "Wallet topped up successfully"})
}

func (h *BillingHandlers) getWalletTransactions(c *gin.Context) {
	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	result := h.billingService.GetWalletTransactions(id, page, pageSize)
	c.JSON(http.StatusOK, result)
}

// ===== Payments =====

func (h *BillingHandlers) listPayments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userID := c.Query("user_id")
	status := c.Query("status")
	method := c.Query("method")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var startTime, endTime *time.Time
	if st := c.Query("start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			startTime = &t
		}
	}
	if et := c.Query("end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			endTime = &t
		}
	}

	result := h.billingService.ListPayments(page, pageSize, userID, status, method, startTime, endTime)
	c.JSON(http.StatusOK, result)
}

func (h *BillingHandlers) getPayment(c *gin.Context) {
	id := c.Param("id")
	payment, err := h.billingService.GetPayment(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, payment)
}

func (h *BillingHandlers) createPayment(c *gin.Context) {
	var req models.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payment, err := h.billingService.CreatePayment(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logAction(c, "payment.create", "payment", payment.ID, "success", map[string]interface{}{
		"user_id":      req.UserID,
		"amount_cents": req.AmountCents,
		"method":       req.Method,
	})

	c.JSON(http.StatusCreated, payment)
}

func (h *BillingHandlers) refundPayment(c *gin.Context) {
	id := c.Param("id")
	var req models.RefundPaymentRequest
	c.ShouldBindJSON(&req) // Optional body

	if err := h.billingService.RefundPayment(id, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logAction(c, "payment.refund", "payment", id, "success", map[string]interface{}{
		"amount_cents": req.AmountCents,
		"reason":       req.Reason,
	})

	c.JSON(http.StatusOK, models.MessageResponse{Message: "Payment refunded"})
}

func (h *BillingHandlers) confirmPayment(c *gin.Context) {
	id := c.Param("id")
	var req models.ConfirmPaymentRequest
	c.ShouldBindJSON(&req) // Optional body

	if err := h.billingService.ConfirmPayment(id, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logAction(c, "payment.confirm", "payment", id, "success", map[string]interface{}{
		"note": req.Note,
	})

	c.JSON(http.StatusOK, models.MessageResponse{Message: "Payment confirmed"})
}

// ===== Configuration =====

func (h *BillingHandlers) getConfig(c *gin.Context) {
	config := h.billingService.GetConfig()
	// Mask sensitive fields
	maskedConfig := config
	if maskedConfig.CasdoorClientSecret != "" {
		maskedConfig.CasdoorClientSecret = "***"
	}
	if maskedConfig.LagoAPIKey != "" {
		maskedConfig.LagoAPIKey = "***"
	}
	if maskedConfig.AlipayPrivateKey != "" {
		maskedConfig.AlipayPrivateKey = "***"
	}
	if maskedConfig.WechatAPIKey != "" {
		maskedConfig.WechatAPIKey = "***"
	}
	if maskedConfig.WechatAPIKeyV3 != "" {
		maskedConfig.WechatAPIKeyV3 = "***"
	}
	if maskedConfig.WechatPrivateKey != "" {
		maskedConfig.WechatPrivateKey = "***"
	}
	c.JSON(http.StatusOK, maskedConfig)
}

func (h *BillingHandlers) updateConfig(c *gin.Context) {
	var config models.BillingConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Preserve existing secrets if new ones are masked or empty
	existingConfig := h.billingService.GetConfig()
	if config.CasdoorClientSecret == "" || config.CasdoorClientSecret == "***" {
		config.CasdoorClientSecret = existingConfig.CasdoorClientSecret
	}
	if config.LagoAPIKey == "" || config.LagoAPIKey == "***" {
		config.LagoAPIKey = existingConfig.LagoAPIKey
	}
	if config.AlipayPrivateKey == "" || config.AlipayPrivateKey == "***" {
		config.AlipayPrivateKey = existingConfig.AlipayPrivateKey
	}
	if config.WechatAPIKey == "" || config.WechatAPIKey == "***" {
		config.WechatAPIKey = existingConfig.WechatAPIKey
	}
	if config.WechatAPIKeyV3 == "" || config.WechatAPIKeyV3 == "***" {
		config.WechatAPIKeyV3 = existingConfig.WechatAPIKeyV3
	}
	if config.WechatPrivateKey == "" || config.WechatPrivateKey == "***" {
		config.WechatPrivateKey = existingConfig.WechatPrivateKey
	}

	if err := h.billingService.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logAction(c, "billing.config.update", "config", "billing", "success", nil)
	c.JSON(http.StatusOK, models.MessageResponse{Message: "Configuration updated"})
}

func (h *BillingHandlers) getStatus(c *gin.Context) {
	status := h.billingService.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ===== Connection Tests =====

func (h *BillingHandlers) testCasdoor(c *gin.Context) {
	result := h.billingService.TestCasdoorConnection()
	c.JSON(http.StatusOK, result)
}

func (h *BillingHandlers) testLago(c *gin.Context) {
	result := h.billingService.TestLagoConnection()
	c.JSON(http.StatusOK, result)
}

func (h *BillingHandlers) testPayment(c *gin.Context) {
	method := c.Param("method")
	result := h.billingService.TestPaymentConnection(method)
	c.JSON(http.StatusOK, result)
}

// ===== User Quick Queries =====

func (h *BillingHandlers) getUserSubscriptionStatus(c *gin.Context) {
	userID := c.Param("id")
	status := h.billingService.GetUserSubscriptionStatus(userID)
	c.JSON(http.StatusOK, status)
}

func (h *BillingHandlers) getUserBalance(c *gin.Context) {
	userID := c.Param("id")
	balance := h.billingService.GetUserBalance(userID)
	c.JSON(http.StatusOK, balance)
}

// ===== Helpers =====

func (h *BillingHandlers) logAction(c *gin.Context, action, resourceType, resourceID, result string, details map[string]interface{}) {
	if h.auditService == nil {
		return
	}
	userID := c.GetString("user_id")
	username := c.GetString("username")
	h.auditService.LogAction(userID, username, action, resourceType, resourceID, result, c.ClientIP(), details)
}
