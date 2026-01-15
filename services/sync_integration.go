package services

import (
	"codeswitch/services/sync"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// SyncIntegration 同步集成
type SyncIntegration struct {
	syncService *sync.SyncService
	enabled     bool
}

// 全局同步集成实例
var globalSyncIntegration *SyncIntegration

// InitSyncIntegration 初始化同步集成
func InitSyncIntegration(syncSettings *SyncSettingsService) {
	if syncSettings == nil {
		return
	}

	globalSyncIntegration = &SyncIntegration{
		syncService: syncSettings.GetSyncService(),
		enabled:     syncSettings.settings.IsEnabled(),
	}
}

// GetSyncIntegration 获取同步集成实例
func GetSyncIntegration() *SyncIntegration {
	return globalSyncIntegration
}

// IsEnabled 检查是否启用
func (si *SyncIntegration) IsEnabled() bool {
	return si != nil && si.enabled && si.syncService != nil && si.syncService.IsEnabled()
}

// StartLLMConsumer 启动 LLM 请求消费者
func (si *SyncIntegration) StartLLMConsumer(handler sync.LLMRequestHandler) error {
	if !si.IsEnabled() {
		return nil
	}
	return si.syncService.StartLLMRequestConsumer(handler)
}

// --- 同步钩子方法 ---

// OnRequestStart 请求开始时调用
func (si *SyncIntegration) OnRequestStart(c *gin.Context, kind, model, provider string, isStream bool, traceID string) {
	if !si.IsEnabled() {
		return
	}

	userID, sessionID := extractUserSession(c)
	if userID == "" {
		return
	}

	// 发布会话状态：思考中 (non-terminal state, no dedup needed)
	si.syncService.PublishSessionStatus(userID, sessionID, "thinking", model, provider, traceID)

	// 发布 LLM 请求事件
	si.syncService.PublishLLMRequest(&sync.LLMRequestEvent{
		TraceID:   traceID,
		UserID:    userID,
		SessionID: sessionID,
		Platform:  kind,
		Provider:  provider,
		Model:     model,
		IsStream:  isStream,
	})
}

// OnStreamStart 流式响应开始时调用
func (si *SyncIntegration) OnStreamStart(c *gin.Context, kind, model, provider, traceID string) {
	if !si.IsEnabled() {
		return
	}

	userID, sessionID := extractUserSession(c)
	if userID == "" {
		return
	}

	// 发布会话状态：流式输出中 (non-terminal state, no dedup needed)
	si.syncService.PublishSessionStatus(userID, sessionID, "streaming", model, provider, traceID)
}

// OnRequestComplete 请求完成时调用
func (si *SyncIntegration) OnRequestComplete(
	c *gin.Context,
	kind string,
	provider string,
	model string,
	traceID string,
	success bool,
	httpCode int,
	errorMsg string,
	tokensInput int,
	tokensOutput int,
	cost float64,
	durationMs int,
) {
	if !si.IsEnabled() {
		return
	}

	userID, sessionID := extractUserSession(c)
	if userID == "" {
		return
	}

	// 发布会话状态：完成或错误 (terminal state, dedup by traceID)
	status := "completed"
	if !success {
		status = "error"
	}
	si.syncService.PublishSessionStatus(userID, sessionID, status, model, provider, traceID)

	// 发布 LLM 响应事件
	si.syncService.PublishLLMResponse(&sync.LLMResponseEvent{
		TraceID:      traceID,
		UserID:       userID,
		SessionID:    sessionID,
		Platform:     kind,
		Provider:     provider,
		Model:        model,
		Success:      success,
		HTTPCode:     httpCode,
		ErrorMessage: errorMsg,
		TokensInput:  tokensInput,
		TokensOutput: tokensOutput,
		Cost:         cost,
		DurationMs:   durationMs,
	})

	// 如果成功，发布助手消息
	if success {
		// 注意：实际内容需要从响应中提取，这里简化处理
		si.syncService.PublishAssistantMessage(userID, sessionID, &sync.ChatMessage{
			ID:           traceID,
			SessionID:    sessionID,
			UserID:       userID,
			Role:         "assistant",
			Content:      "", // 流式响应内容需要累积
			Model:        model,
			Provider:     provider,
			TokensInput:  tokensInput,
			TokensOutput: tokensOutput,
			Cost:         cost,
			DurationMs:   durationMs,
			CreatedAt:    time.Now(),
		})
	}
}

// OnUserMessage 用户发送消息时调用
func (si *SyncIntegration) OnUserMessage(c *gin.Context, bodyBytes []byte) {
	if !si.IsEnabled() {
		return
	}

	userID, sessionID := extractUserSession(c)
	if userID == "" {
		return
	}

	// 提取用户消息内容
	content := extractUserContent(bodyBytes)
	if content == "" {
		return
	}

	si.syncService.PublishUserMessage(userID, sessionID, &sync.ChatMessage{
		ID:        generateSyncTraceID(),
		SessionID: sessionID,
		UserID:    userID,
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	})
}

// OnQuotaChange 配额变更时调用
// quotaTotal, quotaUsed: NEW-API 配额（1 配额 = 0.0001 USD）
func (si *SyncIntegration) OnQuotaChange(
	userID string,
	quotaTotal float64,
	quotaUsed float64,
	lastCost float64,
	model string,
	traceID string,
) {
	if !si.IsEnabled() {
		return
	}

	if userID == "" {
		return
	}

	si.syncService.PublishQuotaChange(&sync.QuotaChangeEvent{
		UserID:      userID,
		QuotaTotal:  quotaTotal,
		QuotaUsed:   quotaUsed,
		QuotaRemain: quotaTotal - quotaUsed,
		LastCost:    lastCost,
		Model:       model,
		TraceID:     traceID,
	})
}

// --- 辅助方法 ---

// extractUserSession 从请求头中提取用户和会话信息
func extractUserSession(c *gin.Context) (userID, sessionID string) {
	// 支持多种头格式
	userID = c.GetHeader("X-User-ID")
	if userID == "" {
		userID = c.GetHeader("X-Codeswitch-User-ID")
	}

	sessionID = c.GetHeader("X-Session-ID")
	if sessionID == "" {
		sessionID = c.GetHeader("X-Codeswitch-Session-ID")
	}

	// 如果没有 session ID，使用默认值
	if sessionID == "" && userID != "" {
		sessionID = "default"
	}

	return
}

// extractUserContent 从请求体中提取用户消息内容
func extractUserContent(bodyBytes []byte) string {
	// Claude 格式
	if content := gjson.GetBytes(bodyBytes, "messages.#(role==\"user\").content").String(); content != "" {
		return content
	}

	// 获取最后一条 user 消息
	messages := gjson.GetBytes(bodyBytes, "messages")
	if messages.Exists() && messages.IsArray() {
		var lastUserContent string
		messages.ForEach(func(_, value gjson.Result) bool {
			if value.Get("role").String() == "user" {
				content := value.Get("content")
				if content.IsArray() {
					// 处理多部分内容
					content.ForEach(func(_, part gjson.Result) bool {
						if part.Get("type").String() == "text" {
							lastUserContent = part.Get("text").String()
						}
						return true
					})
				} else {
					lastUserContent = content.String()
				}
			}
			return true
		})
		if lastUserContent != "" {
			return lastUserContent
		}
	}

	// Codex 格式
	if content := gjson.GetBytes(bodyBytes, "input").String(); content != "" {
		return content
	}

	// Gemini 格式
	if content := gjson.GetBytes(bodyBytes, "contents.0.parts.0.text").String(); content != "" {
		return content
	}

	return ""
}

// generateSyncTraceID 生成同步追踪 ID
func generateSyncTraceID() string {
	return strings.ReplaceAll(time.Now().Format("20060102150405.000000"), ".", "-")
}
