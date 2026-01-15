package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/internal/admin"
	"github.com/gin-gonic/gin"
)

// AdminHandlers 管理后台处理器
type AdminHandlers struct {
	statsService    *admin.StatsService
	monitorService  *admin.MonitorService
	userManager     *admin.UserManager
	metricsExporter *admin.MetricsExporter
	auditService    *admin.AuditService
	alertService    *admin.AlertService
	version         string
}

// NewAdminHandlers 创建管理后台处理器
func NewAdminHandlers(
	statsService *admin.StatsService,
	monitorService *admin.MonitorService,
	userManager *admin.UserManager,
	auditService *admin.AuditService,
	alertService *admin.AlertService,
	version string,
) *AdminHandlers {
	return &AdminHandlers{
		statsService:    statsService,
		monitorService:  monitorService,
		userManager:     userManager,
		auditService:    auditService,
		alertService:    alertService,
		metricsExporter: admin.NewMetricsExporter(statsService, monitorService, userManager, version),
		version:         version,
	}
}

// RegisterAdminRoutes 注册管理后台路由
func (h *AdminHandlers) RegisterAdminRoutes(router *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// 管理后台路由组（需要管理员权限）
	adminGroup := router.Group("/admin")
	adminGroup.Use(authMiddleware)
	adminGroup.Use(h.adminOnlyMiddleware())
	{
		// 系统监控
		adminGroup.GET("/system/status", h.getSystemStatus)
		adminGroup.GET("/system/health", h.getSystemHealth)

		// 统计分析
		adminGroup.GET("/stats/overview", h.getStatsOverview)
		adminGroup.GET("/stats/hourly", h.getHourlyStats)
		adminGroup.GET("/stats/daily", h.getDailyStats)
		adminGroup.GET("/stats/providers", h.getProviderStats)
		adminGroup.GET("/stats/models", h.getModelStats)
		adminGroup.GET("/stats/users", h.getUserStats)

		// 用户管理
		adminGroup.GET("/users", h.listUsers)
		adminGroup.GET("/users/:id", h.getUser)
		adminGroup.POST("/users/:id/disable", h.disableUser)
		adminGroup.POST("/users/:id/enable", h.enableUser)
		adminGroup.POST("/users/:id/admin", h.setUserAdmin)

		// 会话管理
		adminGroup.GET("/sessions", h.listAllSessions)
		adminGroup.GET("/sessions/:id", h.getSessionDetail)
		adminGroup.DELETE("/sessions/:id", h.deleteSessionAdmin)

		// 在线用户
		adminGroup.GET("/online", h.getOnlineUsers)

		// 操作日志
		adminGroup.GET("/audit-logs", h.getAuditLogs)

		// 告警管理
		adminGroup.GET("/alert-rules", h.listAlertRules)
		adminGroup.POST("/alert-rules", h.createAlertRule)
		adminGroup.PUT("/alert-rules/:id", h.updateAlertRule)
		adminGroup.DELETE("/alert-rules/:id", h.deleteAlertRule)
		adminGroup.GET("/alert-history", h.listAlertHistory)
	}
}

// adminOnlyMiddleware 管理员权限中间件
func (h *AdminHandlers) adminOnlyMiddleware() gin.HandlerFunc {
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

// --- 系统监控 ---

func (h *AdminHandlers) getSystemStatus(c *gin.Context) {
	status := h.monitorService.GetSystemStatus(h.version)
	c.JSON(http.StatusOK, status)
}

func (h *AdminHandlers) getSystemHealth(c *gin.Context) {
	status := h.monitorService.GetSystemStatus(h.version)
	httpStatus := http.StatusOK
	if status.Status == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	} else if status.Status == "degraded" {
		httpStatus = http.StatusOK // 降级但仍可用
	}
	c.JSON(httpStatus, gin.H{
		"status":     status.Status,
		"components": status.Components,
	})
}

// --- 统计分析 ---

func (h *AdminHandlers) getStatsOverview(c *gin.Context) {
	overview := h.statsService.GetOverview()
	c.JSON(http.StatusOK, overview)
}

func (h *AdminHandlers) getHourlyStats(c *gin.Context) {
	stats := h.statsService.GetHourlyStats()
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

func (h *AdminHandlers) getDailyStats(c *gin.Context) {
	stats := h.statsService.GetDailyStats()
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

func (h *AdminHandlers) getProviderStats(c *gin.Context) {
	stats := h.statsService.GetProviderStats()
	c.JSON(http.StatusOK, gin.H{"providers": stats})
}

func (h *AdminHandlers) getModelStats(c *gin.Context) {
	stats := h.statsService.GetModelStats()
	c.JSON(http.StatusOK, gin.H{"models": stats})
}

func (h *AdminHandlers) getUserStats(c *gin.Context) {
	stats := h.statsService.GetUserStats()
	c.JSON(http.StatusOK, gin.H{"users": stats})
}

// --- 用户管理 ---

func (h *AdminHandlers) listUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	search := c.Query("search")
	onlyDisabled := c.Query("disabled") == "true"

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	result := h.userManager.ListUsers(page, pageSize, search, onlyDisabled)
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandlers) getUser(c *gin.Context) {
	userID := c.Param("id")
	user := h.userManager.GetUser(userID)
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 获取用户的统计详情
	userStats := h.statsService.GetUserStatsDetail(userID)

	c.JSON(http.StatusOK, gin.H{
		"user":  user,
		"stats": userStats,
	})
}

func (h *AdminHandlers) disableUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.userManager.DisableUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user disabled"})
}

func (h *AdminHandlers) enableUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.userManager.EnableUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user enabled"})
}

func (h *AdminHandlers) setUserAdmin(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userManager.SetUserAdmin(userID, req.IsAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "admin status updated"})
}

// --- 会话管理 ---

func (h *AdminHandlers) listAllSessions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userIDFilter := c.Query("user_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Get sessions from stats service (which tracks all sessions)
	result := h.statsService.GetAllSessions(page, pageSize, userIDFilter)
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandlers) getSessionDetail(c *gin.Context) {
	sessionID := c.Param("id")

	// Get session detail from stats service
	detail := h.statsService.GetSessionDetail(sessionID)
	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (h *AdminHandlers) deleteSessionAdmin(c *gin.Context) {
	sessionID := c.Param("id")
	adminUserID := c.GetString("user_id")
	adminUsername := c.GetString("username")

	// Delete session via stats service
	if err := h.statsService.DeleteSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log the admin action
	if h.auditService != nil {
		h.auditService.LogAction(
			adminUserID,
			adminUsername,
			"session.delete",
			"session",
			sessionID,
			"success",
			c.ClientIP(),
			nil,
		)
	}

	c.JSON(http.StatusOK, gin.H{"message": "session deleted"})
}

// --- 在线用户 ---

func (h *AdminHandlers) getOnlineUsers(c *gin.Context) {
	activeCount := h.userManager.GetActiveUsersCount()
	onlineCount := h.userManager.GetOnlineUsersCount()

	c.JSON(http.StatusOK, gin.H{
		"online_users": onlineCount,
		"active_users": activeCount, // 24小时内活跃
	})
}

// --- 操作日志 ---

func (h *AdminHandlers) getAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userID := c.Query("user_id")
	action := c.Query("action")
	result := c.Query("result")

	if h.auditService == nil {
		c.JSON(http.StatusOK, gin.H{
			"logs":      []interface{}{},
			"total":     0,
			"page":      page,
			"page_size": pageSize,
		})
		return
	}

	query := admin.AuditLogQuery{
		UserID:   userID,
		Action:   action,
		Result:   result,
		Page:     page,
		PageSize: pageSize,
	}

	// Parse time filters if provided
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := parseTime(startTime); err == nil {
			query.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := parseTime(endTime); err == nil {
			query.EndTime = &t
		}
	}

	response := h.auditService.Query(query)
	c.JSON(http.StatusOK, response)
}

// parseTime parses time string in ISO8601 format
func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// --- 告警管理 ---

func (h *AdminHandlers) listAlertRules(c *gin.Context) {
	if h.alertService == nil {
		c.JSON(http.StatusOK, gin.H{
			"rules": []interface{}{},
			"total": 0,
		})
		return
	}

	response := h.alertService.ListRules()
	c.JSON(http.StatusOK, response)
}

func (h *AdminHandlers) createAlertRule(c *gin.Context) {
	if h.alertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "alert service not available"})
		return
	}

	var rule admin.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.alertService.CreateRule(rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log the admin action
	if h.auditService != nil {
		adminUserID := c.GetString("user_id")
		adminUsername := c.GetString("username")
		h.auditService.LogAction(
			adminUserID,
			adminUsername,
			"alert.rule.create",
			"alert_rule",
			created.ID,
			"success",
			c.ClientIP(),
			map[string]interface{}{"name": rule.Name},
		)
	}

	c.JSON(http.StatusCreated, created)
}

func (h *AdminHandlers) updateAlertRule(c *gin.Context) {
	if h.alertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "alert service not available"})
		return
	}

	ruleID := c.Param("id")

	var updates admin.AlertRule
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.alertService.UpdateRule(ruleID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log the admin action
	if h.auditService != nil {
		adminUserID := c.GetString("user_id")
		adminUsername := c.GetString("username")
		h.auditService.LogAction(
			adminUserID,
			adminUsername,
			"alert.rule.update",
			"alert_rule",
			ruleID,
			"success",
			c.ClientIP(),
			nil,
		)
	}

	c.JSON(http.StatusOK, updated)
}

func (h *AdminHandlers) deleteAlertRule(c *gin.Context) {
	if h.alertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "alert service not available"})
		return
	}

	ruleID := c.Param("id")

	if err := h.alertService.DeleteRule(ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log the admin action
	if h.auditService != nil {
		adminUserID := c.GetString("user_id")
		adminUsername := c.GetString("username")
		h.auditService.LogAction(
			adminUserID,
			adminUsername,
			"alert.rule.delete",
			"alert_rule",
			ruleID,
			"success",
			c.ClientIP(),
			nil,
		)
	}

	c.JSON(http.StatusOK, gin.H{"message": "rule deleted"})
}

func (h *AdminHandlers) listAlertHistory(c *gin.Context) {
	if h.alertService == nil {
		c.JSON(http.StatusOK, gin.H{
			"history": []interface{}{},
			"total":   0,
		})
		return
	}

	severity := c.Query("severity")
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	response := h.alertService.ListHistory(severity, status, limit)
	c.JSON(http.StatusOK, response)
}
