package sync

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// publishedTraceCache stores trace IDs to prevent duplicate publishing
// Uses sync.Map for concurrent access, with TTL cleanup
type publishedTraceCache struct {
	cache sync.Map // map[traceID]time.Time (publish time)
}

// hasPublished checks if a trace ID has been published, returns true if already published
func (c *publishedTraceCache) hasPublished(traceID, eventType string) bool {
	if traceID == "" {
		return false
	}
	key := traceID + ":" + eventType
	_, exists := c.cache.Load(key)
	return exists
}

// markPublished marks a trace ID as published
func (c *publishedTraceCache) markPublished(traceID, eventType string) {
	if traceID == "" {
		return
	}
	key := traceID + ":" + eventType
	c.cache.Store(key, time.Now())
}

// cleanup removes entries older than the given duration
func (c *publishedTraceCache) cleanup(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	c.cache.Range(func(key, value interface{}) bool {
		if t, ok := value.(time.Time); ok && t.Before(cutoff) {
			c.cache.Delete(key)
		}
		return true
	})
}

// SyncService 同步服务
type SyncService struct {
	nats         *NATSClient
	config       *SyncConfig
	publishCache *publishedTraceCache // Deduplication cache
}

// SyncConfig 同步配置
type SyncConfig struct {
	NATSConfig    *NATSConfig
	SyncServerURL string // Sync Service API URL
}

// NewSyncService 创建同步服务
func NewSyncService(cfg *SyncConfig) *SyncService {
	if cfg == nil {
		cfg = &SyncConfig{
			NATSConfig: DefaultNATSConfig(),
		}
	}

	return &SyncService{
		nats:         NewNATSClient(cfg.NATSConfig),
		config:       cfg,
		publishCache: &publishedTraceCache{},
	}
}

// Start 启动同步服务
func (s *SyncService) Start() error {
	if err := s.nats.Connect(); err != nil {
		return err
	}

	// Start cache cleanup goroutine (clean entries older than 5 minutes)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if s.publishCache != nil {
				s.publishCache.cleanup(5 * time.Minute)
			}
		}
	}()

	return nil
}

// Stop 停止同步服务
func (s *SyncService) Stop() error {
	return s.nats.Close()
}

// IsEnabled 检查是否启用
func (s *SyncService) IsEnabled() bool {
	return s.nats.IsEnabled() && s.nats.IsConnected()
}

// --- 消息事件 ---

// MessageEvent 消息事件
type MessageEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	TraceID   string                 `json:"trace_id,omitempty"`
	UserID    string                 `json:"user_id"`
	SessionID string                 `json:"session_id"`
	MessageID string                 `json:"message_id,omitempty"`
	Message   *ChatMessage           `json:"message,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	ID              string  `json:"id"`
	SessionID       string  `json:"session_id"`
	UserID          string  `json:"user_id"`
	Role            string  `json:"role"` // user, assistant
	Content         string  `json:"content"`
	Model           string  `json:"model,omitempty"`
	Provider        string  `json:"provider,omitempty"`
	TokensInput     int     `json:"tokens_input,omitempty"`
	TokensOutput    int     `json:"tokens_output,omitempty"`
	Cost            float64 `json:"cost,omitempty"`
	DurationMs      int     `json:"duration_ms,omitempty"`
	IsStream        bool    `json:"is_stream,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// SessionStatusEvent 会话状态事件
type SessionStatusEvent struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Status    string `json:"status"` // idle, thinking, streaming, completed, error
	Model     string `json:"model,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Progress  int    `json:"progress,omitempty"` // 0-100
	Timestamp time.Time `json:"timestamp"`
}

// LLMRequestEvent LLM 请求事件
type LLMRequestEvent struct {
	ID        string    `json:"id"`
	TraceID   string    `json:"trace_id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Platform  string    `json:"platform"` // claude, codex, gemini
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	IsStream  bool      `json:"is_stream"`
	Timestamp time.Time `json:"timestamp"`
}

// LLMResponseEvent LLM 响应事件
type LLMResponseEvent struct {
	ID           string    `json:"id"`
	TraceID      string    `json:"trace_id"`
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	Platform     string    `json:"platform"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Success      bool      `json:"success"`
	HTTPCode     int       `json:"http_code"`
	ErrorMessage string    `json:"error_message,omitempty"`
	TokensInput  int       `json:"tokens_input,omitempty"`
	TokensOutput int       `json:"tokens_output,omitempty"`
	Cost         float64   `json:"cost,omitempty"`
	DurationMs   int       `json:"duration_ms"`
	Timestamp    time.Time `json:"timestamp"`
}

// QuotaChangeEvent 配额变更事件
type QuotaChangeEvent struct {
	UserID      string    `json:"user_id"`
	QuotaTotal  float64   `json:"quota_total"`
	QuotaUsed   float64   `json:"quota_used"`
	QuotaRemain float64   `json:"quota_remain"`
	LastCost    float64   `json:"last_cost"`         // 本次消耗
	Model       string    `json:"model,omitempty"`   // 消耗的模型
	TraceID     string    `json:"trace_id,omitempty"` // 关联的请求 ID
	Timestamp   time.Time `json:"timestamp"`
}

// --- 发布方法 ---

// PublishUserMessage 发布用户消息 (idempotent - only publishes once per message ID)
func (s *SyncService) PublishUserMessage(userID, sessionID string, msg *ChatMessage) error {
	if !s.IsEnabled() {
		return nil
	}

	// Deduplication: skip if already published for this message ID
	if s.publishCache.hasPublished(msg.ID, "user_msg") {
		return nil
	}

	event := &MessageEvent{
		ID:        generateID(),
		Type:      "message.user",
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		MessageID: msg.ID,
		Message:   msg,
	}

	subject := SessionSubject(userID, sessionID, "msg")
	if err := s.nats.PublishWithAck(subject, event); err != nil {
		return err
	}

	// Mark as published after successful send
	s.publishCache.markPublished(msg.ID, "user_msg")
	return nil
}

// PublishAssistantMessage 发布助手消息 (idempotent - only publishes once per message ID)
func (s *SyncService) PublishAssistantMessage(userID, sessionID string, msg *ChatMessage) error {
	if !s.IsEnabled() {
		return nil
	}

	// Deduplication: skip if already published for this message ID (traceID)
	if s.publishCache.hasPublished(msg.ID, "assistant_msg") {
		return nil
	}

	event := &MessageEvent{
		ID:        generateID(),
		Type:      "message.assistant",
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		MessageID: msg.ID,
		Message:   msg,
	}

	subject := SessionSubject(userID, sessionID, "msg")
	if err := s.nats.PublishWithAck(subject, event); err != nil {
		return err
	}

	// Mark as published after successful send
	s.publishCache.markPublished(msg.ID, "assistant_msg")
	return nil
}

// PublishSessionStatus 发布会话状态
// For terminal states (completed/error), only publishes once per traceID
func (s *SyncService) PublishSessionStatus(userID, sessionID, status, model, provider, traceID string) error {
	if !s.IsEnabled() {
		return nil
	}

	// For terminal states, deduplicate by traceID to prevent duplicate "completed" events
	// Each request (identified by traceID) should only emit one terminal status
	if status == "completed" || status == "error" {
		if traceID != "" && s.publishCache.hasPublished(traceID, "session_status_"+status) {
			fmt.Printf("[Sync] Skipping duplicate SessionStatus '%s' (trace_id=%s)\n", status, traceID)
			return nil
		}
		if traceID != "" {
			defer s.publishCache.markPublished(traceID, "session_status_"+status)
		}
	}

	event := &SessionStatusEvent{
		SessionID: sessionID,
		UserID:    userID,
		Status:    status,
		Model:     model,
		Provider:  provider,
		Timestamp: time.Now(),
	}

	subject := SessionSubject(userID, sessionID, "status")
	if err := s.nats.Publish(subject, event); err != nil {
		fmt.Printf("[Sync] Failed to publish SessionStatus '%s' (trace_id=%s): %v\n", status, traceID, err)
		return err
	}
	fmt.Printf("[Sync] Published SessionStatus '%s' (trace_id=%s, session=%s)\n", status, traceID, sessionID)
	return nil
}

// PublishLLMRequest 发布 LLM 请求事件 (idempotent - only publishes once per traceID)
func (s *SyncService) PublishLLMRequest(req *LLMRequestEvent) error {
	if !s.IsEnabled() {
		return nil
	}

	// Deduplication: skip if already published for this traceID
	if s.publishCache.hasPublished(req.TraceID, "llm_request") {
		return nil
	}

	req.ID = generateID()
	req.Timestamp = time.Now()

	subject := LLMRequestSubject(req.Platform)
	if err := s.nats.Publish(subject, req); err != nil {
		return err
	}

	// Mark as published after successful send
	s.publishCache.markPublished(req.TraceID, "llm_request")
	return nil
}

// PublishLLMResponse 发布 LLM 响应事件 (idempotent - only publishes once per traceID)
func (s *SyncService) PublishLLMResponse(resp *LLMResponseEvent) error {
	if !s.IsEnabled() {
		return nil
	}

	// Deduplication: skip if already published for this traceID
	if s.publishCache.hasPublished(resp.TraceID, "llm_response") {
		fmt.Printf("[Sync] Skipping duplicate LLMResponse (trace_id=%s)\n", resp.TraceID)
		return nil
	}

	resp.ID = generateID()
	resp.Timestamp = time.Now()

	subject := LLMResponseSubject(resp.TraceID)
	if err := s.nats.Publish(subject, resp); err != nil {
		fmt.Printf("[Sync] Failed to publish LLMResponse (trace_id=%s): %v\n", resp.TraceID, err)
		return err
	}

	// Mark as published after successful send
	s.publishCache.markPublished(resp.TraceID, "llm_response")
	fmt.Printf("[Sync] Published LLMResponse (trace_id=%s, success=%v)\n", resp.TraceID, resp.Success)
	return nil
}

// PublishQuotaChange 发布配额变更事件 (idempotent - only publishes once per traceID)
func (s *SyncService) PublishQuotaChange(event *QuotaChangeEvent) error {
	if !s.IsEnabled() {
		return nil
	}

	// Deduplication: skip if already published for this traceID (if provided)
	if event.TraceID != "" && s.publishCache.hasPublished(event.TraceID, "quota_change") {
		return nil
	}

	event.Timestamp = time.Now()

	subject := QuotaSubject(event.UserID)
	if err := s.nats.Publish(subject, event); err != nil {
		return err
	}

	// Mark as published after successful send
	if event.TraceID != "" {
		s.publishCache.markPublished(event.TraceID, "quota_change")
	}
	return nil
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Nanosecond()%10000)
}

// --- NATS LLM 消费者 ---

// LLMRequestHandler LLM 请求处理器回调
// 返回 LLMResponseEvent 和错误
type LLMRequestHandler func(req *LLMNATSRequest) (*LLMNATSResponse, error)

// LLMNATSRequest NATS LLM 请求
type LLMNATSRequest struct {
	TraceID   string                 `json:"trace_id"`
	UserID    string                 `json:"user_id"`
	SessionID string                 `json:"session_id,omitempty"`
	Platform  string                 `json:"platform"` // claude, codex, gemini
	Model     string                 `json:"model"`
	Messages  []map[string]interface{} `json:"messages"`
	Stream    bool                   `json:"stream"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// LLMNATSResponse NATS LLM 响应
type LLMNATSResponse struct {
	TraceID      string  `json:"trace_id"`
	Success      bool    `json:"success"`
	Model        string  `json:"model"`
	Content      string  `json:"content"`
	TokensInput  int     `json:"tokens_input"`
	TokensOutput int     `json:"tokens_output"`
	Cost         float64 `json:"cost"`
	DurationMs   int     `json:"duration_ms"`
	Error        string  `json:"error,omitempty"`
}

// StartLLMRequestConsumer 启动 LLM 请求消费者
// 订阅 llm.request.* 主题，调用 handler 处理请求
// NOTE: Response is returned via Request-Reply pattern only, no separate Publish to avoid duplication
func (s *SyncService) StartLLMRequestConsumer(handler LLMRequestHandler) error {
	if !s.IsEnabled() {
		return nil
	}

	// Track if already started to prevent duplicate subscriptions
	if s.publishCache.hasPublished("consumer", "llm_request_started") {
		fmt.Println("[Sync] LLM request consumer already started, skipping")
		return nil
	}
	s.publishCache.markPublished("consumer", "llm_request_started")

	// 订阅所有平台的 LLM 请求
	subjects := []string{"llm.request.claude", "llm.request.codex", "llm.request.gemini"}

	for _, subject := range subjects {
		if err := s.nats.SubscribeWithHandler(subject, func(data []byte) []byte {
			var req LLMNATSRequest
			if err := json.Unmarshal(data, &req); err != nil {
				errResp := &LLMNATSResponse{
					Success: false,
					Error:   fmt.Sprintf("invalid request: %v", err),
				}
				respData, _ := json.Marshal(errResp)
				return respData
			}

			// Deduplication: check if this request was already processed
			if req.TraceID != "" && s.publishCache.hasPublished(req.TraceID, "llm_request_processed") {
				fmt.Printf("[Sync] Duplicate request detected (trace_id=%s), returning cached response\n", req.TraceID)
				// Return empty response for duplicate requests
				return nil
			}

			// 调用处理器
			resp, err := handler(&req)
			if err != nil {
				resp = &LLMNATSResponse{
					TraceID: req.TraceID,
					Success: false,
					Error:   err.Error(),
				}
			}

			// Mark request as processed
			if req.TraceID != "" {
				s.publishCache.markPublished(req.TraceID, "llm_request_processed")
			}

			// Return response via Request-Reply pattern only
			// DO NOT publish separately to avoid duplicate messages
			respData, _ := json.Marshal(resp)
			return respData
		}); err != nil {
			return fmt.Errorf("failed to subscribe %s: %w", subject, err)
		}
		fmt.Printf("[Sync] Subscribed to %s\n", subject)
	}

	return nil
}
