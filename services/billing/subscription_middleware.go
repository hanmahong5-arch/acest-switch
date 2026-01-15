package billing

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SubscriptionConfig subscription-only middleware configuration
type SubscriptionConfig struct {
	// Enabled toggle subscription check
	Enabled bool `json:"enabled"`
	// SkipPaths paths that don't require subscription
	SkipPaths []string `json:"skip_paths"`
	// AllowedPlans plans that grant access (empty = any active subscription)
	AllowedPlans []string `json:"allowed_plans"`
	// CacheTTL how long to cache subscription status
	CacheTTL time.Duration `json:"cache_ttl"`
	// GracePeriod allow access for this duration after subscription expires
	GracePeriod time.Duration `json:"grace_period"`
}

// DefaultSubscriptionConfig returns default subscription configuration
func DefaultSubscriptionConfig() *SubscriptionConfig {
	return &SubscriptionConfig{
		Enabled: true,
		SkipPaths: []string{
			"/health",
			"/api/v1/auth/*",
			"/api/v1/billing/*",
			"/api/v1/payment/*",
		},
		AllowedPlans: []string{}, // Empty = any active subscription
		CacheTTL:     5 * time.Minute,
		GracePeriod:  24 * time.Hour, // 1 day grace period
	}
}

// cachedSubscription represents cached subscription status
type cachedSubscription struct {
	hasActiveSubscription bool
	planCode              string
	expiresAt             time.Time
	cacheExpiresAt        time.Time
}

// SubscriptionMiddleware handles subscription-only access control
type SubscriptionMiddleware struct {
	lago      *LagoService
	config    *SubscriptionConfig
	cache     sync.Map // map[userID]*cachedSubscription
	skipPaths map[string]bool
	mu        sync.RWMutex
}

// NewSubscriptionMiddleware creates a new subscription middleware
func NewSubscriptionMiddleware(lago *LagoService, config *SubscriptionConfig) *SubscriptionMiddleware {
	if config == nil {
		config = DefaultSubscriptionConfig()
	}

	skipPaths := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	m := &SubscriptionMiddleware{
		lago:      lago,
		config:    config,
		skipPaths: skipPaths,
	}

	// Start cache cleanup
	go m.cleanupCache()

	return m
}

// Handler returns Gin middleware handler
func (m *SubscriptionMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Enabled {
			c.Next()
			return
		}

		// Check if path should skip subscription check
		if m.shouldSkip(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Get user ID from auth context
		userID := GetUserID(c)
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "user not authenticated",
			})
			return
		}

		// Check subscription status
		hasSubscription, planCode, err := m.checkSubscription(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":   "service_error",
				"message": "failed to verify subscription",
			})
			return
		}

		if !hasSubscription {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
				"error":   "subscription_required",
				"message": "active subscription required to access this resource",
				"plans": []string{
					"monthly_vip",
					"yearly_vip",
				},
			})
			return
		}

		// Store subscription info in context
		c.Set("subscription_plan", planCode)
		c.Set("has_subscription", true)

		c.Next()
	}
}

// shouldSkip checks if path should skip subscription check
func (m *SubscriptionMiddleware) shouldSkip(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.skipPaths[path] {
		return true
	}

	// Check wildcard paths
	for skipPath := range m.skipPaths {
		if strings.HasSuffix(skipPath, "*") {
			prefix := strings.TrimSuffix(skipPath, "*")
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}

	return false
}

// checkSubscription checks if user has an active subscription
func (m *SubscriptionMiddleware) checkSubscription(userID string) (bool, string, error) {
	// Check cache first
	if cached, ok := m.cache.Load(userID); ok {
		cs := cached.(*cachedSubscription)
		if time.Now().Before(cs.cacheExpiresAt) {
			return cs.hasActiveSubscription, cs.planCode, nil
		}
		m.cache.Delete(userID)
	}

	// Query Lago for subscriptions
	subscriptions, err := m.lago.GetCustomerSubscriptions(userID)
	if err != nil {
		return false, "", err
	}

	// Check for active subscription
	var hasActive bool
	var activePlan string
	now := time.Now()

	for _, sub := range subscriptions {
		if sub.Status == "active" {
			// Check if plan is in allowed list (if specified)
			if len(m.config.AllowedPlans) > 0 {
				allowed := false
				for _, plan := range m.config.AllowedPlans {
					if sub.PlanCode == plan {
						allowed = true
						break
					}
				}
				if !allowed {
					continue
				}
			}

			hasActive = true
			activePlan = sub.PlanCode
			break
		}

		// Check grace period for terminated subscriptions
		if sub.Status == "terminated" && !sub.TerminatedAt.IsZero() {
			gracePeriodEnd := sub.TerminatedAt.Add(m.config.GracePeriod)
			if now.Before(gracePeriodEnd) {
				hasActive = true
				activePlan = sub.PlanCode + " (grace)"
				break
			}
		}
	}

	// Cache the result
	m.cache.Store(userID, &cachedSubscription{
		hasActiveSubscription: hasActive,
		planCode:              activePlan,
		cacheExpiresAt:        time.Now().Add(m.config.CacheTTL),
	})

	return hasActive, activePlan, nil
}

// cleanupCache periodically removes expired cache entries
func (m *SubscriptionMiddleware) cleanupCache() {
	ticker := time.NewTicker(m.config.CacheTTL)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		m.cache.Range(func(key, value interface{}) bool {
			cs := value.(*cachedSubscription)
			if now.After(cs.cacheExpiresAt) {
				m.cache.Delete(key)
			}
			return true
		})
	}
}

// InvalidateCache clears subscription cache for a user
func (m *SubscriptionMiddleware) InvalidateCache(userID string) {
	m.cache.Delete(userID)
}

// ClearCache clears all cached subscriptions
func (m *SubscriptionMiddleware) ClearCache() {
	m.cache.Range(func(key, _ interface{}) bool {
		m.cache.Delete(key)
		return true
	})
}

// GetConfig returns current configuration
func (m *SubscriptionMiddleware) GetConfig() *SubscriptionConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates configuration
func (m *SubscriptionMiddleware) UpdateConfig(config *SubscriptionConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config

	m.skipPaths = make(map[string]bool)
	for _, path := range config.SkipPaths {
		m.skipPaths[path] = true
	}
}

// SetEnabled enables or disables subscription check
func (m *SubscriptionMiddleware) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Enabled = enabled
}

// ============================================================
// Context Helpers
// ============================================================

// GetSubscriptionPlan returns the user's subscription plan from context
func GetSubscriptionPlan(c *gin.Context) string {
	if plan, exists := c.Get("subscription_plan"); exists {
		return plan.(string)
	}
	return ""
}

// HasSubscription checks if user has an active subscription
func HasSubscription(c *gin.Context) bool {
	if has, exists := c.Get("has_subscription"); exists {
		return has.(bool)
	}
	return false
}

// ============================================================
// Subscription Info
// ============================================================

// SubscriptionInfo represents user's subscription information
type SubscriptionInfo struct {
	UserID            string `json:"user_id"`
	HasSubscription   bool   `json:"has_subscription"`
	PlanCode          string `json:"plan_code,omitempty"`
	PlanName          string `json:"plan_name,omitempty"`
	Status            string `json:"status,omitempty"`
	StartedAt         string `json:"started_at,omitempty"`
	CurrentPeriodEnd  string `json:"current_period_end,omitempty"`
	RemainingDays     int    `json:"remaining_days,omitempty"`
}

// GetSubscriptionInfo gets detailed subscription info for a user
func (m *SubscriptionMiddleware) GetSubscriptionInfo(userID string) (*SubscriptionInfo, error) {
	subscriptions, err := m.lago.GetCustomerSubscriptions(userID)
	if err != nil {
		return nil, err
	}

	info := &SubscriptionInfo{
		UserID:          userID,
		HasSubscription: false,
	}

	for _, sub := range subscriptions {
		if sub.Status == "active" {
			info.HasSubscription = true
			info.PlanCode = sub.PlanCode
			info.Status = sub.Status

			// Map plan code to name
			switch sub.PlanCode {
			case "monthly_vip":
				info.PlanName = "月度 VIP"
			case "yearly_vip":
				info.PlanName = "年度 VIP"
			case "free":
				info.PlanName = "免费版"
			default:
				info.PlanName = sub.PlanCode
			}

			if !sub.StartedAt.IsZero() {
				info.StartedAt = sub.StartedAt.Format("2006-01-02")
			}
			if !sub.EndingAt.IsZero() {
				info.CurrentPeriodEnd = sub.EndingAt.Format("2006-01-02")
				info.RemainingDays = int(time.Until(sub.EndingAt).Hours() / 24)
				if info.RemainingDays < 0 {
					info.RemainingDays = 0
				}
			}
			break
		}
	}

	return info, nil
}
