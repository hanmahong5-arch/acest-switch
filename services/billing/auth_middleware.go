package billing

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthConfig authentication middleware configuration
type AuthConfig struct {
	// SkipPaths paths that don't require authentication
	SkipPaths []string `json:"skip_paths"`
	// TokenHeader header name for access token
	TokenHeader string `json:"token_header"`
	// TokenPrefix prefix for token (e.g., "Bearer ")
	TokenPrefix string `json:"token_prefix"`
	// CacheEnabled enable token cache
	CacheEnabled bool `json:"cache_enabled"`
	// CacheTTL cache time-to-live
	CacheTTL time.Duration `json:"cache_ttl"`
}

// DefaultAuthConfig returns default auth configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		SkipPaths: []string{
			"/health",
			"/api/v1/auth/login",
			"/api/v1/auth/callback",
			"/api/v1/auth/refresh",
		},
		TokenHeader:  "Authorization",
		TokenPrefix:  "Bearer ",
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}
}

// AuthContext keys for storing user info in context
type contextKey string

const (
	ContextKeyUserID      contextKey = "user_id"
	ContextKeyUserName    contextKey = "user_name"
	ContextKeyUserEmail   contextKey = "user_email"
	ContextKeyIsAdmin     contextKey = "is_admin"
	ContextKeyAccessToken contextKey = "access_token"
	ContextKeyClaims      contextKey = "claims"
)

// cachedClaims represents cached token claims
type cachedClaims struct {
	claims    *CasdoorClaims
	expiresAt time.Time
}

// AuthMiddleware handles authentication
type AuthMiddleware struct {
	casdoor    *CasdoorService
	config     *AuthConfig
	cache      sync.Map // map[token]*cachedClaims
	skipPaths  map[string]bool
	mu         sync.RWMutex
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(casdoor *CasdoorService, config *AuthConfig) *AuthMiddleware {
	if config == nil {
		config = DefaultAuthConfig()
	}

	skipPaths := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	m := &AuthMiddleware{
		casdoor:   casdoor,
		config:    config,
		skipPaths: skipPaths,
	}

	// Start cache cleanup goroutine
	if config.CacheEnabled {
		go m.cleanupCache()
	}

	return m
}

// Handler returns Gin middleware handler
func (m *AuthMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if path should skip authentication
		if m.shouldSkip(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Extract token from header
		token := m.extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing or invalid authorization header",
			})
			return
		}

		// Validate token and get claims
		claims, err := m.validateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": err.Error(),
			})
			return
		}

		// Store user info in context
		m.setContextValues(c, claims, token)

		c.Next()
	}
}

// shouldSkip checks if path should skip authentication
func (m *AuthMiddleware) shouldSkip(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Exact match
	if m.skipPaths[path] {
		return true
	}

	// Prefix match for paths ending with *
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

// extractToken extracts token from request header
func (m *AuthMiddleware) extractToken(c *gin.Context) string {
	header := c.GetHeader(m.config.TokenHeader)
	if header == "" {
		return ""
	}

	if m.config.TokenPrefix != "" {
		if !strings.HasPrefix(header, m.config.TokenPrefix) {
			return ""
		}
		return strings.TrimPrefix(header, m.config.TokenPrefix)
	}

	return header
}

// validateToken validates token and returns claims
func (m *AuthMiddleware) validateToken(token string) (*CasdoorClaims, error) {
	// Check cache first
	if m.config.CacheEnabled {
		if cached, ok := m.cache.Load(token); ok {
			cc := cached.(*cachedClaims)
			if time.Now().Before(cc.expiresAt) {
				return cc.claims, nil
			}
			// Cache expired, remove it
			m.cache.Delete(token)
		}
	}

	// Parse and validate token
	claims, err := m.casdoor.ParseToken(token)
	if err != nil {
		return nil, err
	}

	// Cache the claims
	if m.config.CacheEnabled {
		m.cache.Store(token, &cachedClaims{
			claims:    claims,
			expiresAt: time.Now().Add(m.config.CacheTTL),
		})
	}

	return claims, nil
}

// setContextValues stores user info in Gin context
func (m *AuthMiddleware) setContextValues(c *gin.Context, claims *CasdoorClaims, token string) {
	// Set Gin context values
	c.Set(string(ContextKeyUserID), claims.Subject)
	c.Set(string(ContextKeyUserName), claims.Name)
	c.Set(string(ContextKeyUserEmail), claims.Email)
	c.Set(string(ContextKeyIsAdmin), claims.IsAdmin)
	c.Set(string(ContextKeyAccessToken), token)
	c.Set(string(ContextKeyClaims), claims)

	// Also set in request context for downstream services
	ctx := c.Request.Context()
	ctx = context.WithValue(ctx, ContextKeyUserID, claims.Subject)
	ctx = context.WithValue(ctx, ContextKeyUserName, claims.Name)
	ctx = context.WithValue(ctx, ContextKeyUserEmail, claims.Email)
	ctx = context.WithValue(ctx, ContextKeyIsAdmin, claims.IsAdmin)
	c.Request = c.Request.WithContext(ctx)
}

// cleanupCache periodically removes expired cache entries
func (m *AuthMiddleware) cleanupCache() {
	ticker := time.NewTicker(m.config.CacheTTL)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		m.cache.Range(func(key, value interface{}) bool {
			cc := value.(*cachedClaims)
			if now.After(cc.expiresAt) {
				m.cache.Delete(key)
			}
			return true
		})
	}
}

// AddSkipPath adds a path to skip list
func (m *AuthMiddleware) AddSkipPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skipPaths[path] = true
	m.config.SkipPaths = append(m.config.SkipPaths, path)
}

// RemoveSkipPath removes a path from skip list
func (m *AuthMiddleware) RemoveSkipPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.skipPaths, path)

	// Update config
	newPaths := make([]string, 0, len(m.config.SkipPaths))
	for _, p := range m.config.SkipPaths {
		if p != path {
			newPaths = append(newPaths, p)
		}
	}
	m.config.SkipPaths = newPaths
}

// InvalidateToken removes a token from cache
func (m *AuthMiddleware) InvalidateToken(token string) {
	m.cache.Delete(token)
}

// ClearCache clears all cached tokens
func (m *AuthMiddleware) ClearCache() {
	m.cache.Range(func(key, _ interface{}) bool {
		m.cache.Delete(key)
		return true
	})
}

// ============================================================
// Context Helper Functions
// ============================================================

// GetUserID extracts user ID from context
func GetUserID(c *gin.Context) string {
	if id, exists := c.Get(string(ContextKeyUserID)); exists {
		return id.(string)
	}
	return ""
}

// GetUserName extracts user name from context
func GetUserName(c *gin.Context) string {
	if name, exists := c.Get(string(ContextKeyUserName)); exists {
		return name.(string)
	}
	return ""
}

// GetUserEmail extracts user email from context
func GetUserEmail(c *gin.Context) string {
	if email, exists := c.Get(string(ContextKeyUserEmail)); exists {
		return email.(string)
	}
	return ""
}

// IsAdmin checks if current user is admin
func IsAdmin(c *gin.Context) bool {
	if isAdmin, exists := c.Get(string(ContextKeyIsAdmin)); exists {
		return isAdmin.(bool)
	}
	return false
}

// GetAccessToken extracts access token from context
func GetAccessToken(c *gin.Context) string {
	if token, exists := c.Get(string(ContextKeyAccessToken)); exists {
		return token.(string)
	}
	return ""
}

// GetClaims extracts full claims from context
func GetClaims(c *gin.Context) *CasdoorClaims {
	if claims, exists := c.Get(string(ContextKeyClaims)); exists {
		return claims.(*CasdoorClaims)
	}
	return nil
}

// GetUserIDFromContext extracts user ID from standard context
func GetUserIDFromContext(ctx context.Context) string {
	if id := ctx.Value(ContextKeyUserID); id != nil {
		return id.(string)
	}
	return ""
}

// ============================================================
// Admin Middleware
// ============================================================

// RequireAdmin creates middleware that requires admin privileges
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "admin privileges required",
			})
			return
		}
		c.Next()
	}
}

// RequireUser creates middleware that requires specific user or admin
func RequireUser(getUserIDParam func(c *gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID := GetUserID(c)
		targetUserID := getUserIDParam(c)

		// Admin can access any user's data
		if IsAdmin(c) {
			c.Next()
			return
		}

		// User can only access their own data
		if currentUserID != targetUserID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "access denied",
			})
			return
		}

		c.Next()
	}
}
