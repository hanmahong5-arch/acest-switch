package billing

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BillingConfig billing middleware configuration
type BillingConfig struct {
	// Enabled toggle billing check
	Enabled bool `json:"enabled"`
	// MinBalance minimum balance required to make requests (in credits)
	MinBalance int64 `json:"min_balance"`
	// ReportAsync report usage asynchronously
	ReportAsync bool `json:"report_async"`
	// SkipPaths paths that don't require billing check
	SkipPaths []string `json:"skip_paths"`
	// FreeModels models that don't consume credits
	FreeModels []string `json:"free_models"`
	// InputTokenPrice price per 1K input tokens (in credits)
	InputTokenPrice float64 `json:"input_token_price"`
	// OutputTokenPrice price per 1K output tokens (in credits)
	OutputTokenPrice float64 `json:"output_token_price"`
}

// DefaultBillingConfig returns default billing configuration
func DefaultBillingConfig() *BillingConfig {
	return &BillingConfig{
		Enabled:    true,
		MinBalance: 100, // 100 credits minimum
		ReportAsync: true,
		SkipPaths: []string{
			"/health",
			"/api/v1/auth/*",
			"/api/v1/billing/*",
		},
		FreeModels: []string{},
		InputTokenPrice:  0.001,  // 0.001 credits per 1K input tokens
		OutputTokenPrice: 0.003,  // 0.003 credits per 1K output tokens
	}
}

// UsageRecord represents a usage record for billing
type UsageRecord struct {
	TraceID      string    `json:"trace_id"`
	UserID       string    `json:"user_id"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalCost    float64   `json:"total_cost"`
	Timestamp    time.Time `json:"timestamp"`
	Reported     bool      `json:"reported"`
}

// BillingMiddleware handles usage billing
type BillingMiddleware struct {
	lago         *LagoService
	config       *BillingConfig
	usageQueue   chan *UsageRecord
	skipPaths    map[string]bool
	freeModels   map[string]bool
	balanceCache sync.Map // map[userID]*cachedBalance
	mu           sync.RWMutex
}

// cachedBalance represents cached user balance
type cachedBalance struct {
	balance   int64
	expiresAt time.Time
}

// NewBillingMiddleware creates a new billing middleware
func NewBillingMiddleware(lago *LagoService, config *BillingConfig) *BillingMiddleware {
	if config == nil {
		config = DefaultBillingConfig()
	}

	skipPaths := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	freeModels := make(map[string]bool)
	for _, model := range config.FreeModels {
		freeModels[model] = true
	}

	m := &BillingMiddleware{
		lago:       lago,
		config:     config,
		usageQueue: make(chan *UsageRecord, 1000),
		skipPaths:  skipPaths,
		freeModels: freeModels,
	}

	// Start usage reporter goroutine
	if config.ReportAsync {
		go m.processUsageQueue()
	}

	return m
}

// PreRequestHandler checks balance before processing request
func (m *BillingMiddleware) PreRequestHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Enabled {
			c.Next()
			return
		}

		// Check if path should skip billing
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

		// Check if model is free
		model := m.extractModel(c)
		if m.freeModels[model] {
			c.Next()
			return
		}

		// Check user balance
		balance, err := m.getUserBalance(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":   "billing_error",
				"message": "failed to check balance",
			})
			return
		}

		if balance < m.config.MinBalance {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
				"error":   "insufficient_balance",
				"message": "insufficient credits, please recharge",
				"balance": balance,
				"minimum": m.config.MinBalance,
			})
			return
		}

		// Generate trace ID for this request
		traceID := uuid.New().String()
		c.Set("trace_id", traceID)
		c.Set("billing_model", model)
		c.Set("billing_start", time.Now())

		c.Next()
	}
}

// PostRequestHandler reports usage after request completes
func (m *BillingMiddleware) PostRequestHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request first
		c.Next()

		if !m.config.Enabled {
			return
		}

		// Check if path should skip billing
		if m.shouldSkip(c.Request.URL.Path) {
			return
		}

		// Get user ID and trace ID
		userID := GetUserID(c)
		if userID == "" {
			return
		}

		traceID, _ := c.Get("trace_id")
		model, _ := c.Get("billing_model")

		// Check if model is free
		if modelStr, ok := model.(string); ok && m.freeModels[modelStr] {
			return
		}

		// Extract token usage from response
		inputTokens, outputTokens := m.extractTokenUsage(c)
		if inputTokens == 0 && outputTokens == 0 {
			return
		}

		// Calculate cost
		totalCost := m.calculateCost(inputTokens, outputTokens)

		// Create usage record
		record := &UsageRecord{
			TraceID:      traceID.(string),
			UserID:       userID,
			Model:        model.(string),
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalCost:    totalCost,
			Timestamp:    time.Now(),
		}

		// Report usage
		if m.config.ReportAsync {
			select {
			case m.usageQueue <- record:
				// Queued successfully
			default:
				// Queue full, report synchronously
				m.reportUsage(record)
			}
		} else {
			m.reportUsage(record)
		}

		// Invalidate balance cache
		m.balanceCache.Delete(userID)
	}
}

// shouldSkip checks if path should skip billing
func (m *BillingMiddleware) shouldSkip(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.skipPaths[path] {
		return true
	}

	// Check wildcard paths
	for skipPath := range m.skipPaths {
		if len(skipPath) > 0 && skipPath[len(skipPath)-1] == '*' {
			prefix := skipPath[:len(skipPath)-1]
			if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
				return true
			}
		}
	}

	return false
}

// extractModel extracts model name from request
func (m *BillingMiddleware) extractModel(c *gin.Context) string {
	// Try to get from header first
	if model := c.GetHeader("X-Model"); model != "" {
		return model
	}

	// Try to parse from request body
	if c.Request.Body != nil {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil {
			// Restore body for downstream handlers
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			var body map[string]interface{}
			if json.Unmarshal(bodyBytes, &body) == nil {
				if model, ok := body["model"].(string); ok {
					return model
				}
			}
		}
	}

	return "unknown"
}

// extractTokenUsage extracts token usage from response
func (m *BillingMiddleware) extractTokenUsage(c *gin.Context) (inputTokens, outputTokens int) {
	// Try to get from response headers (set by upstream)
	if input := c.Writer.Header().Get("X-Input-Tokens"); input != "" {
		inputTokens, _ = strconv.Atoi(input)
	}
	if output := c.Writer.Header().Get("X-Output-Tokens"); output != "" {
		outputTokens, _ = strconv.Atoi(output)
	}

	// Try to get from context (set by relay handler)
	if inputTokens == 0 {
		if v, exists := c.Get("input_tokens"); exists {
			inputTokens = v.(int)
		}
	}
	if outputTokens == 0 {
		if v, exists := c.Get("output_tokens"); exists {
			outputTokens = v.(int)
		}
	}

	return
}

// calculateCost calculates the cost in credits
func (m *BillingMiddleware) calculateCost(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) / 1000.0 * m.config.InputTokenPrice
	outputCost := float64(outputTokens) / 1000.0 * m.config.OutputTokenPrice
	return inputCost + outputCost
}

// getUserBalance gets user balance from Lago (with cache)
func (m *BillingMiddleware) getUserBalance(userID string) (int64, error) {
	// Check cache first
	if cached, ok := m.balanceCache.Load(userID); ok {
		cb := cached.(*cachedBalance)
		if time.Now().Before(cb.expiresAt) {
			return cb.balance, nil
		}
		m.balanceCache.Delete(userID)
	}

	// Get from Lago
	balance, err := m.lago.GetWalletBalance(userID)
	if err != nil {
		return 0, err
	}

	// Cache the balance
	m.balanceCache.Store(userID, &cachedBalance{
		balance:   balance,
		expiresAt: time.Now().Add(30 * time.Second),
	})

	return balance, nil
}

// reportUsage reports usage to Lago
func (m *BillingMiddleware) reportUsage(record *UsageRecord) {
	err := m.lago.SendLLMUsage(
		record.UserID,
		record.TraceID,
		record.InputTokens,
		record.OutputTokens,
	)
	if err != nil {
		// Log error but don't fail the request
		// In production, consider retry mechanism or dead letter queue
		record.Reported = false
	} else {
		record.Reported = true
	}
}

// processUsageQueue processes usage records asynchronously
func (m *BillingMiddleware) processUsageQueue() {
	batch := make([]*UsageRecord, 0, 10)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case record := <-m.usageQueue:
			batch = append(batch, record)
			if len(batch) >= 10 {
				m.processBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				m.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// processBatch processes a batch of usage records
func (m *BillingMiddleware) processBatch(records []*UsageRecord) {
	events := make([]*LagoEvent, 0, len(records))
	for _, record := range records {
		events = append(events, &LagoEvent{
			TransactionID:      record.TraceID,
			ExternalCustomerID: record.UserID,
			Code:               "llm_tokens",
			Timestamp:          record.Timestamp.Unix(),
			Properties: map[string]interface{}{
				"tokens":        record.InputTokens + record.OutputTokens,
				"input_tokens":  record.InputTokens,
				"output_tokens": record.OutputTokens,
				"model":         record.Model,
			},
		})
	}

	if err := m.lago.SendBatchEvents(events); err != nil {
		// Retry individually on batch failure
		for _, record := range records {
			m.reportUsage(record)
		}
	}
}

// ============================================================
// Configuration Management
// ============================================================

// GetConfig returns current billing configuration
func (m *BillingMiddleware) GetConfig() *BillingConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates billing configuration
func (m *BillingMiddleware) UpdateConfig(config *BillingConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config

	// Update skip paths
	m.skipPaths = make(map[string]bool)
	for _, path := range config.SkipPaths {
		m.skipPaths[path] = true
	}

	// Update free models
	m.freeModels = make(map[string]bool)
	for _, model := range config.FreeModels {
		m.freeModels[model] = true
	}
}

// SetEnabled enables or disables billing
func (m *BillingMiddleware) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Enabled = enabled
}

// AddFreeModel adds a model to free list
func (m *BillingMiddleware) AddFreeModel(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.freeModels[model] = true
	m.config.FreeModels = append(m.config.FreeModels, model)
}

// RemoveFreeModel removes a model from free list
func (m *BillingMiddleware) RemoveFreeModel(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.freeModels, model)

	newModels := make([]string, 0)
	for _, m := range m.config.FreeModels {
		if m != model {
			newModels = append(newModels, m)
		}
	}
	m.config.FreeModels = newModels
}

// ClearBalanceCache clears all cached balances
func (m *BillingMiddleware) ClearBalanceCache() {
	m.balanceCache.Range(func(key, _ interface{}) bool {
		m.balanceCache.Delete(key)
		return true
	})
}

// InvalidateUserBalance invalidates a specific user's balance cache
func (m *BillingMiddleware) InvalidateUserBalance(userID string) {
	m.balanceCache.Delete(userID)
}

// ============================================================
// Quota Check Helpers
// ============================================================

// QuotaInfo represents user's quota information
type QuotaInfo struct {
	UserID       string  `json:"user_id"`
	Balance      int64   `json:"balance"`
	MinBalance   int64   `json:"min_balance"`
	HasQuota     bool    `json:"has_quota"`
	Plan         string  `json:"plan,omitempty"`
	FreeTokens   int64   `json:"free_tokens,omitempty"`
	UsedTokens   int64   `json:"used_tokens,omitempty"`
}

// GetQuotaInfo gets user's quota information
func (m *BillingMiddleware) GetQuotaInfo(userID string) (*QuotaInfo, error) {
	balance, err := m.getUserBalance(userID)
	if err != nil {
		return nil, err
	}

	return &QuotaInfo{
		UserID:     userID,
		Balance:    balance,
		MinBalance: m.config.MinBalance,
		HasQuota:   balance >= m.config.MinBalance,
	}, nil
}

// CheckQuota checks if user has sufficient quota
func (m *BillingMiddleware) CheckQuota(userID string) (bool, error) {
	balance, err := m.getUserBalance(userID)
	if err != nil {
		return false, err
	}
	return balance >= m.config.MinBalance, nil
}

// EstimateCost estimates cost for a request
func (m *BillingMiddleware) EstimateCost(estimatedInputTokens, estimatedOutputTokens int) float64 {
	return m.calculateCost(estimatedInputTokens, estimatedOutputTokens)
}
