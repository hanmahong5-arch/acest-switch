package services

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/daodao97/xgo/xdb"
	"github.com/daodao97/xgo/xlog"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	_ "modernc.org/sqlite"
)

// Ailurus PaaS 版本号
const AppVersion = "v0.1.9"

type ProviderRelayService struct {
	providerService *ProviderService
	pricingService  *modelpricing.Service
	server          *http.Server
	addr            string
	startTime       time.Time // Ailurus PaaS: 服务启动时间（用于 metrics）
	// 轮询计数器，用于 Round-Robin 负载均衡
	rrCounter uint64
	// 轮询模式开关：true=Round-Robin，false=按优先级顺序
	roundRobinEnabled uint32
	// 日志写入队列，避免并发写入竞争
	logWriteQueue chan *ReqeustLog
	// Body 日志开关：控制是否存储请求/响应体
	bodyLogEnabled uint32
	// Body 日志写入队列，独立于主日志队列
	bodyLogQueue chan *RequestLogBody
	// 同步集成：用于多端同步功能
	syncIntegration *SyncIntegration

	// NEW-API 统一网关配置
	newAPIEnabled uint32 // 原子操作：是否启用 new-api 模式
	newAPIURL     string // new-api 服务地址
	newAPIToken   string // new-api API Token

	// Lurus-API 集成 (替代 Casdoor + Lago)
	lurusIntegration *LurusIntegration

	// Proxy Control (Phase 3)
	proxyController *ProxyController

	// Config Recovery (Phase 4)
	configRecovery *ConfigRecovery
}

func NewProviderRelayService(providerService *ProviderService, addr string) *ProviderRelayService {
	if addr == "" {
		addr = ":18100"
	}

	home, _ := os.UserHomeDir()
	const sqliteOptions = "?cache=shared&mode=rwc&_busy_timeout=5000&_journal_mode=WAL"

	if err := xdb.Inits([]xdb.Config{
		{
			Name:   "default",
			Driver: "sqlite",
			DSN:    filepath.Join(home, ".code-switch", "app.db"+sqliteOptions),
			// 使用写入队列后，减少连接池避免资源浪费
			// 1 个写入线程 + 几个并发查询足够
			MaxOpenConn: 5,
			MaxIdleConn: 2,
		},
	}); err != nil {
		fmt.Printf("初始化数据库失败: %v\n", err)
	} else if err := ensureRequestLogTable(); err != nil {
		fmt.Printf("初始化 request_log 表失败: %v\n", err)
	}

	// 初始化价格计算服务
	pricingSvc, err := modelpricing.DefaultService()
	if err != nil {
		fmt.Printf("初始化价格服务失败: %v\n", err)
	}

	// 初始化代理控制器 (Phase 3)
	var pc *ProxyController
	if db, dbErr := xdb.DB("default"); dbErr == nil && db != nil {
		pc, err = NewProxyController(db)
		if err != nil {
			fmt.Printf("[ProxyControl] 初始化失败: %v\n", err)
		}
	}

	// 初始化配置恢复服务 (Phase 4)
	var cr *ConfigRecovery
	if db, dbErr := xdb.DB("default"); dbErr == nil && db != nil {
		cr = NewConfigRecovery(db, filepath.Join(home, ".code-switch"))
		fmt.Printf("[Recovery] 配置恢复服务已初始化\n")
	}

	prs := &ProviderRelayService{
		providerService:  providerService,
		pricingService:   pricingSvc,
		addr:             addr,
		startTime:        time.Now(),
		logWriteQueue:    make(chan *ReqeustLog, 1000),    // 缓冲 1000 条日志
		bodyLogQueue:     make(chan *RequestLogBody, 500), // Body 日志队列，较小缓冲
		lurusIntegration: NewLurusIntegration(),
		proxyController:  pc,
		configRecovery:   cr,
	}

	// 启动单个 goroutine 处理所有日志写入，避免写锁竞争
	go prs.processLogWriteQueue()

	// 启动 Body 日志写入队列处理
	go prs.processBodyLogQueue()

	// 启动过期 Body 日志清理任务
	go prs.startBodyLogCleanupTask()

	// 初始化 Lurus-API 集成 (从配置文件)
	if err := prs.lurusIntegration.Initialize(); err != nil {
		fmt.Printf("[Lurus] 初始化失败: %v\n", err)
	}

	return prs
}

func (prs *ProviderRelayService) Start() error {
	// 启动前验证配置
	if warnings := prs.validateConfig(); len(warnings) > 0 {
		fmt.Println("======== Provider 配置验证警告 ========")
		for _, warn := range warnings {
			fmt.Printf("⚠️  %s\n", warn)
		}
		fmt.Println("========================================")
	}

	router := gin.Default()
	prs.registerRoutes(router)

	prs.server = &http.Server{
		Addr:    prs.addr,
		Handler: router,
	}

	fmt.Printf("provider relay server listening on %s\n", prs.addr)

	go func() {
		if err := prs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("provider relay server error: %v\n", err)
		}
	}()
	return nil
}

// validateConfig 验证所有 provider 的配置
// 返回警告列表（非阻塞性错误）
func (prs *ProviderRelayService) validateConfig() []string {
	warnings := make([]string, 0)

	for _, kind := range []string{"claude", "codex", "gemini-cli", "picoclaw"} {
		providers, err := prs.providerService.LoadProviders(kind)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("[%s] 加载配置失败: %v", kind, err))
			continue
		}

		enabledCount := 0
		for _, p := range providers {
			if !p.Enabled {
				continue
			}
			enabledCount++

			// 验证每个启用的 provider
			if errs := p.ValidateConfiguration(); len(errs) > 0 {
				for _, errMsg := range errs {
					warnings = append(warnings, fmt.Sprintf("[%s/%s] %s", kind, p.Name, errMsg))
				}
			}

			// 检查是否配置了模型白名单或映射
			if (p.SupportedModels == nil || len(p.SupportedModels) == 0) &&
				(p.ModelMapping == nil || len(p.ModelMapping) == 0) {
				warnings = append(warnings, fmt.Sprintf(
					"[%s/%s] 未配置 supportedModels 或 modelMapping，将假设支持所有模型（可能导致降级失败）",
					kind, p.Name))
			}
		}

		if enabledCount == 0 {
			warnings = append(warnings, fmt.Sprintf("[%s] 没有启用的 provider", kind))
		}
	}

	return warnings
}

func (prs *ProviderRelayService) Stop() error {
	if prs.server == nil {
		return nil
	}
	// 关闭 Lurus 服务
	if prs.lurusIntegration != nil {
		prs.lurusIntegration.Shutdown()
	}
	// 关闭日志队列
	close(prs.logWriteQueue)
	close(prs.bodyLogQueue)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return prs.server.Shutdown(ctx)
}

// ============================================================
// Lurus Integration Methods
// ============================================================

// GetLurusIntegration returns the Lurus integration instance
func (prs *ProviderRelayService) GetLurusIntegration() *LurusIntegration {
	return prs.lurusIntegration
}

// IsLurusEnabled returns whether Lurus integration is enabled
func (prs *ProviderRelayService) IsLurusEnabled() bool {
	return prs.lurusIntegration != nil && prs.lurusIntegration.IsEnabled()
}

// SetLurusEnabled enables or disables Lurus integration
func (prs *ProviderRelayService) SetLurusEnabled(enabled bool) {
	if prs.lurusIntegration != nil {
		prs.lurusIntegration.SetEnabled(enabled)
	}
}

// processLogWriteQueue 处理日志写入队列，批量写入降低锁占用时间
func (prs *ProviderRelayService) processLogWriteQueue() {
	const batchSize = 10
	const batchTimeout = 100 * time.Millisecond

	fmt.Printf("[Ailurus PaaS] 日志写入队列已启动 (batch_size=%d, timeout=%v)\n", batchSize, batchTimeout)

	batch := make([]*ReqeustLog, 0, batchSize)
	ticker := time.NewTicker(batchTimeout)
	defer ticker.Stop()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		// 批量写入，减少锁占用次数
		fmt.Printf("[Ailurus PaaS] 批量写入 %d 条日志到数据库\n", len(batch))
		successCount := 0
		for _, log := range batch {
			if _, err := xdb.New("request_log").Insert(xdb.Record{
				"trace_id":            log.TraceID,
				"request_id":          log.RequestID,
				"platform":            log.Platform,
				"model":               log.Model,
				"provider":            log.Provider,
				"http_code":           log.HttpCode,
				"input_tokens":        log.InputTokens,
				"output_tokens":       log.OutputTokens,
				"cache_create_tokens": log.CacheCreateTokens,
				"cache_read_tokens":   log.CacheReadTokens,
				"reasoning_tokens":    log.ReasoningTokens,
				"is_stream":           boolToInt(log.IsStream),
				"duration_sec":        log.DurationSec,
				"user_agent":          log.UserAgent,
				"client_ip":           log.ClientIP,
				"user_id":             log.UserID,
				"request_method":      log.RequestMethod,
				"request_path":        log.RequestPath,
				"error_type":          log.ErrorType,
				"error_message":       log.ErrorMessage,
				"provider_error_code": log.ProviderErrorCode,
				"input_cost":          log.InputCost,
				"output_cost":         log.OutputCost,
				"cache_create_cost":   log.CacheCreateCost,
				"cache_read_cost":     log.CacheReadCost,
				"ephemeral_5m_cost":   log.Ephemeral5mCost,
				"ephemeral_1h_cost":   log.Ephemeral1hCost,
				"total_cost":          log.TotalCost,
			}); err != nil {
				fmt.Printf("[Ailurus PaaS] 写入 request_log 失败 (trace_id=%s): %v\n", log.TraceID, err)
			} else {
				successCount++
			}
		}
		fmt.Printf("[Ailurus PaaS] 批量写入完成：%d/%d 成功\n", successCount, len(batch))
		batch = batch[:0]
	}

	for {
		select {
		case log, ok := <-prs.logWriteQueue:
			if !ok {
				// 队列关闭，刷新剩余批次
				fmt.Printf("[Ailurus PaaS] 日志写入队列关闭，刷新剩余 %d 条日志\n", len(batch))
				flushBatch()
				fmt.Printf("[Ailurus PaaS] 日志写入队列已停止\n")
				return
			}
			batch = append(batch, log)
			if len(batch) >= batchSize {
				flushBatch()
			}
		case <-ticker.C:
			// 定时刷新，避免日志积压
			flushBatch()
		}
	}
}

func (prs *ProviderRelayService) Addr() string {
	return prs.addr
}

// IsRoundRobinEnabled 获取轮询模式开关状态
func (prs *ProviderRelayService) IsRoundRobinEnabled() bool {
	return atomic.LoadUint32(&prs.roundRobinEnabled) == 1
}

// SetRoundRobinEnabled 设置轮询模式开关
func (prs *ProviderRelayService) SetRoundRobinEnabled(enabled bool) {
	var val uint32 = 0
	if enabled {
		val = 1
	}
	atomic.StoreUint32(&prs.roundRobinEnabled, val)
	mode := "优先级顺序"
	if enabled {
		mode = "Round-Robin 轮询"
	}
	fmt.Printf("[INFO] 负载均衡模式已切换为：%s\n", mode)
}

// IsBodyLogEnabled 获取 Body 日志开关状态
func (prs *ProviderRelayService) IsBodyLogEnabled() bool {
	return atomic.LoadUint32(&prs.bodyLogEnabled) == 1
}

// SetSyncIntegration 设置同步集成实例
func (prs *ProviderRelayService) SetSyncIntegration(si *SyncIntegration) {
	prs.syncIntegration = si
}

// SetBodyLogEnabled 设置 Body 日志开关
func (prs *ProviderRelayService) SetBodyLogEnabled(enabled bool) {
	var val uint32 = 0
	if enabled {
		val = 1
	}
	atomic.StoreUint32(&prs.bodyLogEnabled, val)
	status := "关闭"
	if enabled {
		status = "开启"
	}
	fmt.Printf("[Ailurus PaaS] 上下行日志已%s\n", status)
}

// IsNewAPIEnabled 获取 new-api 模式开关状态
func (prs *ProviderRelayService) IsNewAPIEnabled() bool {
	return atomic.LoadUint32(&prs.newAPIEnabled) == 1
}

// SetNewAPIEnabled 设置 new-api 模式开关
func (prs *ProviderRelayService) SetNewAPIEnabled(enabled bool) {
	var val uint32 = 0
	if enabled {
		val = 1
	}
	atomic.StoreUint32(&prs.newAPIEnabled, val)
	status := "关闭"
	if enabled {
		status = "开启"
	}
	fmt.Printf("[Ailurus PaaS] NEW-API 统一网关模式已%s\n", status)
}

// SetNewAPIConfig 设置 new-api 配置
func (prs *ProviderRelayService) SetNewAPIConfig(url, token string) {
	prs.newAPIURL = url
	prs.newAPIToken = token
	fmt.Printf("[Ailurus PaaS] NEW-API 配置已更新: URL=%s, Token=%s***\n", url, token[:min(10, len(token))])
}

// GetNewAPIConfig 获取 new-api 配置
func (prs *ProviderRelayService) GetNewAPIConfig() (url, token string, enabled bool) {
	return prs.newAPIURL, prs.newAPIToken, prs.IsNewAPIEnabled()
}

// GetProxyConfigs returns proxy control configurations and statistics for all applications
func (prs *ProviderRelayService) GetProxyConfigs() (map[string]interface{}, error) {
	if prs.proxyController == nil {
		return nil, fmt.Errorf("proxy controller not initialized")
	}

	configs, err := prs.proxyController.GetAllConfigs()
	if err != nil {
		return nil, err
	}

	stats, err := prs.proxyController.GetStats()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"configs": configs,
		"stats":   stats,
	}, nil
}

// GetProxyStats returns proxy statistics for all applications
func (prs *ProviderRelayService) GetProxyStats() (map[string]ProxyControlStats, error) {
	if prs.proxyController == nil {
		return nil, fmt.Errorf("proxy controller not initialized")
	}

	return prs.proxyController.GetStats()
}

// ToggleProxy toggles proxy enable/disable for an application
func (prs *ProviderRelayService) ToggleProxy(appName string, enabled bool) error {
	if prs.proxyController == nil {
		return fmt.Errorf("proxy controller not initialized")
	}

	return prs.proxyController.ToggleProxy(appName, enabled)
}

// GetBackupHistory returns backup history for a specific type (Phase 4)
func (prs *ProviderRelayService) GetBackupHistory(backupType string, limit int) ([]BackupRecord, error) {
	if prs.configRecovery == nil {
		return nil, fmt.Errorf("config recovery not initialized")
	}

	return prs.configRecovery.GetBackupHistory(BackupType(backupType), limit)
}

// RestoreFromBackup restores configuration from a specific backup ID (Phase 4)
func (prs *ProviderRelayService) RestoreFromBackup(backupID int) error {
	if prs.configRecovery == nil {
		return fmt.Errorf("config recovery not initialized")
	}

	return prs.configRecovery.RestoreFromBackup(backupID)
}

// CleanupOldBackups removes old backup records (keep last N per type) (Phase 4)
func (prs *ProviderRelayService) CleanupOldBackups(keepCount int) error {
	if prs.configRecovery == nil {
		return fmt.Errorf("config recovery not initialized")
	}

	return prs.configRecovery.CleanupOldBackups(keepCount)
}

// processBodyLogQueue 处理 Body 日志写入队列
func (prs *ProviderRelayService) processBodyLogQueue() {
	fmt.Printf("[Ailurus PaaS] Body 日志写入队列已启动\n")

	for bodyLog := range prs.bodyLogQueue {
		if _, err := xdb.New("request_log_body").Insert(xdb.Record{
			"trace_id":        bodyLog.TraceID,
			"request_body":    bodyLog.RequestBody,
			"response_body":   bodyLog.ResponseBody,
			"body_size_bytes": bodyLog.BodySizeBytes,
			"created_at":      bodyLog.CreatedAt.Format("2006-01-02 15:04:05"),
			"expires_at":      bodyLog.ExpiresAt.Format("2006-01-02 15:04:05"),
		}); err != nil {
			fmt.Printf("[Ailurus PaaS] 写入 request_log_body 失败 (trace_id=%s): %v\n", bodyLog.TraceID, err)
		}
	}

	fmt.Printf("[Ailurus PaaS] Body 日志写入队列已停止\n")
}

// startBodyLogCleanupTask 启动过期 Body 日志清理任务
func (prs *ProviderRelayService) startBodyLogCleanupTask() {
	// 启动时先清理一次
	prs.cleanupExpiredBodyLogs()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		prs.cleanupExpiredBodyLogs()
	}
}

// cleanupExpiredBodyLogs 清理过期的 Body 日志
func (prs *ProviderRelayService) cleanupExpiredBodyLogs() {
	db, err := xdb.DB("default")
	if err != nil {
		fmt.Printf("[Ailurus PaaS] 获取数据库连接失败: %v\n", err)
		return
	}

	result, err := db.Exec("DELETE FROM request_log_body WHERE expires_at < datetime('now')")
	if err != nil {
		fmt.Printf("[Ailurus PaaS] 清理过期 Body 日志失败: %v\n", err)
		return
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		fmt.Printf("[Ailurus PaaS] 已清理 %d 条过期 Body 日志\n", deleted)
	}
}

// countEnabledProviders 统计启用的供应商数量（Ailurus PaaS Metrics）
func (prs *ProviderRelayService) countEnabledProviders(platform string) int {
	providers, err := prs.providerService.LoadProviders(platform)
	if err != nil {
		return 0
	}
	count := 0
	for _, p := range providers {
		if p.Enabled && p.APIURL != "" && p.APIKey != "" {
			count++
		}
	}
	return count
}

func (prs *ProviderRelayService) registerRoutes(router gin.IRouter) {
	// Ailurus PaaS 健康检查端点（增强版）
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "Ailurus PaaS Gateway",
			"version":   AppVersion,
			"timestamp": time.Now().Unix(),
		})
	})

	// Readiness 检查（检查供应商可用性）
	router.GET("/readiness", func(c *gin.Context) {
		claudeProviders, _ := prs.providerService.LoadProviders("claude")
		codexProviders, _ := prs.providerService.LoadProviders("codex")
		geminiProviders, _ := prs.providerService.LoadProviders("gemini-cli")
		picoClawProviders, _ := prs.providerService.LoadProviders("picoclaw")

		claudeReady := 0
		codexReady := 0
		geminiReady := 0
		picoClawReady := 0
		for _, p := range claudeProviders {
			if p.Enabled && p.APIURL != "" && p.APIKey != "" {
				claudeReady++
			}
		}
		for _, p := range codexProviders {
			if p.Enabled && p.APIURL != "" && p.APIKey != "" {
				codexReady++
			}
		}
		for _, p := range geminiProviders {
			if p.Enabled && p.APIURL != "" && p.APIKey != "" {
				geminiReady++
			}
		}
		for _, p := range picoClawProviders {
			if p.Enabled && p.APIURL != "" && p.APIKey != "" {
				picoClawReady++
			}
		}

		ready := claudeReady > 0 || codexReady > 0 || geminiReady > 0 || picoClawReady > 0
		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"ready":              ready,
			"claude_providers":   claudeReady,
			"codex_providers":    codexReady,
			"picoclaw_providers": picoClawReady,
			"timestamp":          time.Now().Unix(),
		})
	})

	// Prometheus Metrics 导出端点
	router.GET("/metrics", func(c *gin.Context) {
		// 简化版 Prometheus 格式 metrics
		// 实际生产环境可集成 prometheus/client_golang
		metrics := fmt.Sprintf(`# HELP ailurus_paas_info Ailurus PaaS Gateway info
# TYPE ailurus_paas_info gauge
ailurus_paas_info{version="%s",service="gateway"} 1

# HELP ailurus_paas_uptime_seconds Gateway uptime in seconds
# TYPE ailurus_paas_uptime_seconds counter
ailurus_paas_uptime_seconds %.0f

# HELP ailurus_paas_providers_total Number of configured providers
# TYPE ailurus_paas_providers_total gauge
ailurus_paas_providers_total{platform="claude",status="enabled"} %d
ailurus_paas_providers_total{platform="codex",status="enabled"} %d
ailurus_paas_providers_total{platform="picoclaw",status="enabled"} %d
`, AppVersion, time.Since(prs.startTime).Seconds(), prs.countEnabledProviders("claude"), prs.countEnabledProviders("codex"), prs.countEnabledProviders("picoclaw"))

		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(metrics))
	})

	// 设备注册端点（Codex CLI 等客户端会调用，返回空成功即可）
	router.POST("/v1/device/register", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Device registered (proxy mode)",
		})
	})
	// LLM 代理路由
	// 如果启用了 Lurus-API 配额检查，使用配额验证中间件包装
	if prs.lurusIntegration != nil && prs.lurusIntegration.IsEnabled() {
		router.POST("/v1/messages", prs.lurusIntegration.WrapWithQuotaCheck(prs.proxyHandler("claude", "/v1/messages")))
		router.POST("/responses", prs.lurusIntegration.WrapWithQuotaCheck(prs.proxyHandler("codex", "/responses")))
		router.POST("/v1/chat/completions", prs.lurusIntegration.WrapWithQuotaCheck(prs.proxyHandler("codex", "/v1/chat/completions")))
		router.POST("/chat/completions", prs.lurusIntegration.WrapWithQuotaCheck(prs.proxyHandler("codex", "/chat/completions")))
		router.POST("/v1beta/models/*modelAction", prs.lurusIntegration.WrapWithQuotaCheck(prs.geminiNativeHandler()))
		// PicoClaw routes (OpenAI-compatible via /pc/ prefix)
		router.POST("/pc/v1/chat/completions", prs.lurusIntegration.WrapWithQuotaCheck(prs.proxyHandler("picoclaw", "/v1/chat/completions")))
		router.POST("/pc/chat/completions", prs.lurusIntegration.WrapWithQuotaCheck(prs.proxyHandler("picoclaw", "/chat/completions")))
	} else {
		// 无配额检查模式：直接代理
		router.POST("/v1/messages", prs.proxyHandler("claude", "/v1/messages"))
		router.POST("/responses", prs.proxyHandler("codex", "/responses"))
		router.POST("/v1/chat/completions", prs.proxyHandler("codex", "/v1/chat/completions"))
		router.POST("/chat/completions", prs.proxyHandler("codex", "/chat/completions"))
		router.POST("/v1beta/models/*modelAction", prs.geminiNativeHandler())
		// PicoClaw routes (OpenAI-compatible via /pc/ prefix)
		router.POST("/pc/v1/chat/completions", prs.proxyHandler("picoclaw", "/v1/chat/completions"))
		router.POST("/pc/chat/completions", prs.proxyHandler("picoclaw", "/chat/completions"))
	}

	// 注册 Lurus-API 相关路由 (认证、配额、订阅)
	if prs.lurusIntegration != nil {
		prs.lurusIntegration.RegisterLurusRoutes(router)
	}
}

func (prs *ProviderRelayService) proxyHandler(kind string, endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var bodyBytes []byte
		if c.Request.Body != nil {
			data, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
			bodyBytes = data
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// 同步集成：发布用户消息事件
		if prs.syncIntegration != nil {
			prs.syncIntegration.OnUserMessage(c, bodyBytes)
		}

		isStream := gjson.GetBytes(bodyBytes, "stream").Bool()
		requestedModel := gjson.GetBytes(bodyBytes, "model").String()

		// 如果未指定模型，记录警告但不拦截
		if requestedModel == "" {
			fmt.Printf("[WARN] 请求未指定模型名，无法执行模型智能降级\n")
		}

		// NEW-API 统一网关模式：直接转发到 new-api
		if prs.IsNewAPIEnabled() && prs.newAPIURL != "" && prs.newAPIToken != "" {
			fmt.Printf("[Ailurus PaaS] NEW-API 模式: 转发到 %s (model=%s, stream=%v)\n",
				prs.newAPIURL, requestedModel, isStream)

			success, err := prs.forwardToNewAPI(c, kind, endpoint, bodyBytes, isStream, requestedModel)
			if success {
				return
			}

			// NEW-API 失败，尝试 fallback 到本地 provider
			fmt.Printf("[WARN] NEW-API 请求失败 (%v), 尝试 fallback 到本地 provider\n", err)
		}

		providers, err := prs.providerService.LoadProviders(kind)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load providers"})
			return
		}

		active := make([]Provider, 0, len(providers))
		skippedCount := 0
		for _, provider := range providers {
			// 基础过滤：enabled、URL、APIKey
			if !provider.Enabled || provider.APIURL == "" || provider.APIKey == "" {
				continue
			}

			// 配置验证：失败则自动跳过
			if errs := provider.ValidateConfiguration(); len(errs) > 0 {
				fmt.Printf("[WARN] Provider %s 配置验证失败，已自动跳过: %v\n", provider.Name, errs)
				skippedCount++
				continue
			}

			// 核心过滤：只保留支持请求模型的 provider
			if requestedModel != "" && !provider.IsModelSupported(requestedModel) {
				fmt.Printf("[INFO] Provider %s 不支持模型 %s，已跳过\n", provider.Name, requestedModel)
				skippedCount++
				continue
			}

			active = append(active, provider)
		}

		if len(active) == 0 {
			if requestedModel != "" {
				c.JSON(http.StatusNotFound, gin.H{
					"error": fmt.Sprintf("没有可用的 provider 支持模型 '%s'（已跳过 %d 个不兼容的 provider）", requestedModel, skippedCount),
				})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": "no providers available"})
			}
			return
		}

		// 按优先级排序（Level 小的优先）
		sort.SliceStable(active, func(i, j int) bool {
			levelI := active[i].Level
			levelJ := active[j].Level
			// Level 为 0 时视为默认值 1
			if levelI == 0 {
				levelI = 1
			}
			if levelJ == 0 {
				levelJ = 1
			}
			return levelI < levelJ
		})

		fmt.Printf("[INFO] 找到 %d 个可用的 provider（已过滤 %d 个）：", len(active), skippedCount)
		for _, p := range active {
			level := p.Level
			if level == 0 {
				level = 1
			}
			fmt.Printf("%s(L%d) ", p.Name, level)
		}
		fmt.Println()

		// 根据轮询模式决定起始索引
		var startIdx int
		if prs.IsRoundRobinEnabled() {
			// Round-Robin 模式：使用计数器轮询
			startIdx = int(atomic.AddUint64(&prs.rrCounter, 1)-1) % len(active)
			fmt.Printf("[INFO] Round-Robin 模式：从第 %d 个 provider 开始（%s）\n", startIdx+1, active[startIdx].Name)
		} else {
			// 优先级模式：从第一个（优先级最高的）开始
			startIdx = 0
			fmt.Printf("[INFO] 优先级模式：从优先级最高的 provider 开始（%s）\n", active[startIdx].Name)
		}

		query := flattenQuery(c.Request.URL.Query())
		clientHeaders := cloneHeaders(c.Request.Header)

		var lastErr error
		attemptCount := 0
		// 从 startIdx 开始轮询，遍历所有 provider
		for j := 0; j < len(active); j++ {
			i := (startIdx + j) % len(active)
			provider := active[i]
			attemptCount++

			effectiveModel := provider.GetEffectiveModel(requestedModel)

			currentBodyBytes := bodyBytes
			if effectiveModel != requestedModel && requestedModel != "" {
				fmt.Printf("[INFO]   Provider %s 映射模型: %s -> %s\n", provider.Name, requestedModel, effectiveModel)

				modifiedBody, err := ReplaceModelInRequestBody(bodyBytes, effectiveModel)
				if err != nil {
					fmt.Printf("[ERROR]   替换模型名失败: %v\n", err)
					lastErr = err
					continue
				}
				currentBodyBytes = modifiedBody
			}

			fmt.Printf("[INFO]   [%d/%d] Provider: %s | Model: %s\n",
				j+1, len(active), provider.Name, effectiveModel)

			startTime := time.Now()
			ok, err := prs.forwardRequest(c, kind, provider, endpoint, query, clientHeaders, currentBodyBytes, isStream, effectiveModel)
			duration := time.Since(startTime)

			if ok {
				fmt.Printf("[INFO]   ✓ 成功: %s | 耗时: %.2fs\n", provider.Name, duration.Seconds())
				return
			}

			errorMsg := "未知错误"
			if err != nil {
				errorMsg = err.Error()
			}
			fmt.Printf("[WARN]   ✗ 失败: %s | 错误: %s | 耗时: %.2fs\n",
				provider.Name, errorMsg, duration.Seconds())
			lastErr = err
		}

		message := fmt.Sprintf("所有 %d 个 provider 均失败（共尝试 %d 次）", len(active), attemptCount)
		if lastErr != nil {
			message = fmt.Sprintf("%s: %s", message, lastErr.Error())
		}
		xlog.Error("all is error")
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
	}
}

func (prs *ProviderRelayService) forwardRequest(
	c *gin.Context,
	kind string,
	provider Provider,
	endpoint string,
	query map[string]string,
	clientHeaders map[string]string,
	bodyBytes []byte,
	isStream bool,
	model string,
) (bool, error) {
	// Google Gemini 特殊处理：使用原生API而不是OpenAI兼容端点
	isGemini := strings.Contains(strings.ToLower(provider.APIURL), "generativelanguage.googleapis.com")

	var targetURL string
	if isGemini {
		// 使用Gemini原生API端点
		// https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent
		baseURL := "https://generativelanguage.googleapis.com/v1beta"
		if isStream {
			targetURL = fmt.Sprintf("%s/models/%s:streamGenerateContent", baseURL, model)
		} else {
			targetURL = fmt.Sprintf("%s/models/%s:generateContent", baseURL, model)
		}
		// Gemini原生API使用URL参数传递API key
		if query == nil {
			query = make(map[string]string)
		}
		query["key"] = provider.APIKey
		fmt.Printf("[Ailurus PaaS] Gemini使用原生API: %s\n", targetURL)
	} else {
		targetURL = joinURL(provider.APIURL, endpoint)
	}

	headers := cloneMap(clientHeaders)
	needsStreamConversion := false
	actualStream := isStream

	// Gemini 请求格式转换
	// 如果 kind == "gemini-cli"，说明请求已经是 Gemini 原生格式，无需转换
	if isGemini && kind != "gemini-cli" {
		// 使用Gemini原生格式
		converter := &GeminiConverter{}
		var openAIReq map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
			return false, fmt.Errorf("解析OpenAI请求失败: %w", err)
		}

		geminiReq, err := converter.ConvertOpenAIToGemini(openAIReq)
		if err != nil {
			return false, fmt.Errorf("转换为Gemini格式失败: %w", err)
		}

		geminiBytes, err := json.Marshal(geminiReq)
		if err != nil {
			return false, fmt.Errorf("序列化Gemini请求失败: %w", err)
		}
		bodyBytes = geminiBytes

		// Gemini原生API不使用stream参数，流式由endpoint决定
		// 暂时禁用流式以获取complete response和usage
		if isStream {
			targetURL = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
			needsStreamConversion = true
			actualStream = false
		}
	}

	// Authorization header设置
	if !isGemini {
		// 其他provider使用Bearer token
		headers["Authorization"] = fmt.Sprintf("Bearer %s", provider.APIKey)
	}
	// Gemini使用URL参数，不需要Authorization header

	if _, ok := headers["Accept"]; !ok {
		headers["Accept"] = "application/json"
	}

	// Ailurus PaaS 增强日志：自动记录追踪信息
	traceID := generateTraceID()

	// 同步集成：发布请求开始事件
	if prs.syncIntegration != nil {
		prs.syncIntegration.OnRequestStart(c, kind, model, provider.Name, isStream, traceID)
	}

	// Body 日志捕获（仅在开关开启时）
	shouldLogBody := prs.IsBodyLogEnabled()
	var responseBuffer bytes.Buffer

	requestLog := &ReqeustLog{
		TraceID:       traceID,
		RequestID:     c.GetHeader("X-Request-ID"), // 兼容客户端传入的请求 ID
		Platform:      kind,
		Provider:      provider.Name,
		Model:         model,
		IsStream:      isStream, // 记录客户端的原始流式请求意图
		UserAgent:     c.GetHeader("User-Agent"),
		ClientIP:      getClientIP(c),
		UserID:        c.GetHeader("X-User-ID"), // 支持多租户场景
		RequestMethod: c.Request.Method,
		RequestPath:   c.Request.URL.Path,
	}

	// 将 Trace ID 添加到响应头，方便客户端关联日志
	c.Header("X-Trace-ID", traceID)

	start := time.Now()
	defer func() {
		requestLog.DurationSec = time.Since(start).Seconds()

		// 根据 HTTP 状态码填充错误信息
		if requestLog.HttpCode >= 400 {
			requestLog.ErrorType = classifyHTTPError(requestLog.HttpCode)
			// 错误消息将在下面的错误处理中填充
		}

		// 计算价格（在插入数据库前）
		if prs.pricingService != nil {
			costBreakdown := prs.pricingService.CalculateCost(requestLog.Model, modelpricing.UsageSnapshot{
				InputTokens:       requestLog.InputTokens,
				OutputTokens:      requestLog.OutputTokens,
				CacheCreateTokens: requestLog.CacheCreateTokens,
				CacheReadTokens:   requestLog.CacheReadTokens,
			})
			requestLog.InputCost = costBreakdown.InputCost
			requestLog.OutputCost = costBreakdown.OutputCost
			requestLog.CacheCreateCost = costBreakdown.CacheCreateCost
			requestLog.CacheReadCost = costBreakdown.CacheReadCost
			requestLog.Ephemeral5mCost = costBreakdown.Ephemeral5mCost
			requestLog.Ephemeral1hCost = costBreakdown.Ephemeral1hCost
			requestLog.TotalCost = costBreakdown.TotalCost
		}

		// 发送到写入队列，由单个 goroutine 顺序处理
		// 使用 select 避免阻塞
		select {
		case prs.logWriteQueue <- requestLog:
			// 成功入队
		default:
			// 队列满，丢弃日志并记录警告
			fmt.Printf("[WARN] 日志队列已满，丢弃日志 (trace_id=%s)\n", requestLog.TraceID)
		}

		// Body 日志：仅在开关开启且有数据时发送
		fmt.Printf("[DEBUG Body Log] shouldLogBody=%v, bodyBytes=%d, responseBuffer=%d, traceID=%s\n",
			shouldLogBody, len(bodyBytes), responseBuffer.Len(), traceID)
		if shouldLogBody && (len(bodyBytes) > 0 || responseBuffer.Len() > 0) {
			bodyLog := &RequestLogBody{
				TraceID:       traceID,
				RequestBody:   string(bodyBytes),
				ResponseBody:  responseBuffer.String(),
				BodySizeBytes: int64(len(bodyBytes) + responseBuffer.Len()),
				CreatedAt:     time.Now(),
				ExpiresAt:     time.Now().Add(7 * 24 * time.Hour), // 7 天过期
			}
			select {
			case prs.bodyLogQueue <- bodyLog:
				fmt.Printf("[DEBUG] Body log queued for trace_id=%s\n", traceID)
			default:
				fmt.Printf("[WARN] Body log queue full, dropped trace_id=%s\n", traceID)
			}
		} else {
			fmt.Printf("[DEBUG] Body log skipped for trace_id=%s\n", traceID)
		}
	}()

	// 创建带超时的 HTTP 客户端
	// 对于流式响应，超时应该更长，因为响应是增量传输的
	timeout := 60 * time.Second
	if isStream {
		timeout = 300 * time.Second // 流式响应 5 分钟超时
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	fmt.Printf("[Ailurus PaaS] 发送请求 (trace_id=%s, provider=%s, model=%s, stream=%v, timeout=%v)\n",
		traceID, provider.Name, model, isStream, timeout)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequest("POST", targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		requestLog.HttpCode = 0
		requestLog.ErrorType = "network_error"
		requestLog.ErrorMessage = err.Error()
		fmt.Printf("[Ailurus PaaS] 创建请求失败 (trace_id=%s): %v\n", traceID, err)
		return false, err
	}

	// 设置请求头
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	// 设置查询参数
	if len(query) > 0 {
		q := httpReq.URL.Query()
		for key, value := range query {
			q.Set(key, value)
		}
		httpReq.URL.RawQuery = q.Encode()
	}

	// 发送请求
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		requestLog.HttpCode = 0
		requestLog.ErrorType = "network_error"
		requestLog.ErrorMessage = err.Error()
		fmt.Printf("[Ailurus PaaS] 请求失败 (trace_id=%s): %v\n", traceID, err)
		return false, err
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	requestLog.HttpCode = status

	fmt.Printf("[Ailurus PaaS] 收到响应 (trace_id=%s, status=%d)\n", traceID, status)

	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		// 复制响应头
		for key, values := range resp.Header {
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
		c.Writer.WriteHeader(status)

		fmt.Printf("[Ailurus PaaS] 开始流式传输 (trace_id=%s)\n", traceID)

		// 同步集成：发布流式开始事件
		if actualStream && prs.syncIntegration != nil {
			prs.syncIntegration.OnStreamStart(c, kind, model, provider.Name, traceID)
		}

		// 流式复制响应体
		if actualStream {
			// 真实的流式响应
			hook := ReqeustLogHook(c, kind, requestLog)
			buf := make([]byte, 4096)
			for {
				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					data := buf[:n]
					// 调用钩子解析数据
					shouldContinue, processedData := hook(data)
					// 写入客户端
					if _, writeErr := c.Writer.Write(processedData); writeErr != nil {
						fmt.Printf("[Ailurus PaaS] 写入客户端失败 (trace_id=%s): %v\n", traceID, writeErr)
						return false, writeErr
					}
					c.Writer.(http.Flusher).Flush()

					// Body 日志：捕获响应数据（限制 10MB 防止内存溢出）
					if shouldLogBody && responseBuffer.Len() < 10*1024*1024 {
						responseBuffer.Write(processedData)
					}

					if !shouldContinue {
						break
					}
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					fmt.Printf("[Ailurus PaaS] 读取响应失败 (trace_id=%s): %v\n", traceID, readErr)
					return false, readErr
				}
			}
		} else if needsStreamConversion {
			// Gemini 特殊处理：读取非流式响应，转换格式，提取usage，然后模拟流式返回
			respData, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				fmt.Printf("[Ailurus PaaS] 读取响应失败 (trace_id=%s): %v\n", traceID, readErr)
				return false, readErr
			}

			respStr := string(respData)

			// 如果是Gemini原生响应，需要转换为OpenAI格式
			var openAIResp map[string]interface{}
			if isGemini {
				// 解析Gemini原生响应
				var geminiResp GeminiResponse
				if err := json.Unmarshal(respData, &geminiResp); err != nil {
					fmt.Printf("[ERROR] 解析Gemini响应失败 (trace_id=%s): %v\n", traceID, err)
					return false, err
				}

				// 从Gemini响应提取token统计
				requestLog.InputTokens = geminiResp.UsageMetadata.PromptTokenCount
				requestLog.OutputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
				fmt.Printf("[DEBUG] Gemini原生响应token统计 (trace_id=%s): in=%d, out=%d\n",
					traceID, requestLog.InputTokens, requestLog.OutputTokens)

				// 如果是 gemini-cli 平台，保持 Gemini 原生格式；否则转换为 OpenAI 格式
				if kind == "gemini-cli" {
					// 保持 Gemini 原生格式（无需转换）
					respStr = string(respData)
					fmt.Printf("[DEBUG] Gemini响应保持原生格式 (trace_id=%s)\n", traceID)
				} else {
					// 转换为OpenAI格式
					converter := &GeminiConverter{}
					openAIResp = converter.ConvertGeminiToOpenAI(&geminiResp, model)

					// 序列化为JSON
					openAIBytes, _ := json.Marshal(openAIResp)
					respStr = string(openAIBytes)
					fmt.Printf("[DEBUG] Gemini响应已转换为OpenAI格式 (trace_id=%s)\n", traceID)
				}
			} else {
				// 非Gemini，使用原有解析逻辑
				parserFn := ClaudeCodeParseTokenUsageFromResponse
				if kind == "codex" {
					parserFn = CodexParseTokenUsageFromResponse
				}
				parserFn(respStr, requestLog)
			}

			// 捕获原始响应
			if shouldLogBody && len(respData) <= 10*1024*1024 {
				responseBuffer.Write(respData)
			}

			// 将响应转换为SSE流式格式返回给客户端
			if err := prs.simulateStreamResponse(c, respStr, traceID); err != nil {
				fmt.Printf("[Ailurus PaaS] 模拟流式响应失败 (trace_id=%s): %v\n", traceID, err)
				return false, err
			}
		} else {
			// 非流式响应
			respData, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				fmt.Printf("[Ailurus PaaS] 读取响应失败 (trace_id=%s): %v\n", traceID, readErr)
				return false, readErr
			}

			// 检查并解压 gzip 数据
			if len(respData) > 2 && respData[0] == 0x1f && respData[1] == 0x8b {
				fmt.Printf("[DEBUG] 检测到gzip压缩响应，开始解压 (trace_id=%s)\n", traceID)
				gzReader, err := gzip.NewReader(bytes.NewReader(respData))
				if err != nil {
					fmt.Printf("[ERROR] 创建gzip reader失败 (trace_id=%s): %v\n", traceID, err)
					return false, err
				}
				defer gzReader.Close()

				decompressed, err := io.ReadAll(gzReader)
				if err != nil {
					fmt.Printf("[ERROR] 解压gzip数据失败 (trace_id=%s): %v\n", traceID, err)
					return false, err
				}
				respData = decompressed
				fmt.Printf("[DEBUG] gzip解压成功，原始大小=%d (trace_id=%s)\n", len(respData), traceID)
			}

			respStr := string(respData)
			var finalData []byte

			// Gemini 原生格式转换为 OpenAI 格式
			if isGemini {
				// 解析 Gemini 原生响应
				var geminiResp GeminiResponse
				if err := json.Unmarshal(respData, &geminiResp); err != nil {
					fmt.Printf("[ERROR] 解析Gemini响应失败 (trace_id=%s): %v\n", traceID, err)
					return false, err
				}

				// 从 Gemini 响应提取 token 统计
				requestLog.InputTokens = geminiResp.UsageMetadata.PromptTokenCount
				requestLog.OutputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
				fmt.Printf("[DEBUG] Gemini原生响应token统计 (trace_id=%s): in=%d, out=%d\n",
					traceID, requestLog.InputTokens, requestLog.OutputTokens)

				// 如果是 gemini-cli 平台，保持 Gemini 原生格式；否则转换为 OpenAI 格式
				if kind == "gemini-cli" {
					// 保持 Gemini 原生格式（无需转换）
					finalData = respData
					respStr = string(respData)
					fmt.Printf("[DEBUG] Gemini响应保持原生格式 (trace_id=%s)\n", traceID)
				} else {
					// 转换为 OpenAI 格式
					converter := &GeminiConverter{}
					openAIResp := converter.ConvertGeminiToOpenAI(&geminiResp, model)

					// 序列化为 JSON
					openAIBytes, _ := json.Marshal(openAIResp)
					finalData = openAIBytes
					respStr = string(openAIBytes)
					fmt.Printf("[DEBUG] Gemini响应已转换为OpenAI格式 (trace_id=%s)\n", traceID)
				}
			} else {
				// 非 Gemini，解析原始响应的 usage
				parserFn := ClaudeCodeParseTokenUsageFromResponse
				if kind == "codex" {
					parserFn = CodexParseTokenUsageFromResponse
				}
				parserFn(respStr, requestLog)
				fmt.Printf("[DEBUG] 非流式响应 token 统计 (trace_id=%s): in=%d, out=%d\n",
					traceID, requestLog.InputTokens, requestLog.OutputTokens)
				finalData = respData
			}

			// 捕获响应数据（限制 10MB）
			if shouldLogBody {
				if len(respData) <= 10*1024*1024 {
					responseBuffer.Write(respData) // 记录原始响应
				} else {
					responseBuffer.Write(respData[:10*1024*1024])
				}
			}

			// 写入客户端（转换后的数据）
			if _, writeErr := c.Writer.Write(finalData); writeErr != nil {
				fmt.Printf("[Ailurus PaaS] 复制响应失败 (trace_id=%s): %v\n", traceID, writeErr)
				return false, writeErr
			}
		}

		fmt.Printf("[Ailurus PaaS] 流式传输完成 (trace_id=%s, in=%d, out=%d, total_cost=%.6f)\n",
			traceID, requestLog.InputTokens, requestLog.OutputTokens, requestLog.TotalCost)

		// 同步集成：发布请求完成事件（成功）
		if prs.syncIntegration != nil {
			prs.syncIntegration.OnRequestComplete(
				c, kind, provider.Name, model, traceID,
				true, status, "",
				requestLog.InputTokens, requestLog.OutputTokens,
				requestLog.TotalCost, int(requestLog.DurationSec*1000),
			)
		}
		return true, nil
	}

	// 记录 HTTP 错误详情
	requestLog.ErrorType = classifyHTTPError(status)
	respBody, _ := io.ReadAll(resp.Body)
	requestLog.ErrorMessage = string(respBody)

	// 尝试解析供应商错误码
	if errorCode := gjson.Get(string(respBody), "error.code").String(); errorCode != "" {
		requestLog.ProviderErrorCode = errorCode
	} else if errorType := gjson.Get(string(respBody), "error.type").String(); errorType != "" {
		requestLog.ProviderErrorCode = errorType
	}

	// 打印详细的错误信息
	fmt.Printf("[ERROR] Upstream error (trace_id=%s, status=%d): %s\n", traceID, status, string(respBody))

	// 同步集成：发布请求完成事件（失败）
	if prs.syncIntegration != nil {
		prs.syncIntegration.OnRequestComplete(
			c, kind, provider.Name, model, traceID,
			false, status, string(respBody),
			requestLog.InputTokens, requestLog.OutputTokens,
			requestLog.TotalCost, int(requestLog.DurationSec*1000),
		)
	}

	return false, fmt.Errorf("upstream status %d: %s", status, string(respBody))
}

func cloneHeaders(header http.Header) map[string]string {
	cloned := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) > 0 {
			cloned[key] = values[len(values)-1]
		}
	}
	return cloned
}

func cloneMap(m map[string]string) map[string]string {
	cloned := make(map[string]string, len(m))
	for k, v := range m {
		cloned[k] = v
	}
	return cloned
}

func flattenQuery(values map[string][]string) map[string]string {
	query := make(map[string]string, len(values))
	for key, items := range values {
		if len(items) > 0 {
			query[key] = items[len(items)-1]
		}
	}
	return query
}

func joinURL(base string, endpoint string) string {
	base = strings.TrimSuffix(base, "/")
	endpoint = "/" + strings.TrimPrefix(endpoint, "/")
	return base + endpoint
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// generateTraceID 生成全局唯一追踪 ID (UUID v4 格式)
func generateTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 降级：使用时间戳
		return fmt.Sprintf("trace-%d", time.Now().UnixNano())
	}
	// UUID v4 格式：xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// getClientIP 从请求中提取真实客户端 IP
func getClientIP(c *gin.Context) string {
	// 优先从代理头获取
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For 可能包含多个 IP，取第一个
		if idx := strings.Index(ip, ","); idx > 0 {
			return strings.TrimSpace(ip[:idx])
		}
		return ip
	}
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	// 降级使用直连 IP
	return c.ClientIP()
}

// classifyHTTPError 根据 HTTP 状态码分类错误类型
func classifyHTTPError(statusCode int) string {
	switch {
	case statusCode == 0:
		return "network_error"
	case statusCode == 401 || statusCode == 403:
		return "auth_error"
	case statusCode == 429:
		return "rate_limit"
	case statusCode >= 400 && statusCode < 500:
		return "client_error"
	case statusCode >= 500:
		return "server_error"
	default:
		return "unknown_error"
	}
}

func ensureRequestLogColumn(db *sql.DB, column string, definition string) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('request_log') WHERE name = '%s'", column)
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		alter := fmt.Sprintf("ALTER TABLE request_log ADD COLUMN %s %s", column, definition)
		if _, err := db.Exec(alter); err != nil {
			return err
		}
	}
	return nil
}

func ensureRequestLogTable() error {
	db, err := xdb.DB("default")
	if err != nil {
		return err
	}
	return ensureRequestLogTableWithDB(db)
}

func ensureRequestLogTableWithDB(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}

	const createTableSQL = `CREATE TABLE IF NOT EXISTS request_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trace_id TEXT,
		request_id TEXT,
		platform TEXT,
		model TEXT,
		provider TEXT,
		http_code INTEGER,
		input_tokens INTEGER,
		output_tokens INTEGER,
		cache_create_tokens INTEGER,
		cache_read_tokens INTEGER,
		reasoning_tokens INTEGER,
		is_stream INTEGER DEFAULT 0,
		duration_sec REAL DEFAULT 0,
		user_agent TEXT,
		client_ip TEXT,
		user_id TEXT,
		request_method TEXT,
		request_path TEXT,
		error_type TEXT,
		error_message TEXT,
		provider_error_code TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`

	if _, err := db.Exec(createTableSQL); err != nil {
		return err
	}

	if err := ensureRequestLogColumn(db, "created_at", "DATETIME DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "is_stream", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "duration_sec", "REAL DEFAULT 0"); err != nil {
		return err
	}

	// Ailurus PaaS 增强字段 - 追踪和监控
	if err := ensureRequestLogColumn(db, "trace_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "request_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "user_agent", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "client_ip", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "user_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "request_method", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "request_path", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "error_type", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "error_message", "TEXT"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_error_code", "TEXT"); err != nil {
		return err
	}

	// 价格字段 - 用于性能优化，避免重复计算
	if err := ensureRequestLogColumn(db, "input_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "output_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "cache_create_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "cache_read_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "ephemeral_5m_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "ephemeral_1h_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "total_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}

	// 创建索引以提升查询性能
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_trace_id ON request_log(trace_id)",
		"CREATE INDEX IF NOT EXISTS idx_request_id ON request_log(request_id)",
		"CREATE INDEX IF NOT EXISTS idx_platform_provider ON request_log(platform, provider)",
		"CREATE INDEX IF NOT EXISTS idx_created_at ON request_log(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_http_code ON request_log(http_code)",
		"CREATE INDEX IF NOT EXISTS idx_user_id ON request_log(user_id)",
		// 复合索引优化聚合查询（provider/platform/model + created_at）
		"CREATE INDEX IF NOT EXISTS idx_provider_created_at ON request_log(provider, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_platform_created_at ON request_log(platform, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_model_created_at ON request_log(model, created_at)",
	}
	for _, sql := range indexes {
		if _, err := db.Exec(sql); err != nil {
			return err
		}
	}

	// 创建 request_log_body 表（独立存储请求/响应体，便于清理）
	const createBodyTableSQL = `CREATE TABLE IF NOT EXISTS request_log_body (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trace_id TEXT NOT NULL,
		request_body TEXT,
		response_body TEXT,
		body_size_bytes INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME
	)`
	if _, err := db.Exec(createBodyTableSQL); err != nil {
		return err
	}

	// 创建 body 表索引
	bodyIndexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_body_trace_id ON request_log_body(trace_id)",
		"CREATE INDEX IF NOT EXISTS idx_body_expires_at ON request_log_body(expires_at)",
	}
	for _, sql := range bodyIndexes {
		if _, err := db.Exec(sql); err != nil {
			return err
		}
	}

	return nil
}

func ReqeustLogHook(c *gin.Context, kind string, usage *ReqeustLog) func(data []byte) (bool, []byte) { // SSE 钩子：累计字节和解析 token 用量
	return func(data []byte) (bool, []byte) {
		payload := strings.TrimSpace(string(data))

		parserFn := ClaudeCodeParseTokenUsageFromResponse
		if kind == "codex" {
			parserFn = CodexParseTokenUsageFromResponse
		}
		parseEventPayload(payload, parserFn, usage)

		return true, data
	}
}

func parseEventPayload(payload string, parser func(string, *ReqeustLog), usage *ReqeustLog) {
	lines := strings.Split(payload, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			parser(strings.TrimPrefix(line, "data: "), usage)
		}
	}
}

type ReqeustLog struct {
	ID                int64   `json:"id"`
	TraceID           string  `json:"trace_id"`   // 全局追踪 ID (UUID)
	RequestID         string  `json:"request_id"` // 客户端请求 ID
	Platform          string  `json:"platform"`   // claude code or codex
	Model             string  `json:"model"`
	Provider          string  `json:"provider"` // provider name
	HttpCode          int     `json:"http_code"`
	InputTokens       int     `json:"input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	CacheCreateTokens int     `json:"cache_create_tokens"`
	CacheReadTokens   int     `json:"cache_read_tokens"`
	ReasoningTokens   int     `json:"reasoning_tokens"`
	IsStream          bool    `json:"is_stream"`
	DurationSec       float64 `json:"duration_sec"`
	UserAgent         string  `json:"user_agent"`          // 用户代理（识别 TUI/GUI 客户端）
	ClientIP          string  `json:"client_ip"`           // 客户端 IP
	UserID            string  `json:"user_id"`             // 用户标识（多租户）
	RequestMethod     string  `json:"request_method"`      // HTTP 方法
	RequestPath       string  `json:"request_path"`        // 请求路径
	ErrorType         string  `json:"error_type"`          // 错误类型（network/auth/rate_limit/server/etc）
	ErrorMessage      string  `json:"error_message"`       // 错误详细信息
	ProviderErrorCode string  `json:"provider_error_code"` // 供应商错误码
	CreatedAt         string  `json:"created_at"`
	InputCost         float64 `json:"input_cost"`
	OutputCost        float64 `json:"output_cost"`
	CacheCreateCost   float64 `json:"cache_create_cost"`
	CacheReadCost     float64 `json:"cache_read_cost"`
	Ephemeral5mCost   float64 `json:"ephemeral_5m_cost"`
	Ephemeral1hCost   float64 `json:"ephemeral_1h_cost"`
	TotalCost         float64 `json:"total_cost"`
	HasPricing        bool    `json:"has_pricing"`
}

// RequestLogBody 请求/响应体存储结构（独立表，7天过期）
type RequestLogBody struct {
	ID            int64     `json:"id"`
	TraceID       string    `json:"trace_id"`
	RequestBody   string    `json:"request_body"`
	ResponseBody  string    `json:"response_body"`
	BodySizeBytes int64     `json:"body_size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// claude code usage parser
func ClaudeCodeParseTokenUsageFromResponse(data string, usage *ReqeustLog) {
	usage.InputTokens += int(gjson.Get(data, "message.usage.input_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "message.usage.output_tokens").Int())
	usage.CacheCreateTokens += int(gjson.Get(data, "message.usage.cache_creation_input_tokens").Int())
	usage.CacheReadTokens += int(gjson.Get(data, "message.usage.cache_read_input_tokens").Int())

	usage.InputTokens += int(gjson.Get(data, "usage.input_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "usage.output_tokens").Int())
}

// codex usage parser - 支持多种格式
func CodexParseTokenUsageFromResponse(data string, usage *ReqeustLog) {
	// 调试日志：输出包含 usage 信息的 SSE 数据
	if strings.Contains(data, "usage") || strings.Contains(data, "usageMetadata") {
		fmt.Printf("[DEBUG] SSE data with usage (provider=%s): %s\n", usage.Provider, data)
	}

	// OpenAI Responses API 格式 (response.usage)
	usage.InputTokens += int(gjson.Get(data, "response.usage.input_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "response.usage.output_tokens").Int())
	usage.CacheReadTokens += int(gjson.Get(data, "response.usage.input_tokens_details.cached_tokens").Int())
	usage.ReasoningTokens += int(gjson.Get(data, "response.usage.output_tokens_details.reasoning_tokens").Int())

	// 标准 OpenAI Chat Completions 格式 (usage.prompt_tokens / completion_tokens)
	// DeepSeek、GLM、OpenAI 等供应商使用此格式
	usage.InputTokens += int(gjson.Get(data, "usage.prompt_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "usage.completion_tokens").Int())

	// 兼容一些供应商使用 input_tokens/output_tokens 的情况
	usage.InputTokens += int(gjson.Get(data, "usage.input_tokens").Int())
	usage.OutputTokens += int(gjson.Get(data, "usage.output_tokens").Int())

	// Google Gemini usageMetadata 格式
	usage.InputTokens += int(gjson.Get(data, "usageMetadata.promptTokenCount").Int())
	usage.OutputTokens += int(gjson.Get(data, "usageMetadata.candidatesTokenCount").Int())
	usage.CacheReadTokens += int(gjson.Get(data, "usageMetadata.cachedContentTokenCount").Int())

	// DeepSeek 推理 tokens（在 completion_tokens_details 中）
	usage.ReasoningTokens += int(gjson.Get(data, "usage.completion_tokens_details.reasoning_tokens").Int())
	// 缓存相关（部分供应商支持）
	usage.CacheReadTokens += int(gjson.Get(data, "usage.prompt_tokens_details.cached_tokens").Int())
}

// ReplaceModelInRequestBody 替换请求体中的模型名
// 使用 gjson + sjson 实现高性能 JSON 操作，避免完整反序列化
func ReplaceModelInRequestBody(bodyBytes []byte, newModel string) ([]byte, error) {
	// 检查请求体中是否存在 model 字段
	result := gjson.GetBytes(bodyBytes, "model")
	if !result.Exists() {
		return bodyBytes, fmt.Errorf("请求体中未找到 model 字段")
	}

	// 使用 sjson.SetBytes 替换模型名（高性能操作）
	modified, err := sjson.SetBytes(bodyBytes, "model", newModel)
	if err != nil {
		return bodyBytes, fmt.Errorf("替换模型名失败: %w", err)
	}

	return modified, nil
}

// simulateStreamResponse 将非流式响应模拟为 SSE 流式响应返回给客户端
// 用于 Google Gemini 等流式响应不包含 usage 信息的 provider
func (prs *ProviderRelayService) simulateStreamResponse(c *gin.Context, respStr string, traceID string) error {
	// 解析响应 JSON
	var respData map[string]interface{}
	if err := json.Unmarshal([]byte(respStr), &respData); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	// 提取基本信息
	id, _ := respData["id"].(string)
	model, _ := respData["model"].(string)
	created, _ := respData["created"].(float64)

	// 提取 choices
	choices, ok := respData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return fmt.Errorf("响应中没有 choices")
	}

	choice := choices[0].(map[string]interface{})
	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("响应中没有 message")
	}

	role, _ := message["role"].(string)
	finishReason, _ := choice["finish_reason"].(string)

	// 第一个 chunk: role + content/tool_calls
	delta := map[string]interface{}{
		"role": role,
	}

	// 检查是否有 content
	if content, hasContent := message["content"].(string); hasContent && content != "" {
		delta["content"] = content
	}

	// 检查是否有 tool_calls（保留原始数据，包括 Gemini 的 thought_signature）
	if toolCalls, hasToolCalls := message["tool_calls"].([]interface{}); hasToolCalls {
		// 直接使用原始 tool_calls，保留 extra_content 中的 thought_signature
		delta["tool_calls"] = toolCalls
	}

	chunk1 := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": int64(created),
		"model":   model,
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"delta": delta,
			},
		},
	}

	chunk1Bytes, _ := json.Marshal(chunk1)
	c.Writer.Write([]byte("data: "))
	c.Writer.Write(chunk1Bytes)
	c.Writer.Write([]byte("\n\n"))
	c.Writer.(http.Flusher).Flush()

	// 第二个 chunk: finish_reason
	chunk2 := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": int64(created),
		"model":   model,
		"choices": []interface{}{
			map[string]interface{}{
				"index":         0,
				"delta":         map[string]interface{}{}, // 空 delta
				"finish_reason": finishReason,
			},
		},
	}

	chunk2Bytes, _ := json.Marshal(chunk2)
	c.Writer.Write([]byte("data: "))
	c.Writer.Write(chunk2Bytes)
	c.Writer.Write([]byte("\n\n"))
	c.Writer.(http.Flusher).Flush()

	// 最后: [DONE]
	c.Writer.Write([]byte("data: [DONE]\n\n"))
	c.Writer.(http.Flusher).Flush()

	fmt.Printf("[DEBUG] Gemini 流式模拟完成 (trace_id=%s, finish_reason=%s)\n", traceID, finishReason)
	return nil
}

// geminiNativeHandler 处理 Gemini 原生 API 请求
// 支持 /v1beta/models/{model}:generateContent 和 /v1beta/models/{model}:streamGenerateContent
func (prs *ProviderRelayService) geminiNativeHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 URL 路径提取模型名和操作（如 gemini-2.5-pro:generateContent）
		modelAction := strings.TrimPrefix(c.Param("modelAction"), "/")

		// 解析模型和操作
		var model string
		var isStream bool
		if strings.HasSuffix(modelAction, ":streamGenerateContent") {
			model = strings.TrimSuffix(modelAction, ":streamGenerateContent")
			isStream = true
		} else if strings.HasSuffix(modelAction, ":generateContent") {
			model = strings.TrimSuffix(modelAction, ":generateContent")
			isStream = false
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Gemini API endpoint"})
			return
		}
		if model == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "model parameter required"})
			return
		}

		// 检测是否请求 SSE 格式
		altFormat := c.Query("alt")
		needSSEFormat := altFormat == "sse"
		if needSSEFormat {
			fmt.Printf("[Gemini Native] 客户端请求 SSE 格式 (alt=%s)\n", altFormat)
		}

		// 读取 Gemini 原生格式请求体
		var bodyBytes []byte
		if c.Request.Body != nil {
			data, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
			bodyBytes = data
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// NEW-API 统一网关模式：转换格式并转发到 new-api
		if prs.IsNewAPIEnabled() && prs.newAPIURL != "" && prs.newAPIToken != "" {
			fmt.Printf("[Gemini Native] NEW-API 模式: 转发到 %s (model=%s, stream=%v)\n",
				prs.newAPIURL, model, isStream)

			success, err := prs.forwardGeminiToNewAPI(c, model, bodyBytes, isStream, needSSEFormat)
			if success {
				return
			}

			// NEW-API 失败，尝试 fallback 到本地 gemini-cli provider
			fmt.Printf("[WARN] Gemini->NewAPI 请求失败 (%v), 尝试 fallback 到本地 provider\n", err)
		}

		// 加载 gemini-cli 平台的 providers
		providers, err := prs.providerService.LoadProviders("gemini-cli")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load providers"})
			return
		}

		fmt.Printf("[Gemini Native] 加载了 %d 个 gemini-cli providers\n", len(providers))

		// 过滤出支持该模型的 active providers
		active := make([]Provider, 0, len(providers))
		skippedCount := 0
		for _, provider := range providers {
			fmt.Printf("[DEBUG] 检查 provider: %s (enabled=%v)\n", provider.Name, provider.Enabled)
			if !provider.Enabled || provider.APIURL == "" || provider.APIKey == "" {
				fmt.Printf("[DEBUG] Provider %s 被跳过: enabled=%v, hasURL=%v, hasKey=%v\n",
					provider.Name, provider.Enabled, provider.APIURL != "", provider.APIKey != "")
				continue
			}
			if errs := provider.ValidateConfiguration(); len(errs) > 0 {
				fmt.Printf("[DEBUG] Provider %s 配置验证失败: %v\n", provider.Name, errs)
				skippedCount++
				continue
			}
			supported := provider.IsModelSupported(model)
			fmt.Printf("[DEBUG] Provider %s 是否支持模型 %s: %v\n", provider.Name, model, supported)
			if !supported {
				fmt.Printf("[INFO] Provider %s 不支持模型 %s，已跳过\n", provider.Name, model)
				skippedCount++
				continue
			}
			active = append(active, provider)
		}

		if len(active) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("没有可用的 provider 支持模型 '%s'", model),
			})
			return
		}

		// 使用第一个匹配的 provider
		provider := active[0]

		// 应用模型映射
		mappedModel := model
		if provider.ModelMapping != nil {
			if mapped, ok := provider.ModelMapping[model]; ok {
				mappedModel = mapped
				fmt.Printf("[Gemini Native] 模型映射: %s -> %s\n", model, mappedModel)
			}
		}

		fmt.Printf("[Gemini Native] 使用 provider: %s (model=%s)\n", provider.Name, mappedModel)

		// 构建目标 URL（Gemini 原生格式）
		action := "generateContent"
		if isStream {
			action = "streamGenerateContent"
		}
		targetPath := fmt.Sprintf("/models/%s:%s", mappedModel, action)

		fmt.Printf("[Ailurus PaaS] Gemini使用原生API: %s%s?key=%s\n", provider.APIURL, targetPath, "***")

		// 如果客户端请求 SSE 格式，需要特殊处理
		if needSSEFormat && isStream {
			// 直接发送请求到 Google API
			targetURL := fmt.Sprintf("%s%s?key=%s", provider.APIURL, targetPath, provider.APIKey)
			req, err := http.NewRequest("POST", targetURL, bytes.NewReader(bodyBytes))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "create request failed"})
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 5 * 60 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("request failed: %v", err)})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				c.JSON(resp.StatusCode, gin.H{"error": string(body)})
				return
			}

			// 读取完整响应
			respData, err := io.ReadAll(resp.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "read response failed"})
				return
			}

			// 解析 JSON 数组 [{...},{...}]
			var jsonArray []map[string]interface{}
			if err := json.Unmarshal(respData, &jsonArray); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("parse response failed: %v", err)})
				return
			}

			// 转换为 SSE 格式并发送
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.WriteHeader(200)

			for _, chunk := range jsonArray {
				chunkBytes, _ := json.Marshal(chunk)
				c.Writer.Write([]byte("data: "))
				c.Writer.Write(chunkBytes)
				c.Writer.Write([]byte("\r\n\r\n"))
				if f, ok := c.Writer.(http.Flusher); ok {
					f.Flush()
				}
			}

			// Google Gemini SSE 不发送 [DONE] 标记，直接关闭连接即可
			fmt.Printf("[Gemini Native] SSE 格式转换完成 (%d chunks)\n", len(jsonArray))
			return
		}

		// 否则直接转发 Gemini 原生格式请求
		success, err := prs.forwardRequest(
			c,
			"gemini-cli",
			provider,
			targetPath,
			map[string]string{"key": provider.APIKey}, // query
			nil, // clientHeaders
			bodyBytes,
			isStream,
			mappedModel,
		)

		if !success && err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("request failed: %v", err)})
			return
		}
	}
}

// forwardToNewAPI 将请求转发到 NEW-API 统一网关
// 返回 (成功, 错误)
func (prs *ProviderRelayService) forwardToNewAPI(
	c *gin.Context,
	kind string,
	endpoint string,
	bodyBytes []byte,
	isStream bool,
	model string,
) (bool, error) {
	// 生成追踪 ID
	traceID := generateTraceID()
	c.Header("X-Trace-ID", traceID)

	// 根据 kind 选择 new-api 的端点
	// Claude Code 使用 /v1/messages (Anthropic 格式)
	// Codex 和其他使用 /v1/chat/completions (OpenAI 格式)
	var targetEndpoint string
	switch kind {
	case "claude":
		targetEndpoint = "/v1/messages"
	case "codex":
		// Codex 可能使用 /responses 或 /v1/chat/completions
		if endpoint == "/responses" {
			targetEndpoint = "/v1/chat/completions" // new-api 统一使用 OpenAI 格式
		} else {
			targetEndpoint = endpoint
		}
	default:
		targetEndpoint = "/v1/chat/completions"
	}

	targetURL := strings.TrimSuffix(prs.newAPIURL, "/") + targetEndpoint

	// 初始化请求日志
	requestLog := &ReqeustLog{
		TraceID:       traceID,
		RequestID:     c.GetHeader("X-Request-ID"),
		Platform:      kind,
		Provider:      "new-api", // 标记为 new-api 统一网关
		Model:         model,
		IsStream:      isStream,
		UserAgent:     c.GetHeader("User-Agent"),
		ClientIP:      getClientIP(c),
		UserID:        c.GetHeader("X-User-ID"),
		RequestMethod: c.Request.Method,
		RequestPath:   c.Request.URL.Path,
	}

	// Body 日志捕获
	shouldLogBody := prs.IsBodyLogEnabled()
	var responseBuffer bytes.Buffer

	start := time.Now()
	defer func() {
		requestLog.DurationSec = time.Since(start).Seconds()

		// 计算价格
		if prs.pricingService != nil {
			costBreakdown := prs.pricingService.CalculateCost(requestLog.Model, modelpricing.UsageSnapshot{
				InputTokens:       requestLog.InputTokens,
				OutputTokens:      requestLog.OutputTokens,
				CacheCreateTokens: requestLog.CacheCreateTokens,
				CacheReadTokens:   requestLog.CacheReadTokens,
			})
			requestLog.InputCost = costBreakdown.InputCost
			requestLog.OutputCost = costBreakdown.OutputCost
			requestLog.CacheCreateCost = costBreakdown.CacheCreateCost
			requestLog.CacheReadCost = costBreakdown.CacheReadCost
			requestLog.Ephemeral5mCost = costBreakdown.Ephemeral5mCost
			requestLog.Ephemeral1hCost = costBreakdown.Ephemeral1hCost
			requestLog.TotalCost = costBreakdown.TotalCost
		}

		// 发送到写入队列
		select {
		case prs.logWriteQueue <- requestLog:
		default:
			fmt.Printf("[WARN] 日志队列已满，丢弃日志 (trace_id=%s)\n", requestLog.TraceID)
		}

		// Body 日志
		if shouldLogBody && (len(bodyBytes) > 0 || responseBuffer.Len() > 0) {
			bodyLog := &RequestLogBody{
				TraceID:       traceID,
				RequestBody:   string(bodyBytes),
				ResponseBody:  responseBuffer.String(),
				BodySizeBytes: int64(len(bodyBytes) + responseBuffer.Len()),
				CreatedAt:     time.Now(),
				ExpiresAt:     time.Now().Add(7 * 24 * time.Hour),
			}
			select {
			case prs.bodyLogQueue <- bodyLog:
			default:
				fmt.Printf("[WARN] Body log queue full, dropped trace_id=%s\n", traceID)
			}
		}
	}()

	// 创建 HTTP 客户端
	timeout := 60 * time.Second
	if isStream {
		timeout = 300 * time.Second
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	fmt.Printf("[Ailurus PaaS] NEW-API 请求 (trace_id=%s, url=%s, model=%s, stream=%v)\n",
		traceID, targetURL, model, isStream)

	// 创建请求
	httpReq, err := http.NewRequest("POST", targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		requestLog.HttpCode = 0
		requestLog.ErrorType = "network_error"
		requestLog.ErrorMessage = err.Error()
		return false, err
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+prs.newAPIToken)
	httpReq.Header.Set("X-Trace-ID", traceID)

	// 复制原始请求的部分头
	if ua := c.GetHeader("User-Agent"); ua != "" {
		httpReq.Header.Set("X-Original-User-Agent", ua)
	}
	if userID := c.GetHeader("X-User-ID"); userID != "" {
		httpReq.Header.Set("X-User-ID", userID)
	}

	// 发送请求
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		requestLog.HttpCode = 0
		requestLog.ErrorType = "network_error"
		requestLog.ErrorMessage = err.Error()
		fmt.Printf("[Ailurus PaaS] NEW-API 请求失败 (trace_id=%s): %v\n", traceID, err)
		return false, err
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	requestLog.HttpCode = status

	fmt.Printf("[Ailurus PaaS] NEW-API 响应 (trace_id=%s, status=%d)\n", traceID, status)

	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		// 复制响应头
		for key, values := range resp.Header {
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
		c.Writer.WriteHeader(status)

		// 同步集成：发布流式开始事件
		if isStream && prs.syncIntegration != nil {
			prs.syncIntegration.OnStreamStart(c, kind, model, "new-api", traceID)
		}

		// 流式/非流式响应处理
		if isStream {
			// 流式响应
			hook := ReqeustLogHook(c, kind, requestLog)
			buf := make([]byte, 4096)
			for {
				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					data := buf[:n]
					shouldContinue, processedData := hook(data)

					if _, writeErr := c.Writer.Write(processedData); writeErr != nil {
						return false, writeErr
					}
					c.Writer.(http.Flusher).Flush()

					if shouldLogBody && responseBuffer.Len() < 10*1024*1024 {
						responseBuffer.Write(processedData)
					}

					if !shouldContinue {
						break
					}
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					return false, readErr
				}
			}
		} else {
			// 非流式响应
			respData, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return false, readErr
			}

			// 解析 token 用量
			respStr := string(respData)
			parserFn := ClaudeCodeParseTokenUsageFromResponse
			if kind == "codex" {
				parserFn = CodexParseTokenUsageFromResponse
			}
			parserFn(respStr, requestLog)

			// 捕获响应
			if shouldLogBody && len(respData) <= 10*1024*1024 {
				responseBuffer.Write(respData)
			}

			// 写入客户端
			if _, writeErr := c.Writer.Write(respData); writeErr != nil {
				return false, writeErr
			}
		}

		fmt.Printf("[Ailurus PaaS] NEW-API 完成 (trace_id=%s, in=%d, out=%d, cost=%.6f)\n",
			traceID, requestLog.InputTokens, requestLog.OutputTokens, requestLog.TotalCost)

		// 同步集成：发布请求完成事件
		if prs.syncIntegration != nil {
			prs.syncIntegration.OnRequestComplete(
				c, kind, "new-api", model, traceID,
				true, status, "",
				requestLog.InputTokens, requestLog.OutputTokens,
				requestLog.TotalCost, int(requestLog.DurationSec*1000),
			)
		}

		return true, nil
	}

	// 错误响应
	requestLog.ErrorType = classifyHTTPError(status)
	respBody, _ := io.ReadAll(resp.Body)
	requestLog.ErrorMessage = string(respBody)

	if errorCode := gjson.Get(string(respBody), "error.code").String(); errorCode != "" {
		requestLog.ProviderErrorCode = errorCode
	}

	fmt.Printf("[ERROR] NEW-API error (trace_id=%s, status=%d): %s\n", traceID, status, string(respBody))

	// 同步集成：发布请求完成事件（失败）
	if prs.syncIntegration != nil {
		prs.syncIntegration.OnRequestComplete(
			c, kind, "new-api", model, traceID,
			false, status, string(respBody),
			requestLog.InputTokens, requestLog.OutputTokens,
			requestLog.TotalCost, int(requestLog.DurationSec*1000),
		)
	}

	return false, fmt.Errorf("new-api status %d: %s", status, string(respBody))
}

// ============================================================================
// Gemini ↔ OpenAI 格式转换函数（用于 new-api 统一网关）
// ============================================================================

// convertGeminiToOpenAI 将 Gemini 原生格式请求转换为 OpenAI 兼容格式
// Gemini 格式: { "contents": [{"parts": [{"text": "..."}], "role": "user"}] }
// OpenAI 格式: { "model": "...", "messages": [{"role": "user", "content": "..."}] }
func convertGeminiToOpenAI(geminiBody []byte, model string, isStream bool) ([]byte, error) {
	// 解析 Gemini contents
	contents := gjson.GetBytes(geminiBody, "contents")
	if !contents.Exists() || !contents.IsArray() {
		return nil, fmt.Errorf("invalid Gemini request: missing contents array")
	}

	// 转换为 OpenAI messages
	messages := make([]map[string]interface{}, 0)
	for _, content := range contents.Array() {
		role := content.Get("role").String()
		parts := content.Get("parts").Array()

		// 转换 role: model -> assistant, user -> user
		openAIRole := role
		if role == "model" {
			openAIRole = "assistant"
		}

		// 提取文本内容
		textParts := make([]string, 0)
		for _, part := range parts {
			if text := part.Get("text").String(); text != "" {
				textParts = append(textParts, text)
			}
		}

		if len(textParts) > 0 {
			messages = append(messages, map[string]interface{}{
				"role":    openAIRole,
				"content": strings.Join(textParts, "\n"),
			})
		}
	}

	// 提取 system instruction（如果有）
	systemInstruction := gjson.GetBytes(geminiBody, "systemInstruction")
	if systemInstruction.Exists() {
		systemParts := systemInstruction.Get("parts").Array()
		systemTexts := make([]string, 0)
		for _, part := range systemParts {
			if text := part.Get("text").String(); text != "" {
				systemTexts = append(systemTexts, text)
			}
		}
		if len(systemTexts) > 0 {
			// 将 system instruction 作为第一条消息
			systemMsg := map[string]interface{}{
				"role":    "system",
				"content": strings.Join(systemTexts, "\n"),
			}
			messages = append([]map[string]interface{}{systemMsg}, messages...)
		}
	}

	// 构建 OpenAI 请求
	openAIRequest := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   isStream,
	}

	// 转换 generation config（温度、max_tokens 等）
	generationConfig := gjson.GetBytes(geminiBody, "generationConfig")
	if generationConfig.Exists() {
		if temp := generationConfig.Get("temperature"); temp.Exists() {
			openAIRequest["temperature"] = temp.Float()
		}
		if maxTokens := generationConfig.Get("maxOutputTokens"); maxTokens.Exists() {
			openAIRequest["max_tokens"] = maxTokens.Int()
		}
		if topP := generationConfig.Get("topP"); topP.Exists() {
			openAIRequest["top_p"] = topP.Float()
		}
	}

	return json.Marshal(openAIRequest)
}

// convertOpenAIToGemini 将 OpenAI 响应转换为 Gemini 原生格式
// OpenAI 格式: { "choices": [{"message": {"role": "assistant", "content": "..."}}] }
// Gemini 格式: { "candidates": [{"content": {"parts": [{"text": "..."}], "role": "model"}}] }
func convertOpenAIToGemini(openAIBody []byte) ([]byte, error) {
	// 解析 OpenAI choices
	choices := gjson.GetBytes(openAIBody, "choices")
	if !choices.Exists() || !choices.IsArray() || len(choices.Array()) == 0 {
		// 可能是错误响应，直接返回原始内容
		return openAIBody, nil
	}

	candidates := make([]map[string]interface{}, 0)
	for _, choice := range choices.Array() {
		message := choice.Get("message")
		delta := choice.Get("delta") // 流式响应用 delta

		var role, content string
		if message.Exists() {
			role = message.Get("role").String()
			content = message.Get("content").String()
		} else if delta.Exists() {
			role = delta.Get("role").String()
			content = delta.Get("content").String()
		}

		// 转换 role: assistant -> model
		geminiRole := role
		if role == "assistant" {
			geminiRole = "model"
		}

		// 构建 Gemini candidate
		candidate := map[string]interface{}{
			"content": map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": content},
				},
				"role": geminiRole,
			},
		}

		// 转换 finish_reason
		if finishReason := choice.Get("finish_reason").String(); finishReason != "" {
			geminiFinishReason := strings.ToUpper(finishReason)
			if geminiFinishReason == "STOP" {
				geminiFinishReason = "STOP"
			} else if geminiFinishReason == "LENGTH" {
				geminiFinishReason = "MAX_TOKENS"
			}
			candidate["finishReason"] = geminiFinishReason
		}

		candidates = append(candidates, candidate)
	}

	// 构建 Gemini 响应
	geminiResponse := map[string]interface{}{
		"candidates": candidates,
	}

	// 转换 usage 信息
	usage := gjson.GetBytes(openAIBody, "usage")
	if usage.Exists() {
		geminiResponse["usageMetadata"] = map[string]interface{}{
			"promptTokenCount":     usage.Get("prompt_tokens").Int(),
			"candidatesTokenCount": usage.Get("completion_tokens").Int(),
			"totalTokenCount":      usage.Get("total_tokens").Int(),
		}
	}

	return json.Marshal(geminiResponse)
}

// convertOpenAIStreamChunkToGemini 将 OpenAI 流式响应 chunk 转换为 Gemini 格式
func convertOpenAIStreamChunkToGemini(chunk []byte) ([]byte, error) {
	// OpenAI 流式 chunk: {"choices":[{"delta":{"content":"Hi"},"index":0}]}
	// Gemini 流式 chunk: {"candidates":[{"content":{"parts":[{"text":"Hi"}],"role":"model"}}]}

	choices := gjson.GetBytes(chunk, "choices")
	if !choices.Exists() || !choices.IsArray() {
		return chunk, nil
	}

	candidates := make([]map[string]interface{}, 0)
	for _, choice := range choices.Array() {
		delta := choice.Get("delta")
		content := delta.Get("content").String()

		// 构建 Gemini candidate
		candidate := map[string]interface{}{
			"content": map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": content},
				},
				"role": "model",
			},
		}

		// 转换 finish_reason
		if finishReason := choice.Get("finish_reason").String(); finishReason != "" {
			geminiFinishReason := "STOP"
			if finishReason == "length" {
				geminiFinishReason = "MAX_TOKENS"
			}
			candidate["finishReason"] = geminiFinishReason
		}

		candidates = append(candidates, candidate)
	}

	// 构建 Gemini 响应
	geminiResponse := map[string]interface{}{
		"candidates": candidates,
	}

	// 转换 usage（如果有）
	usage := gjson.GetBytes(chunk, "usage")
	if usage.Exists() {
		geminiResponse["usageMetadata"] = map[string]interface{}{
			"promptTokenCount":     usage.Get("prompt_tokens").Int(),
			"candidatesTokenCount": usage.Get("completion_tokens").Int(),
			"totalTokenCount":      usage.Get("total_tokens").Int(),
		}
	}

	return json.Marshal(geminiResponse)
}

// forwardGeminiToNewAPI 将 Gemini 原生请求转发到 new-api 并转换响应
func (prs *ProviderRelayService) forwardGeminiToNewAPI(
	c *gin.Context,
	model string,
	bodyBytes []byte,
	isStream bool,
	needSSEFormat bool,
) (bool, error) {
	// 生成追踪 ID
	traceID := generateTraceID()
	c.Header("X-Trace-ID", traceID)

	// 转换 Gemini 请求为 OpenAI 格式
	openAIBody, err := convertGeminiToOpenAI(bodyBytes, model, isStream)
	if err != nil {
		return false, fmt.Errorf("convert Gemini to OpenAI failed: %v", err)
	}

	fmt.Printf("[Gemini->NewAPI] 转换请求格式 (model=%s, stream=%v)\n", model, isStream)

	// 构建 new-api URL
	targetURL := strings.TrimSuffix(prs.newAPIURL, "/") + "/v1/chat/completions"

	// 初始化请求日志
	requestLog := &ReqeustLog{
		TraceID:       traceID,
		RequestID:     c.GetHeader("X-Request-ID"),
		Platform:      "gemini-cli",
		Provider:      "new-api",
		Model:         model,
		IsStream:      isStream,
		UserAgent:     c.GetHeader("User-Agent"),
		ClientIP:      getClientIP(c),
		UserID:        c.GetHeader("X-User-ID"),
		RequestMethod: c.Request.Method,
		RequestPath:   c.Request.URL.Path,
	}

	// Body 日志捕获
	shouldLogBody := prs.IsBodyLogEnabled()
	var responseBuffer bytes.Buffer

	start := time.Now()
	defer func() {
		requestLog.DurationSec = time.Since(start).Seconds()

		// 计算价格
		if prs.pricingService != nil {
			costBreakdown := prs.pricingService.CalculateCost(requestLog.Model, modelpricing.UsageSnapshot{
				InputTokens:       requestLog.InputTokens,
				OutputTokens:      requestLog.OutputTokens,
				CacheCreateTokens: requestLog.CacheCreateTokens,
				CacheReadTokens:   requestLog.CacheReadTokens,
			})
			requestLog.InputCost = costBreakdown.InputCost
			requestLog.OutputCost = costBreakdown.OutputCost
			requestLog.CacheCreateCost = costBreakdown.CacheCreateCost
			requestLog.CacheReadCost = costBreakdown.CacheReadCost
			requestLog.Ephemeral5mCost = costBreakdown.Ephemeral5mCost
			requestLog.Ephemeral1hCost = costBreakdown.Ephemeral1hCost
			requestLog.TotalCost = costBreakdown.TotalCost
		}

		// 发送到写入队列
		select {
		case prs.logWriteQueue <- requestLog:
		default:
			fmt.Printf("[WARN] 日志队列已满，丢弃日志 (trace_id=%s)\n", requestLog.TraceID)
		}

		// Body 日志
		if shouldLogBody && (len(bodyBytes) > 0 || responseBuffer.Len() > 0) {
			bodyLog := &RequestLogBody{
				TraceID:       traceID,
				RequestBody:   string(bodyBytes),
				ResponseBody:  responseBuffer.String(),
				BodySizeBytes: int64(len(bodyBytes) + responseBuffer.Len()),
				CreatedAt:     time.Now(),
				ExpiresAt:     time.Now().Add(7 * 24 * time.Hour),
			}
			select {
			case prs.bodyLogQueue <- bodyLog:
			default:
				fmt.Printf("[WARN] Body log queue full, dropped trace_id=%s\n", traceID)
			}
		}
	}()

	// 创建 HTTP 客户端
	timeout := 60 * time.Second
	if isStream {
		timeout = 300 * time.Second
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// 创建请求
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(openAIBody))
	if err != nil {
		return false, fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+prs.newAPIToken)
	req.Header.Set("X-Trace-ID", traceID)

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		requestLog.ErrorType = "network_error"
		requestLog.ErrorMessage = err.Error()
		return false, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	requestLog.HttpCode = status

	if status >= 200 && status < 300 {
		// 处理流式响应
		if isStream {
			// 设置 SSE 或 Gemini 流式响应头
			if needSSEFormat {
				c.Writer.Header().Set("Content-Type", "text/event-stream")
			} else {
				c.Writer.Header().Set("Content-Type", "application/json")
			}
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Header().Set("X-Trace-ID", traceID)
			c.Writer.WriteHeader(200)

			// 收集所有 Gemini 格式的响应（用于非 SSE 格式）
			geminiChunks := make([]map[string]interface{}, 0)

			reader := bufio.NewReader(resp.Body)
			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						break
					}
					break
				}

				line = bytes.TrimSpace(line)
				if len(line) == 0 {
					continue
				}

				// 解析 SSE data 行
				if bytes.HasPrefix(line, []byte("data: ")) {
					data := bytes.TrimPrefix(line, []byte("data: "))

					// 检查结束标记
					if bytes.Equal(data, []byte("[DONE]")) {
						if needSSEFormat {
							// Gemini SSE 不发送 [DONE]，直接结束
						}
						break
					}

					// 转换 OpenAI chunk 为 Gemini 格式
					geminiChunk, err := convertOpenAIStreamChunkToGemini(data)
					if err != nil {
						continue
					}

					// 记录 Body
					if shouldLogBody {
						responseBuffer.Write(geminiChunk)
						responseBuffer.WriteByte('\n')
					}

					// 提取 token 统计
					if usage := gjson.GetBytes(data, "usage"); usage.Exists() {
						requestLog.InputTokens = int(usage.Get("prompt_tokens").Int())
						requestLog.OutputTokens = int(usage.Get("completion_tokens").Int())
					}

					if needSSEFormat {
						// 发送 SSE 格式
						c.Writer.Write([]byte("data: "))
						c.Writer.Write(geminiChunk)
						c.Writer.Write([]byte("\r\n\r\n"))
						if f, ok := c.Writer.(http.Flusher); ok {
							f.Flush()
						}
					} else {
						// 收集 chunks
						var chunkMap map[string]interface{}
						if err := json.Unmarshal(geminiChunk, &chunkMap); err == nil {
							geminiChunks = append(geminiChunks, chunkMap)
						}
					}
				}
			}

			// 非 SSE 格式：返回 JSON 数组
			if !needSSEFormat {
				arrayBytes, _ := json.Marshal(geminiChunks)
				c.Writer.Write(arrayBytes)
			}

			fmt.Printf("[Gemini->NewAPI] 流式响应完成 (trace_id=%s)\n", traceID)
			return true, nil
		}

		// 非流式响应：转换为 Gemini 格式
		respBody, _ := io.ReadAll(resp.Body)
		geminiResp, err := convertOpenAIToGemini(respBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "convert response failed"})
			return false, err
		}

		// 记录 Body
		if shouldLogBody {
			responseBuffer.Write(geminiResp)
		}

		// 提取 token 统计
		if usage := gjson.GetBytes(respBody, "usage"); usage.Exists() {
			requestLog.InputTokens = int(usage.Get("prompt_tokens").Int())
			requestLog.OutputTokens = int(usage.Get("completion_tokens").Int())
		}

		c.Header("Content-Type", "application/json")
		c.Header("X-Trace-ID", traceID)
		c.Writer.WriteHeader(200)
		c.Writer.Write(geminiResp)

		fmt.Printf("[Gemini->NewAPI] 完成 (trace_id=%s, in=%d, out=%d)\n",
			traceID, requestLog.InputTokens, requestLog.OutputTokens)

		return true, nil
	}

	// 错误响应
	requestLog.ErrorType = classifyHTTPError(status)
	respBody, _ := io.ReadAll(resp.Body)
	requestLog.ErrorMessage = string(respBody)

	fmt.Printf("[ERROR] Gemini->NewAPI error (trace_id=%s, status=%d): %s\n", traceID, status, string(respBody))
	return false, fmt.Errorf("new-api status %d: %s", status, string(respBody))
}

// ============================================================
// LLM Log Configuration and Query Functions
// ============================================================

// LLMLogConfig represents the configuration for LLM logging
type LLMLogConfig struct {
	Enabled         bool   `json:"enabled"`
	StoragePath     string `json:"storage_path"`      // Custom storage path (default: ~/.code-switch)
	SaveFullContent bool   `json:"save_full_content"` // Whether to save full request/response
	RetentionDays   int    `json:"retention_days"`    // Retention period in days (0 = forever)
	MaxFileSizeMB   int    `json:"max_file_size_mb"`  // Max file size in MB
	AutoCleanup     bool   `json:"auto_cleanup"`      // Whether to auto cleanup old logs
}

// LogFilter represents filters for querying logs
type LogFilter struct {
	Platform  string  `json:"platform"`   // claude, codex, gemini-cli
	Model     string  `json:"model"`      // Model name filter
	Provider  string  `json:"provider"`   // Provider name filter
	StartTime string  `json:"start_time"` // ISO 8601 format
	EndTime   string  `json:"end_time"`   // ISO 8601 format
	MinCost   float64 `json:"min_cost"`   // Minimum cost filter
	MaxCost   float64 `json:"max_cost"`   // Maximum cost filter
	HasError  *bool   `json:"has_error"`  // Filter by error status
	Page      int     `json:"page"`       // Page number (1-based)
	PageSize  int     `json:"page_size"`  // Items per page
	SortBy    string  `json:"sort_by"`    // Sort field
	SortOrder string  `json:"sort_order"` // asc or desc
}

// LogQueryResult represents the result of a log query
type LogQueryResult struct {
	Logs       []ReqeustLog `json:"logs"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
	TotalPages int          `json:"total_pages"`
}

// LogDetail represents detailed information about a single log entry
type LogDetail struct {
	Log          ReqeustLog `json:"log"`
	RequestBody  string     `json:"request_body,omitempty"`
	ResponseBody string     `json:"response_body,omitempty"`
}

// LogStatistics represents usage statistics
type LogStatistics struct {
	TotalRequests     int            `json:"total_requests"`
	TotalTokens       int            `json:"total_tokens"`
	TotalInputTokens  int            `json:"total_input_tokens"`
	TotalOutputTokens int            `json:"total_output_tokens"`
	TotalCost         float64        `json:"total_cost"`
	SuccessRate       float64        `json:"success_rate"`
	AvgDuration       float64        `json:"avg_duration_sec"`
	ByPlatform        map[string]int `json:"by_platform"`
	ByModel           map[string]int `json:"by_model"`
	ByProvider        map[string]int `json:"by_provider"`
	Period            string         `json:"period"` // today, week, month, all
}

// GetLLMLogConfig returns the current LLM log configuration
func (prs *ProviderRelayService) GetLLMLogConfig() LLMLogConfig {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".code-switch", "llm-log-settings.json")

	config := LLMLogConfig{
		Enabled:         prs.IsBodyLogEnabled(),
		StoragePath:     filepath.Join(home, ".code-switch"),
		SaveFullContent: prs.IsBodyLogEnabled(),
		RetentionDays:   7,
		MaxFileSizeMB:   10,
		AutoCleanup:     true,
	}

	// Try to load from file
	data, err := os.ReadFile(configPath)
	if err == nil {
		_ = json.Unmarshal(data, &config)
	}

	return config
}

// SetLLMLogConfig sets the LLM log configuration
func (prs *ProviderRelayService) SetLLMLogConfig(config LLMLogConfig) error {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".code-switch", "llm-log-settings.json")

	// Update body log enabled status
	prs.SetBodyLogEnabled(config.SaveFullContent)

	// Save to file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// QueryLogs queries LLM logs with filters
func (prs *ProviderRelayService) QueryLogs(filter LogFilter) (*LogQueryResult, error) {
	// Set defaults
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	if filter.SortBy == "" {
		filter.SortBy = "created_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	// Build query
	where := "1=1"
	args := make([]interface{}, 0)

	if filter.Platform != "" {
		where += " AND platform = ?"
		args = append(args, filter.Platform)
	}
	if filter.Model != "" {
		where += " AND model LIKE ?"
		args = append(args, "%"+filter.Model+"%")
	}
	if filter.Provider != "" {
		where += " AND provider = ?"
		args = append(args, filter.Provider)
	}
	if filter.StartTime != "" {
		where += " AND created_at >= ?"
		args = append(args, filter.StartTime)
	}
	if filter.EndTime != "" {
		where += " AND created_at <= ?"
		args = append(args, filter.EndTime)
	}
	if filter.MinCost > 0 {
		where += " AND total_cost >= ?"
		args = append(args, filter.MinCost)
	}
	if filter.MaxCost > 0 {
		where += " AND total_cost <= ?"
		args = append(args, filter.MaxCost)
	}
	if filter.HasError != nil {
		if *filter.HasError {
			where += " AND http_code >= 400"
		} else {
			where += " AND http_code < 400"
		}
	}

	// Count total
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	var total int
	countSQL := "SELECT COUNT(*) FROM request_log WHERE " + where
	if err := db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Query with pagination
	offset := (filter.Page - 1) * filter.PageSize
	orderBy := filter.SortBy
	if filter.SortOrder == "desc" {
		orderBy += " DESC"
	} else {
		orderBy += " ASC"
	}

	querySQL := fmt.Sprintf(`
		SELECT id, trace_id, request_id, platform, model, provider, http_code,
		       input_tokens, output_tokens, cache_create_tokens, cache_read_tokens,
		       reasoning_tokens, is_stream, duration_sec, user_agent, client_ip,
		       user_id, request_method, request_path, error_type, error_message,
		       provider_error_code, input_cost, output_cost, cache_create_cost,
		       cache_read_cost, ephemeral_5m_cost, ephemeral_1h_cost, total_cost,
		       created_at
		FROM request_log
		WHERE %s
		ORDER BY %s
		LIMIT ? OFFSET ?
	`, where, orderBy)

	args = append(args, filter.PageSize, offset)
	rows, err := db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]ReqeustLog, 0)
	for rows.Next() {
		var log ReqeustLog
		var isStream int
		if err := rows.Scan(
			&log.ID, &log.TraceID, &log.RequestID, &log.Platform, &log.Model, &log.Provider,
			&log.HttpCode, &log.InputTokens, &log.OutputTokens, &log.CacheCreateTokens,
			&log.CacheReadTokens, &log.ReasoningTokens, &isStream, &log.DurationSec,
			&log.UserAgent, &log.ClientIP, &log.UserID, &log.RequestMethod, &log.RequestPath,
			&log.ErrorType, &log.ErrorMessage, &log.ProviderErrorCode, &log.InputCost,
			&log.OutputCost, &log.CacheCreateCost, &log.CacheReadCost, &log.Ephemeral5mCost,
			&log.Ephemeral1hCost, &log.TotalCost, &log.CreatedAt,
		); err != nil {
			continue
		}
		log.IsStream = isStream == 1
		logs = append(logs, log)
	}

	totalPages := (total + filter.PageSize - 1) / filter.PageSize

	return &LogQueryResult{
		Logs:       logs,
		Total:      total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetLogDetail returns detailed information about a single log entry
func (prs *ProviderRelayService) GetLogDetail(traceID string) (*LogDetail, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	// Query main log
	querySQL := `
		SELECT id, trace_id, request_id, platform, model, provider, http_code,
		       input_tokens, output_tokens, cache_create_tokens, cache_read_tokens,
		       reasoning_tokens, is_stream, duration_sec, user_agent, client_ip,
		       user_id, request_method, request_path, error_type, error_message,
		       provider_error_code, input_cost, output_cost, cache_create_cost,
		       cache_read_cost, ephemeral_5m_cost, ephemeral_1h_cost, total_cost,
		       created_at
		FROM request_log
		WHERE trace_id = ?
		LIMIT 1
	`

	var log ReqeustLog
	var isStream int
	if err := db.QueryRow(querySQL, traceID).Scan(
		&log.ID, &log.TraceID, &log.RequestID, &log.Platform, &log.Model, &log.Provider,
		&log.HttpCode, &log.InputTokens, &log.OutputTokens, &log.CacheCreateTokens,
		&log.CacheReadTokens, &log.ReasoningTokens, &isStream, &log.DurationSec,
		&log.UserAgent, &log.ClientIP, &log.UserID, &log.RequestMethod, &log.RequestPath,
		&log.ErrorType, &log.ErrorMessage, &log.ProviderErrorCode, &log.InputCost,
		&log.OutputCost, &log.CacheCreateCost, &log.CacheReadCost, &log.Ephemeral5mCost,
		&log.Ephemeral1hCost, &log.TotalCost, &log.CreatedAt,
	); err != nil {
		return nil, err
	}
	log.IsStream = isStream == 1

	detail := &LogDetail{Log: log}

	// Query body if available
	bodySQL := "SELECT request_body, response_body FROM request_log_body WHERE trace_id = ? LIMIT 1"
	var reqBody, respBody sql.NullString
	if err := db.QueryRow(bodySQL, traceID).Scan(&reqBody, &respBody); err == nil {
		if reqBody.Valid {
			detail.RequestBody = reqBody.String
		}
		if respBody.Valid {
			detail.ResponseBody = respBody.String
		}
	}

	return detail, nil
}

// GetLogStatistics returns usage statistics
func (prs *ProviderRelayService) GetLogStatistics(period string) (*LogStatistics, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	// Determine time range
	var timeFilter string
	switch period {
	case "today":
		timeFilter = "AND date(created_at) = date('now')"
	case "week":
		timeFilter = "AND created_at >= datetime('now', '-7 days')"
	case "month":
		timeFilter = "AND created_at >= datetime('now', '-30 days')"
	default:
		timeFilter = ""
		period = "all"
	}

	stats := &LogStatistics{
		Period:     period,
		ByPlatform: make(map[string]int),
		ByModel:    make(map[string]int),
		ByProvider: make(map[string]int),
	}

	// Aggregate statistics
	aggSQL := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens + output_tokens), 0) as total_tokens,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(AVG(duration_sec), 0) as avg_duration,
			COALESCE(SUM(CASE WHEN http_code < 400 THEN 1 ELSE 0 END) * 100.0 / NULLIF(COUNT(*), 0), 0) as success_rate
		FROM request_log
		WHERE 1=1 %s
	`, timeFilter)

	if err := db.QueryRow(aggSQL).Scan(
		&stats.TotalRequests, &stats.TotalTokens, &stats.TotalInputTokens,
		&stats.TotalOutputTokens, &stats.TotalCost, &stats.AvgDuration, &stats.SuccessRate,
	); err != nil {
		return nil, err
	}

	// Group by platform
	platformSQL := fmt.Sprintf("SELECT platform, COUNT(*) FROM request_log WHERE 1=1 %s GROUP BY platform", timeFilter)
	platformRows, err := db.Query(platformSQL)
	if err == nil {
		defer platformRows.Close()
		for platformRows.Next() {
			var platform string
			var count int
			if platformRows.Scan(&platform, &count) == nil {
				stats.ByPlatform[platform] = count
			}
		}
	}

	// Group by model (top 10)
	modelSQL := fmt.Sprintf("SELECT model, COUNT(*) as cnt FROM request_log WHERE 1=1 %s GROUP BY model ORDER BY cnt DESC LIMIT 10", timeFilter)
	modelRows, err := db.Query(modelSQL)
	if err == nil {
		defer modelRows.Close()
		for modelRows.Next() {
			var model string
			var count int
			if modelRows.Scan(&model, &count) == nil {
				stats.ByModel[model] = count
			}
		}
	}

	// Group by provider
	providerSQL := fmt.Sprintf("SELECT provider, COUNT(*) FROM request_log WHERE 1=1 %s GROUP BY provider", timeFilter)
	providerRows, err := db.Query(providerSQL)
	if err == nil {
		defer providerRows.Close()
		for providerRows.Next() {
			var provider string
			var count int
			if providerRows.Scan(&provider, &count) == nil {
				stats.ByProvider[provider] = count
			}
		}
	}

	return stats, nil
}

// ExportLogs exports logs to a file
func (prs *ProviderRelayService) ExportLogs(filter LogFilter, format string) (string, error) {
	// Query all matching logs (ignore pagination for export)
	filter.Page = 1
	filter.PageSize = 10000 // Max export limit

	result, err := prs.QueryLogs(filter)
	if err != nil {
		return "", err
	}

	// Determine export path
	home, _ := os.UserHomeDir()
	exportDir := filepath.Join(home, ".code-switch", "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102_150405")
	var exportPath string
	var data []byte

	switch format {
	case "csv":
		exportPath = filepath.Join(exportDir, fmt.Sprintf("llm_logs_%s.csv", timestamp))
		data = prs.logsToCSV(result.Logs)
	default: // json
		exportPath = filepath.Join(exportDir, fmt.Sprintf("llm_logs_%s.json", timestamp))
		data, err = json.MarshalIndent(result.Logs, "", "  ")
		if err != nil {
			return "", err
		}
	}

	if err := os.WriteFile(exportPath, data, 0644); err != nil {
		return "", err
	}

	return exportPath, nil
}

// logsToCSV converts logs to CSV format
func (prs *ProviderRelayService) logsToCSV(logs []ReqeustLog) []byte {
	var buf bytes.Buffer

	// Header
	buf.WriteString("ID,TraceID,Platform,Model,Provider,HttpCode,InputTokens,OutputTokens,TotalCost,DurationSec,CreatedAt,ErrorType\n")

	// Data rows
	for _, log := range logs {
		buf.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%d,%d,%d,%.6f,%.3f,%s,%s\n",
			log.ID, log.TraceID, log.Platform, log.Model, log.Provider,
			log.HttpCode, log.InputTokens, log.OutputTokens, log.TotalCost,
			log.DurationSec, log.CreatedAt, log.ErrorType,
		))
	}

	return buf.Bytes()
}

// CleanupOldLogs removes logs older than the specified retention period
func (prs *ProviderRelayService) CleanupOldLogs(retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	db, err := xdb.DB("default")
	if err != nil {
		return 0, err
	}

	// Delete from request_log_body first (foreign key consideration)
	bodySQL := fmt.Sprintf("DELETE FROM request_log_body WHERE created_at < datetime('now', '-%d days')", retentionDays)
	bodyResult, err := db.Exec(bodySQL)
	if err != nil {
		fmt.Printf("[LLM Log] Failed to cleanup body logs: %v\n", err)
	} else {
		bodyDeleted, _ := bodyResult.RowsAffected()
		if bodyDeleted > 0 {
			fmt.Printf("[LLM Log] Cleaned up %d body log entries\n", bodyDeleted)
		}
	}

	// Delete from request_log
	logSQL := fmt.Sprintf("DELETE FROM request_log WHERE created_at < datetime('now', '-%d days')", retentionDays)
	logResult, err := db.Exec(logSQL)
	if err != nil {
		return 0, err
	}

	deleted, _ := logResult.RowsAffected()
	if deleted > 0 {
		fmt.Printf("[LLM Log] Cleaned up %d log entries older than %d days\n", deleted, retentionDays)
	}

	return int(deleted), nil
}
