package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/internal/admin"
	"github.com/aspect-code/codeswitch/sync-service/internal/auth"
	"github.com/aspect-code/codeswitch/sync-service/internal/message"
	"github.com/aspect-code/codeswitch/sync-service/internal/presence"
	"github.com/aspect-code/codeswitch/sync-service/internal/session"
	"github.com/aspect-code/codeswitch/sync-service/pkg/models"
	"github.com/gin-gonic/gin"
)

// Server HTTP API 服务器
type Server struct {
	router          *gin.Engine
	authService     *auth.Service
	sessionManager  *session.Manager
	messageHandler  *message.Handler
	presenceTracker *presence.Tracker
	logger          *slog.Logger
	// 管理后台服务
	statsService    *admin.StatsService
	monitorService  *admin.MonitorService
	userManager     *admin.UserManager
	auditService    *admin.AuditService
	alertService    *admin.AlertService
	billingService  *admin.BillingService
	adminHandlers   *AdminHandlers
	billingHandlers *BillingHandlers
	version         string
}

// NewServer 创建 API 服务器
func NewServer(
	authService *auth.Service,
	sessionManager *session.Manager,
	messageHandler *message.Handler,
	presenceTracker *presence.Tracker,
	statsService *admin.StatsService,
	monitorService *admin.MonitorService,
	userManager *admin.UserManager,
	auditService *admin.AuditService,
	alertService *admin.AlertService,
	billingService *admin.BillingService,
	logger *slog.Logger,
	mode string,
	version string,
) *Server {
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	s := &Server{
		router:          gin.New(),
		authService:     authService,
		sessionManager:  sessionManager,
		messageHandler:  messageHandler,
		presenceTracker: presenceTracker,
		statsService:    statsService,
		monitorService:  monitorService,
		userManager:     userManager,
		auditService:    auditService,
		alertService:    alertService,
		billingService:  billingService,
		logger:          logger,
		version:         version,
	}

	// 创建管理后台处理器
	s.adminHandlers = NewAdminHandlers(statsService, monitorService, userManager, auditService, alertService, version)

	// 创建计费处理器
	s.billingHandlers = NewBillingHandlers(billingService, auditService)

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(gin.Recovery())
	s.router.Use(s.loggerMiddleware())
	s.router.Use(s.corsMiddleware())
}

func (s *Server) setupRoutes() {
	// 健康检查
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/ready", s.readyCheck)

	// Prometheus metrics（公开访问）
	s.router.GET("/metrics", s.metricsHandler)

	// API v1
	v1 := s.router.Group("/api/v1")
	{
		// 认证 (无需 Token)
		v1.POST("/auth/login", s.login)
		v1.POST("/auth/refresh", s.refreshToken)

		// 需要认证的路由
		authorized := v1.Group("")
		authorized.Use(s.authMiddleware())
		{
			// 用户
			authorized.GET("/user/me", s.getCurrentUser)
			authorized.POST("/auth/logout", s.logout)

			// 会话
			authorized.GET("/sessions", s.listSessions)
			authorized.POST("/sessions", s.createSession)
			authorized.GET("/sessions/:id", s.getSession)
			authorized.PUT("/sessions/:id", s.updateSession)
			authorized.DELETE("/sessions/:id", s.deleteSession)
			authorized.POST("/sessions/:id/archive", s.archiveSession)

			// 消息
			authorized.GET("/sessions/:id/messages", s.getMessages)
			authorized.POST("/sessions/:id/messages", s.createMessage)
			authorized.DELETE("/sessions/:id/messages/:msgId", s.deleteMessage)

			// 状态
			authorized.POST("/sessions/:id/typing", s.sendTypingEvent)
			authorized.POST("/heartbeat", s.heartbeat)

			// 同步
			authorized.POST("/sync", s.syncData)

			// 在线状态
			authorized.GET("/presence", s.getMyPresence)
			authorized.GET("/presence/:userId", s.getUserPresence)
		}

		// 计费路由
		s.billingHandlers.RegisterBillingRoutes(v1, s.authMiddleware())

		// 管理后台路由（需要管理员权限）
		s.adminHandlers.RegisterAdminRoutes(v1, s.authMiddleware())
	}
}

// Run 启动服务器
func (s *Server) Run(addr string) error {
	s.logger.Info("Starting API server", "addr", addr)
	return s.router.Run(addr)
}

// --- Middleware ---

func (s *Server) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		s.logger.Info("HTTP Request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", time.Since(start),
			"ip", c.ClientIP(),
		)
	}
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := s.authService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("device_id", claims.DeviceID)
		c.Set("is_admin", claims.IsAdmin)
		c.Next()
	}
}

// --- Handlers ---

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) readyCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (s *Server) metricsHandler(c *gin.Context) {
	metrics := s.adminHandlers.metricsExporter.Export()
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(metrics))
}

func (s *Server) login(c *gin.Context) {
	var req models.AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.authService.Login(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) refreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) logout(c *gin.Context) {
	userID := c.GetString("user_id")
	deviceID := c.GetString("device_id")
	s.authService.Logout(userID, deviceID)
	s.presenceTracker.SetOffline(userID, deviceID)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (s *Server) getCurrentUser(c *gin.Context) {
	userID := c.GetString("user_id")
	user, ok := s.authService.GetUser(userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) listSessions(c *gin.Context) {
	userID := c.GetString("user_id")
	sessions := s.sessionManager.GetUserSessions(userID)
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (s *Server) createSession(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := s.sessionManager.CreateSession(c.Request.Context(), userID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

func (s *Server) getSession(c *gin.Context) {
	sessionID := c.Param("id")
	session, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (s *Server) updateSession(c *gin.Context) {
	sessionID := c.Param("id")
	session, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Title    string `json:"title"`
		IsPinned *bool  `json:"is_pinned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != "" {
		session.Title = req.Title
	}
	if req.IsPinned != nil {
		session.IsPinned = *req.IsPinned
	}

	if err := s.sessionManager.UpdateSession(c.Request.Context(), session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (s *Server) deleteSession(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.Param("id")

	if err := s.sessionManager.DeleteSession(c.Request.Context(), userID, sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (s *Server) archiveSession(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.Param("id")

	if err := s.sessionManager.ArchiveSession(c.Request.Context(), userID, sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "archived"})
}

func (s *Server) getMessages(c *gin.Context) {
	sessionID := c.Param("id")
	limit := 100
	offset := 0

	messages, err := s.messageHandler.GetMessages(sessionID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func (s *Server) createMessage(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.Param("id")

	var req struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg := &models.Message{
		SessionID: sessionID,
		UserID:    userID,
		Role:      req.Role,
		Content:   req.Content,
	}

	if err := s.messageHandler.CreateMessage(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, msg)
}

func (s *Server) deleteMessage(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.Param("id")
	msgID := c.Param("msgId")

	if err := s.messageHandler.DeleteMessage(c.Request.Context(), userID, sessionID, msgID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (s *Server) sendTypingEvent(c *gin.Context) {
	userID := c.GetString("user_id")
	deviceID := c.GetString("device_id")
	sessionID := c.Param("id")

	var req struct {
		IsTyping bool `json:"is_typing"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.sessionManager.SendTypingEvent(c.Request.Context(), userID, sessionID, deviceID, req.IsTyping); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (s *Server) heartbeat(c *gin.Context) {
	userID := c.GetString("user_id")
	deviceID := c.GetString("device_id")

	var req struct {
		DeviceType    string `json:"device_type"`
		ClientVersion string `json:"client_version"`
	}
	c.ShouldBindJSON(&req)

	if err := s.presenceTracker.Heartbeat(c.Request.Context(), userID, deviceID, req.DeviceType, req.ClientVersion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "ok",
		"server_time": time.Now().Unix(),
	})
}

func (s *Server) syncData(c *gin.Context) {
	userID := c.GetString("user_id")

	var req models.SyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userID

	sessions := s.sessionManager.GetUserSessions(userID)

	// 收集所有消息
	var allMessages []models.Message
	for _, sess := range sessions {
		msgs, _ := s.messageHandler.GetMessagesAfterID(sess.ID, req.LastMsgID, 100)
		for _, m := range msgs {
			allMessages = append(allMessages, *m)
		}
	}

	// 转换 sessions
	var sessionsResult []models.Session
	for _, s := range sessions {
		sessionsResult = append(sessionsResult, *s)
	}

	resp := &models.SyncResponse{
		Sessions:   sessionsResult,
		Messages:   allMessages,
		ServerTime: time.Now().Unix(),
		HasMore:    false,
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) getMyPresence(c *gin.Context) {
	userID := c.GetString("user_id")
	presence := s.presenceTracker.GetUserPresence(userID)
	c.JSON(http.StatusOK, gin.H{"devices": presence})
}

func (s *Server) getUserPresence(c *gin.Context) {
	targetUserID := c.Param("userId")
	presence := s.presenceTracker.GetUserPresence(targetUserID)
	c.JSON(http.StatusOK, gin.H{
		"user_id": targetUserID,
		"online":  len(presence) > 0,
		"devices": presence,
	})
}
