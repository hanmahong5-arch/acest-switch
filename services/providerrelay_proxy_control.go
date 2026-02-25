package services

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProxyControlMiddleware returns middleware that checks if proxy is enabled for the app
func (prs *ProviderRelayService) ProxyControlMiddleware(proxyController *ProxyController) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Detect app from request path
		appName := detectAppFromPath(c.Request.URL.Path)

		// Check if proxy is enabled for this app
		if !proxyController.IsProxyEnabled(appName) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   fmt.Sprintf("%s proxy is currently disabled", appName),
				"type":    "proxy_disabled",
				"app":     appName,
				"message": "Please enable proxy for this application in settings",
			})
			c.Abort()
			return
		}

		// Record request
		go proxyController.RecordRequest(appName)

		// Continue to next handler
		c.Next()
	}
}

// detectAppFromPath detects application name from request path
func detectAppFromPath(path string) string {
	path = strings.ToLower(path)

	// PicoClaw routes (must check before generic OpenAI routes)
	if strings.HasPrefix(path, "/pc/") {
		return "picoclaw"
	}

	// Claude Code routes
	if strings.Contains(path, "/v1/messages") {
		return "claude"
	}

	// Codex routes
	if strings.Contains(path, "/responses") {
		return "codex"
	}

	// Gemini routes
	if strings.Contains(path, "/v1beta/models") || strings.Contains(path, "/models/") {
		return "gemini"
	}

	// OpenAI-compatible routes (default to codex)
	if strings.Contains(path, "/v1/chat/completions") || strings.Contains(path, "/chat/completions") {
		return "codex"
	}

	// Default to unknown
	return "unknown"
}

// ProxyControlExtension extends ProviderRelayService with proxy control
type ProxyControlExtension struct {
	*ProviderRelayService
	proxyController *ProxyController
}

// NewProxyControlExtension creates a new proxy control extension
func NewProxyControlExtension(prs *ProviderRelayService, pc *ProxyController) *ProxyControlExtension {
	return &ProxyControlExtension{
		ProviderRelayService: prs,
		proxyController:      pc,
	}
}

// GetProxyController returns the proxy controller
func (pce *ProxyControlExtension) GetProxyController() *ProxyController {
	return pce.proxyController
}

// HandleGetProxyConfigs returns proxy control configurations
func (pce *ProxyControlExtension) HandleGetProxyConfigs(c *gin.Context) {
	configs, err := pce.proxyController.GetAllConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get proxy configurations",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
	})
}

// HandleToggleProxy toggles proxy for an app
func (pce *ProxyControlExtension) HandleToggleProxy(c *gin.Context) {
	appName := c.Param("app")
	if appName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "app name is required",
		})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	if err := pce.proxyController.ToggleProxy(appName, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to toggle proxy: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"app":     appName,
		"enabled": req.Enabled,
	})
}

// HandleGetProxyStats returns proxy statistics
func (pce *ProxyControlExtension) HandleGetProxyStats(c *gin.Context) {
	stats, err := pce.proxyController.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get proxy statistics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// HandleUpdateProxyConfig updates proxy configuration for an app
func (pce *ProxyControlExtension) HandleUpdateProxyConfig(c *gin.Context) {
	appName := c.Param("app")
	if appName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "app name is required",
		})
		return
	}

	var config ProxyControlConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Ensure app name matches
	config.AppName = appName

	if err := pce.proxyController.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to update proxy config: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  config,
	})
}

// RegisterProxyControlRoutes registers proxy control API routes
func (pce *ProxyControlExtension) RegisterProxyControlRoutes(router gin.IRouter) {
	// Get all proxy configurations
	router.GET("/api/proxy-control", pce.HandleGetProxyConfigs)

	// Get proxy statistics
	router.GET("/api/proxy-control/stats", pce.HandleGetProxyStats)

	// Toggle proxy for an app
	router.POST("/api/proxy-control/:app/toggle", pce.HandleToggleProxy)

	// Update proxy configuration
	router.PUT("/api/proxy-control/:app", pce.HandleUpdateProxyConfig)
}
