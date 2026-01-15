package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"codeswitch/services/billing"

	"github.com/gin-gonic/gin"
)

// BillingIntegration integrates billing services into the Gateway
type BillingIntegration struct {
	manager    *billing.BillingManager
	enabled    bool
	configPath string
	mu         sync.RWMutex
}

// NewBillingIntegration creates a new billing integration
func NewBillingIntegration() *BillingIntegration {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".code-switch", "billing.json")

	return &BillingIntegration{
		manager:    billing.NewBillingManager(),
		configPath: configPath,
	}
}

// Initialize initializes the billing system from config file or environment
func (bi *BillingIntegration) Initialize() error {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	// Try to load from config file first
	if _, err := os.Stat(bi.configPath); err == nil {
		if err := bi.manager.InitializeFromFile(bi.configPath); err != nil {
			fmt.Printf("[Billing] Failed to load config from file: %v\n", err)
			// Fall through to try environment variables
		} else {
			bi.enabled = true
			fmt.Printf("[Billing] Initialized from config file: %s\n", bi.configPath)
			return nil
		}
	}

	// Try to initialize from environment variables
	if os.Getenv("LAGO_API_URL") != "" || os.Getenv("CASDOOR_ENDPOINT") != "" {
		if err := bi.manager.InitializeFromEnv(); err != nil {
			return fmt.Errorf("failed to initialize from environment: %w", err)
		}
		bi.enabled = true
		fmt.Println("[Billing] Initialized from environment variables")
		return nil
	}

	// No configuration found - billing disabled
	fmt.Println("[Billing] No configuration found, billing disabled")
	return nil
}

// InitializeWithConfig initializes with explicit configuration
func (bi *BillingIntegration) InitializeWithConfig(config *billing.BillingManagerConfig) error {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	if err := bi.manager.Initialize(config); err != nil {
		return err
	}
	bi.enabled = true
	return nil
}

// IsEnabled returns whether billing is enabled
func (bi *BillingIntegration) IsEnabled() bool {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.enabled
}

// SetEnabled enables or disables billing
func (bi *BillingIntegration) SetEnabled(enabled bool) {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	bi.enabled = enabled
}

// Manager returns the underlying billing manager
func (bi *BillingIntegration) Manager() *billing.BillingManager {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	return bi.manager
}

// ============================================================
// Middleware Integration
// ============================================================

// AuthMiddleware returns Gin middleware for authentication
// Returns a no-op middleware if billing is disabled
func (bi *BillingIntegration) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil || manager.AuthMiddleware() == nil {
			c.Next()
			return
		}

		manager.AuthMiddleware().Handler()(c)
	}
}

// SubscriptionMiddleware returns Gin middleware for subscription check
// Returns a no-op middleware if billing is disabled
func (bi *BillingIntegration) SubscriptionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil || manager.SubscriptionMiddleware() == nil {
			c.Next()
			return
		}

		manager.SubscriptionMiddleware().Handler()(c)
	}
}

// BillingPreMiddleware returns Gin middleware for pre-request billing check (per-token mode)
// Returns a no-op middleware if billing is disabled
func (bi *BillingIntegration) BillingPreMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil || manager.BillingMiddleware() == nil {
			c.Next()
			return
		}

		manager.BillingMiddleware().PreRequestHandler()(c)
	}
}

// BillingPostMiddleware returns Gin middleware for post-request usage reporting (per-token mode)
// Returns a no-op middleware if billing is disabled
func (bi *BillingIntegration) BillingPostMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil || manager.BillingMiddleware() == nil {
			c.Next()
			return
		}

		manager.BillingMiddleware().PostRequestHandler()(c)
	}
}

// ============================================================
// Route Registration Helpers
// ============================================================

// WrapWithBilling wraps a handler with billing middleware
func (bi *BillingIntegration) WrapWithBilling(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil {
			handler(c)
			return
		}

		// Pre-request: check balance
		if bm := manager.BillingMiddleware(); bm != nil {
			bm.PreRequestHandler()(c)
			if c.IsAborted() {
				return
			}
		}

		// Execute original handler
		handler(c)

		// Post-request: report usage
		if bm := manager.BillingMiddleware(); bm != nil {
			bm.PostRequestHandler()(c)
		}
	}
}

// WrapWithAuth wraps a handler with authentication middleware
func (bi *BillingIntegration) WrapWithAuth(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil || manager.AuthMiddleware() == nil {
			handler(c)
			return
		}

		manager.AuthMiddleware().Handler()(c)
		if c.IsAborted() {
			return
		}

		handler(c)
	}
}

// WrapWithAuthAndSubscription wraps a handler with auth and subscription check
// This is the recommended wrapper for subscription-only billing mode
func (bi *BillingIntegration) WrapWithAuthAndSubscription(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil {
			handler(c)
			return
		}

		// Auth check
		if am := manager.AuthMiddleware(); am != nil {
			am.Handler()(c)
			if c.IsAborted() {
				return
			}
		}

		// Subscription check
		if sm := manager.SubscriptionMiddleware(); sm != nil {
			sm.Handler()(c)
			if c.IsAborted() {
				return
			}
		}

		// Execute original handler
		handler(c)
	}
}

// WrapWithAuthAndBilling wraps a handler with both auth and per-token billing middleware
func (bi *BillingIntegration) WrapWithAuthAndBilling(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil {
			handler(c)
			return
		}

		// Auth check
		if am := manager.AuthMiddleware(); am != nil {
			am.Handler()(c)
			if c.IsAborted() {
				return
			}
		}

		// Pre-request billing check
		if bm := manager.BillingMiddleware(); bm != nil {
			bm.PreRequestHandler()(c)
			if c.IsAborted() {
				return
			}
		}

		// Execute original handler
		handler(c)

		// Post-request: report usage
		if bm := manager.BillingMiddleware(); bm != nil {
			bm.PostRequestHandler()(c)
		}
	}
}

// ============================================================
// API Endpoints
// ============================================================

// RegisterBillingRoutes registers billing-related API routes
func (bi *BillingIntegration) RegisterBillingRoutes(router gin.IRouter) {
	// Billing status endpoint (public)
	router.GET("/api/v1/billing/status", func(c *gin.Context) {
		bi.mu.RLock()
		enabled := bi.enabled
		manager := bi.manager
		bi.mu.RUnlock()

		if !enabled || manager == nil {
			c.JSON(200, gin.H{
				"enabled": false,
				"message": "Billing is not configured",
			})
			return
		}

		c.JSON(200, gin.H{
			"enabled":  true,
			"services": manager.Status(),
		})
	})

	// Auth routes (public)
	authGroup := router.Group("/api/v1/auth")
	{
		// OAuth login URL
		authGroup.GET("/login", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Casdoor() == nil {
				c.JSON(503, gin.H{"error": "Authentication service not configured"})
				return
			}

			redirectURI := c.Query("redirect_uri")
			if redirectURI == "" {
				redirectURI = c.Request.Host + "/api/v1/auth/callback"
			}

			state := c.Query("state")
			authURL := manager.Casdoor().GetAuthURL(redirectURI, state)

			c.JSON(200, gin.H{
				"auth_url": authURL,
			})
		})

		// OAuth callback
		authGroup.GET("/callback", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Casdoor() == nil {
				c.JSON(503, gin.H{"error": "Authentication service not configured"})
				return
			}

			code := c.Query("code")
			if code == "" {
				c.JSON(400, gin.H{"error": "Missing authorization code"})
				return
			}

			redirectURI := c.Query("redirect_uri")
			token, err := manager.Casdoor().ExchangeToken(code, redirectURI)
			if err != nil {
				c.JSON(500, gin.H{"error": "Token exchange failed", "details": err.Error()})
				return
			}

			c.JSON(200, token)
		})

		// Token refresh
		authGroup.POST("/refresh", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Casdoor() == nil {
				c.JSON(503, gin.H{"error": "Authentication service not configured"})
				return
			}

			var req struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "Invalid request"})
				return
			}

			token, err := manager.Casdoor().RefreshToken(req.RefreshToken)
			if err != nil {
				c.JSON(500, gin.H{"error": "Token refresh failed", "details": err.Error()})
				return
			}

			c.JSON(200, token)
		})
	}

	// Protected billing routes (require authentication)
	billingGroup := router.Group("/api/v1/billing")
	billingGroup.Use(bi.AuthMiddleware())
	{
		// Get user quota/balance
		billingGroup.GET("/quota", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.BillingMiddleware() == nil {
				c.JSON(503, gin.H{"error": "Billing service not configured"})
				return
			}

			userID := billing.GetUserID(c)
			if userID == "" {
				c.JSON(401, gin.H{"error": "User not authenticated"})
				return
			}

			quota, err := manager.BillingMiddleware().GetQuotaInfo(userID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to get quota", "details": err.Error()})
				return
			}

			c.JSON(200, quota)
		})

		// Get user subscriptions (raw list)
		billingGroup.GET("/subscriptions", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Lago() == nil {
				c.JSON(503, gin.H{"error": "Billing service not configured"})
				return
			}

			userID := billing.GetUserID(c)
			subs, err := manager.GetUserSubscriptions(userID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to get subscriptions", "details": err.Error()})
				return
			}

			c.JSON(200, gin.H{"subscriptions": subs})
		})

		// Get subscription info (formatted, with remaining days)
		billingGroup.GET("/subscription-info", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.SubscriptionMiddleware() == nil {
				c.JSON(503, gin.H{"error": "Subscription service not configured"})
				return
			}

			userID := billing.GetUserID(c)
			if userID == "" {
				c.JSON(401, gin.H{"error": "User not authenticated"})
				return
			}

			info, err := manager.SubscriptionMiddleware().GetSubscriptionInfo(userID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to get subscription info", "details": err.Error()})
				return
			}

			c.JSON(200, info)
		})

		// Create subscription
		billingGroup.POST("/subscriptions", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Lago() == nil {
				c.JSON(503, gin.H{"error": "Billing service not configured"})
				return
			}

			var req struct {
				PlanCode string `json:"plan_code"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "Invalid request"})
				return
			}

			userID := billing.GetUserID(c)
			sub, err := manager.CreateSubscription(userID, req.PlanCode)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to create subscription", "details": err.Error()})
				return
			}

			c.JSON(200, sub)
		})

		// Get user balance
		billingGroup.GET("/balance", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Lago() == nil {
				c.JSON(503, gin.H{"error": "Billing service not configured"})
				return
			}

			userID := billing.GetUserID(c)
			balance, err := manager.GetUserBalance(userID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to get balance", "details": err.Error()})
				return
			}

			c.JSON(200, gin.H{
				"user_id": userID,
				"balance": balance,
			})
		})
	}

	// Payment routes
	paymentGroup := router.Group("/api/v1/payment")
	paymentGroup.Use(bi.AuthMiddleware())
	{
		// Create recharge order
		paymentGroup.POST("/recharge", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Payment() == nil {
				c.JSON(503, gin.H{"error": "Payment service not configured"})
				return
			}

			var req struct {
				Amount      int64  `json:"amount"`
				Method      string `json:"method"` // "alipay" or "wechat"
				Description string `json:"description"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "Invalid request"})
				return
			}

			userID := billing.GetUserID(c)
			var method billing.PaymentMethod
			switch req.Method {
			case "alipay":
				method = billing.PaymentMethodAlipay
			case "wechat":
				method = billing.PaymentMethodWechat
			default:
				c.JSON(400, gin.H{"error": "Invalid payment method"})
				return
			}

			order, err := manager.Payment().CreateRechargeOrder(userID, req.Amount, method, req.Description)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to create order", "details": err.Error()})
				return
			}

			c.JSON(200, order)
		})
	}

	// Payment callbacks (public, webhook endpoints)
	callbackGroup := router.Group("/api/v1/payment")
	{
		// Alipay callback
		callbackGroup.POST("/alipay/notify", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Payment() == nil {
				c.String(500, "fail")
				return
			}

			// Parse form data
			if err := c.Request.ParseForm(); err != nil {
				c.String(400, "fail")
				return
			}

			params := make(map[string]string)
			for key, values := range c.Request.Form {
				if len(values) > 0 {
					params[key] = values[0]
				}
			}

			_, err := manager.Payment().HandleAlipayCallback(params)
			if err != nil {
				c.String(500, "fail")
				return
			}

			c.String(200, "success")
		})

		// WeChat callback
		callbackGroup.POST("/wechat/notify", func(c *gin.Context) {
			bi.mu.RLock()
			manager := bi.manager
			bi.mu.RUnlock()

			if manager == nil || manager.Payment() == nil {
				c.JSON(500, gin.H{"code": "FAIL", "message": "Service unavailable"})
				return
			}

			// WeChat Pay V3 notification handling would go here
			// This requires parsing the encrypted notification body
			// For now, return success placeholder
			c.JSON(200, gin.H{"code": "SUCCESS", "message": "OK"})
		})
	}
}

// ============================================================
// Configuration Management
// ============================================================

// SaveConfig saves current configuration to file
func (bi *BillingIntegration) SaveConfig() error {
	bi.mu.RLock()
	manager := bi.manager
	bi.mu.RUnlock()

	if manager == nil {
		return fmt.Errorf("billing manager not initialized")
	}

	return manager.SaveConfig(bi.configPath)
}

// LoadConfig loads configuration from file
func (bi *BillingIntegration) LoadConfig() (*billing.BillingManagerConfig, error) {
	data, err := os.ReadFile(bi.configPath)
	if err != nil {
		return nil, err
	}

	var config billing.BillingManagerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetConfigPath returns the configuration file path
func (bi *BillingIntegration) GetConfigPath() string {
	return bi.configPath
}

// ============================================================
// Token Usage Reporting Hook
// ============================================================

// ReportTokenUsage reports token usage for a request
// This should be called after LLM response is complete
func (bi *BillingIntegration) ReportTokenUsage(c *gin.Context, inputTokens, outputTokens int) {
	bi.mu.RLock()
	enabled := bi.enabled
	manager := bi.manager
	bi.mu.RUnlock()

	if !enabled || manager == nil {
		return
	}

	// Store in context for post-middleware to pick up
	c.Set("input_tokens", inputTokens)
	c.Set("output_tokens", outputTokens)
}

// ============================================================
// Shutdown
// ============================================================

// Shutdown gracefully shuts down billing services
func (bi *BillingIntegration) Shutdown() error {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	if bi.manager != nil {
		return bi.manager.Shutdown()
	}
	return nil
}
