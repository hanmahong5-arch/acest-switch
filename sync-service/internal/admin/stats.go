package admin

import (
	"sort"
	"sync"
	"time"
)

// AdminSession represents a session for admin management
type AdminSession struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Title        string    `json:"title"`
	MessageCount int       `json:"message_count"`
	TokenCount   int       `json:"token_count"`
	Cost         float64   `json:"cost"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AdminMessage represents a message for admin view
type AdminMessage struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id"`
	Role         string    `json:"role"`
	Content      string    `json:"content"`
	Model        string    `json:"model,omitempty"`
	TokensInput  int       `json:"tokens_input,omitempty"`
	TokensOutput int       `json:"tokens_output,omitempty"`
	Cost         float64   `json:"cost,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// SessionListResponse represents paginated sessions response
type SessionListResponse struct {
	Sessions []AdminSession `json:"sessions"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// SessionDetailResponse represents session detail with messages
type SessionDetailResponse struct {
	Session  AdminSession   `json:"session"`
	Messages []AdminMessage `json:"messages"`
}

// StatsService 统计分析服务
type StatsService struct {
	mu sync.RWMutex

	// 实时计数器
	totalRequests   int64
	totalTokensIn   int64
	totalTokensOut  int64
	totalCost       float64
	totalErrors     int64

	// 按时间窗口统计
	hourlyStats  map[string]*TimeWindowStats
	dailyStats   map[string]*TimeWindowStats

	// 按维度统计
	providerStats map[string]*ProviderStats
	modelStats    map[string]*ModelStats
	userStats     map[string]*UserStats

	// 会话管理
	sessions       map[string]*AdminSession
	sessionsByUser map[string][]string // userID -> []sessionID
	messages       map[string][]AdminMessage // sessionID -> messages

	// Cache for GetOverview (performance optimization)
	overviewCache     map[string]interface{}
	overviewCacheTime time.Time
	overviewCacheTTL  time.Duration
}

// TimeWindowStats 时间窗口统计
type TimeWindowStats struct {
	Timestamp    time.Time `json:"timestamp"`
	Requests     int64     `json:"requests"`
	TokensIn     int64     `json:"tokens_in"`
	TokensOut    int64     `json:"tokens_out"`
	Cost         float64   `json:"cost"`
	Errors       int64     `json:"errors"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
}

// ProviderStats 供应商统计
type ProviderStats struct {
	Provider     string    `json:"provider"`
	Requests     int64     `json:"requests"`
	TokensIn     int64     `json:"tokens_in"`
	TokensOut    int64     `json:"tokens_out"`
	Cost         float64   `json:"cost"`
	Errors       int64     `json:"errors"`
	SuccessRate  float64   `json:"success_rate"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	LastUsed     time.Time `json:"last_used"`
}

// ModelStats 模型统计
type ModelStats struct {
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	Requests     int64     `json:"requests"`
	TokensIn     int64     `json:"tokens_in"`
	TokensOut    int64     `json:"tokens_out"`
	Cost         float64   `json:"cost"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	LastUsed     time.Time `json:"last_used"`
}

// UserStats 用户统计
type UserStats struct {
	UserID       string    `json:"user_id"`
	Requests     int64     `json:"requests"`
	TokensIn     int64     `json:"tokens_in"`
	TokensOut    int64     `json:"tokens_out"`
	Cost         float64   `json:"cost"`
	Sessions     int64     `json:"sessions"`
	Messages     int64     `json:"messages"`
	LastActive   time.Time `json:"last_active"`
}

// NewStatsService 创建统计服务
func NewStatsService() *StatsService {
	s := &StatsService{
		hourlyStats:      make(map[string]*TimeWindowStats),
		dailyStats:       make(map[string]*TimeWindowStats),
		providerStats:    make(map[string]*ProviderStats),
		modelStats:       make(map[string]*ModelStats),
		userStats:        make(map[string]*UserStats),
		sessions:         make(map[string]*AdminSession),
		sessionsByUser:   make(map[string][]string),
		messages:         make(map[string][]AdminMessage),
		overviewCache:    nil,
		overviewCacheTTL: 5 * time.Second, // Cache overview for 5 seconds
	}

	// 启动清理过期统计的协程
	go s.cleanupLoop()

	return s
}

// RecordRequest 记录请求
func (s *StatsService) RecordRequest(
	userID, provider, model string,
	tokensIn, tokensOut int,
	cost float64,
	latencyMs int,
	isError bool,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Invalidate overview cache on new data
	s.overviewCache = nil

	now := time.Now()
	hourKey := now.Format("2006-01-02-15")
	dayKey := now.Format("2006-01-02")

	// 更新总计
	s.totalRequests++
	s.totalTokensIn += int64(tokensIn)
	s.totalTokensOut += int64(tokensOut)
	s.totalCost += cost
	if isError {
		s.totalErrors++
	}

	// 更新小时统计
	if _, ok := s.hourlyStats[hourKey]; !ok {
		s.hourlyStats[hourKey] = &TimeWindowStats{Timestamp: now.Truncate(time.Hour)}
	}
	hs := s.hourlyStats[hourKey]
	hs.Requests++
	hs.TokensIn += int64(tokensIn)
	hs.TokensOut += int64(tokensOut)
	hs.Cost += cost
	if isError {
		hs.Errors++
	}
	hs.AvgLatencyMs = (hs.AvgLatencyMs*float64(hs.Requests-1) + float64(latencyMs)) / float64(hs.Requests)

	// 更新日统计
	if _, ok := s.dailyStats[dayKey]; !ok {
		s.dailyStats[dayKey] = &TimeWindowStats{Timestamp: now.Truncate(24 * time.Hour)}
	}
	ds := s.dailyStats[dayKey]
	ds.Requests++
	ds.TokensIn += int64(tokensIn)
	ds.TokensOut += int64(tokensOut)
	ds.Cost += cost
	if isError {
		ds.Errors++
	}
	ds.AvgLatencyMs = (ds.AvgLatencyMs*float64(ds.Requests-1) + float64(latencyMs)) / float64(ds.Requests)

	// 更新供应商统计
	if _, ok := s.providerStats[provider]; !ok {
		s.providerStats[provider] = &ProviderStats{Provider: provider}
	}
	ps := s.providerStats[provider]
	ps.Requests++
	ps.TokensIn += int64(tokensIn)
	ps.TokensOut += int64(tokensOut)
	ps.Cost += cost
	if isError {
		ps.Errors++
	}
	ps.SuccessRate = float64(ps.Requests-ps.Errors) / float64(ps.Requests) * 100
	ps.AvgLatencyMs = (ps.AvgLatencyMs*float64(ps.Requests-1) + float64(latencyMs)) / float64(ps.Requests)
	ps.LastUsed = now

	// 更新模型统计
	modelKey := provider + "/" + model
	if _, ok := s.modelStats[modelKey]; !ok {
		s.modelStats[modelKey] = &ModelStats{Model: model, Provider: provider}
	}
	ms := s.modelStats[modelKey]
	ms.Requests++
	ms.TokensIn += int64(tokensIn)
	ms.TokensOut += int64(tokensOut)
	ms.Cost += cost
	ms.AvgLatencyMs = (ms.AvgLatencyMs*float64(ms.Requests-1) + float64(latencyMs)) / float64(ms.Requests)
	ms.LastUsed = now

	// 更新用户统计
	if userID != "" {
		if _, ok := s.userStats[userID]; !ok {
			s.userStats[userID] = &UserStats{UserID: userID}
		}
		us := s.userStats[userID]
		us.Requests++
		us.TokensIn += int64(tokensIn)
		us.TokensOut += int64(tokensOut)
		us.Cost += cost
		us.LastActive = now
	}
}

// RecordSession 记录会话创建
func (s *StatsService) RecordSession(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID != "" {
		if _, ok := s.userStats[userID]; !ok {
			s.userStats[userID] = &UserStats{UserID: userID}
		}
		s.userStats[userID].Sessions++
		s.userStats[userID].LastActive = time.Now()
	}
}

// RecordMessage 记录消息创建
func (s *StatsService) RecordMessage(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID != "" {
		if _, ok := s.userStats[userID]; !ok {
			s.userStats[userID] = &UserStats{UserID: userID}
		}
		s.userStats[userID].Messages++
		s.userStats[userID].LastActive = time.Now()
	}
}

// GetOverview 获取总览统计 (with caching for performance)
func (s *StatsService) GetOverview() map[string]interface{} {
	s.mu.RLock()
	// Check cache first
	if s.overviewCache != nil && time.Since(s.overviewCacheTime) < s.overviewCacheTTL {
		result := s.overviewCache
		s.mu.RUnlock()
		return result
	}
	s.mu.RUnlock()

	// Cache miss or expired, rebuild
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.overviewCache != nil && time.Since(s.overviewCacheTime) < s.overviewCacheTTL {
		return s.overviewCache
	}

	s.overviewCache = map[string]interface{}{
		"total_requests":   s.totalRequests,
		"total_tokens_in":  s.totalTokensIn,
		"total_tokens_out": s.totalTokensOut,
		"total_cost":       s.totalCost,
		"total_errors":     s.totalErrors,
		"success_rate":     s.calculateSuccessRate(),
		"active_users":     len(s.userStats),
		"active_providers": len(s.providerStats),
		"active_models":    len(s.modelStats),
	}
	s.overviewCacheTime = time.Now()

	return s.overviewCache
}

func (s *StatsService) calculateSuccessRate() float64 {
	if s.totalRequests == 0 {
		return 100.0
	}
	return float64(s.totalRequests-s.totalErrors) / float64(s.totalRequests) * 100
}

// GetHourlyStats 获取小时统计（最近24小时）
func (s *StatsService) GetHourlyStats() []*TimeWindowStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*TimeWindowStats, 0, 24)
	now := time.Now()
	for i := 23; i >= 0; i-- {
		key := now.Add(-time.Duration(i) * time.Hour).Format("2006-01-02-15")
		if stat, ok := s.hourlyStats[key]; ok {
			result = append(result, stat)
		} else {
			result = append(result, &TimeWindowStats{
				Timestamp: now.Add(-time.Duration(i) * time.Hour).Truncate(time.Hour),
			})
		}
	}
	return result
}

// GetDailyStats 获取日统计（最近30天）
func (s *StatsService) GetDailyStats() []*TimeWindowStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*TimeWindowStats, 0, 30)
	now := time.Now()
	for i := 29; i >= 0; i-- {
		key := now.AddDate(0, 0, -i).Format("2006-01-02")
		if stat, ok := s.dailyStats[key]; ok {
			result = append(result, stat)
		} else {
			result = append(result, &TimeWindowStats{
				Timestamp: now.AddDate(0, 0, -i).Truncate(24 * time.Hour),
			})
		}
	}
	return result
}

// GetProviderStats 获取供应商统计
func (s *StatsService) GetProviderStats() []*ProviderStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ProviderStats, 0, len(s.providerStats))
	for _, stat := range s.providerStats {
		result = append(result, stat)
	}
	return result
}

// GetModelStats 获取模型统计
func (s *StatsService) GetModelStats() []*ModelStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ModelStats, 0, len(s.modelStats))
	for _, stat := range s.modelStats {
		result = append(result, stat)
	}
	return result
}

// GetUserStats 获取用户统计
func (s *StatsService) GetUserStats() []*UserStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*UserStats, 0, len(s.userStats))
	for _, stat := range s.userStats {
		result = append(result, stat)
	}
	return result
}

// GetUserStatsDetail 获取单个用户统计
func (s *StatsService) GetUserStatsDetail(userID string) *UserStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if stat, ok := s.userStats[userID]; ok {
		return stat
	}
	return nil
}

// cleanupLoop 清理过期统计数据
func (s *StatsService) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

func (s *StatsService) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 清理超过48小时的小时统计
	for key, stat := range s.hourlyStats {
		if now.Sub(stat.Timestamp) > 48*time.Hour {
			delete(s.hourlyStats, key)
		}
	}

	// 清理超过90天的日统计
	for key, stat := range s.dailyStats {
		if now.Sub(stat.Timestamp) > 90*24*time.Hour {
			delete(s.dailyStats, key)
		}
	}
}

// --- Session Management ---

// TrackSession tracks a session for admin monitoring
func (s *StatsService) TrackSession(session AdminSession) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = &session

	// Update user's session list
	userSessions := s.sessionsByUser[session.UserID]
	found := false
	for _, id := range userSessions {
		if id == session.ID {
			found = true
			break
		}
	}
	if !found {
		s.sessionsByUser[session.UserID] = append(userSessions, session.ID)
	}
}

// TrackMessage tracks a message for admin monitoring
func (s *StatsService) TrackMessage(msg AdminMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add message to session
	s.messages[msg.SessionID] = append(s.messages[msg.SessionID], msg)

	// Update session stats
	if session, ok := s.sessions[msg.SessionID]; ok {
		session.MessageCount++
		session.TokenCount += msg.TokensInput + msg.TokensOutput
		session.Cost += msg.Cost
		session.UpdatedAt = time.Now()
	}
}

// GetAllSessions returns paginated list of all sessions
func (s *StatsService) GetAllSessions(page, pageSize int, userIDFilter string) SessionListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*AdminSession

	if userIDFilter != "" {
		// Filter by user
		sessionIDs := s.sessionsByUser[userIDFilter]
		for _, id := range sessionIDs {
			if session, ok := s.sessions[id]; ok {
				filtered = append(filtered, session)
			}
		}
	} else {
		// Get all sessions
		for _, session := range s.sessions {
			filtered = append(filtered, session)
		}
	}

	total := len(filtered)

	// Sort by UpdatedAt descending (most recent first) - O(n log n) instead of O(n²)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})

	// Pagination
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= len(filtered) {
		return SessionListResponse{
			Sessions: []AdminSession{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		}
	}

	if end > len(filtered) {
		end = len(filtered)
	}

	// Convert to value slice
	result := make([]AdminSession, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, *filtered[i])
	}

	return SessionListResponse{
		Sessions: result,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

// GetSessionDetail returns session with its messages
func (s *StatsService) GetSessionDetail(sessionID string) *SessionDetailResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}

	messages := s.messages[sessionID]
	if messages == nil {
		messages = []AdminMessage{}
	}

	return &SessionDetailResponse{
		Session:  *session,
		Messages: messages,
	}
}

// DeleteSession removes a session and its messages
func (s *StatsService) DeleteSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil // Already deleted or doesn't exist
	}

	// Remove from user's session list
	userID := session.UserID
	userSessions := s.sessionsByUser[userID]
	newSessions := make([]string, 0, len(userSessions))
	for _, id := range userSessions {
		if id != sessionID {
			newSessions = append(newSessions, id)
		}
	}
	s.sessionsByUser[userID] = newSessions

	// Delete messages
	delete(s.messages, sessionID)

	// Delete session
	delete(s.sessions, sessionID)

	return nil
}
