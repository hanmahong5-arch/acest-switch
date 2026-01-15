package admin

import (
	"sync"
	"time"
)

// UserManager 用户管理服务
type UserManager struct {
	mu sync.RWMutex

	// 用户状态
	users       map[string]*UserInfo
	disabledUsers map[string]bool
}

// UserInfo 用户信息
type UserInfo struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	IsAdmin      bool      `json:"is_admin"`
	IsDisabled   bool      `json:"is_disabled"`
	CreatedAt    time.Time `json:"created_at"`
	LastLoginAt  time.Time `json:"last_login_at,omitempty"`
	LastActiveAt time.Time `json:"last_active_at,omitempty"`
	LoginCount   int64     `json:"login_count"`
	DeviceCount  int       `json:"device_count"`
	SessionCount int       `json:"session_count"`
	MessageCount int64     `json:"message_count"`
	TotalTokens  int64     `json:"total_tokens"`
	TotalCost    float64   `json:"total_cost"`
}

// UserListResponse 用户列表响应
type UserListResponse struct {
	Users      []*UserInfo `json:"users"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// NewUserManager 创建用户管理器
func NewUserManager() *UserManager {
	return &UserManager{
		users:        make(map[string]*UserInfo),
		disabledUsers: make(map[string]bool),
	}
}

// RegisterUser 注册用户（内部调用）
func (m *UserManager) RegisterUser(userID, username, email string, isAdmin bool) *UserInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, ok := m.users[userID]; ok {
		// 更新现有用户
		user.Username = username
		if email != "" {
			user.Email = email
		}
		return user
	}

	user := &UserInfo{
		UserID:    userID,
		Username:  username,
		Email:     email,
		IsAdmin:   isAdmin,
		CreatedAt: time.Now(),
	}
	m.users[userID] = user
	return user
}

// RecordLogin 记录用户登录
func (m *UserManager) RecordLogin(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, ok := m.users[userID]; ok {
		user.LoginCount++
		user.LastLoginAt = time.Now()
		user.LastActiveAt = time.Now()
	}
}

// RecordActivity 记录用户活动
func (m *UserManager) RecordActivity(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, ok := m.users[userID]; ok {
		user.LastActiveAt = time.Now()
	}
}

// UpdateUserStats 更新用户统计
func (m *UserManager) UpdateUserStats(userID string, tokens int64, cost float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, ok := m.users[userID]; ok {
		user.TotalTokens += tokens
		user.TotalCost += cost
		user.LastActiveAt = time.Now()
	}
}

// UpdateDeviceCount 更新设备数量
func (m *UserManager) UpdateDeviceCount(userID string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, ok := m.users[userID]; ok {
		user.DeviceCount = count
	}
}

// UpdateSessionCount 更新会话数量
func (m *UserManager) UpdateSessionCount(userID string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, ok := m.users[userID]; ok {
		user.SessionCount = count
	}
}

// IncrementMessageCount 增加消息数量
func (m *UserManager) IncrementMessageCount(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, ok := m.users[userID]; ok {
		user.MessageCount++
		user.LastActiveAt = time.Now()
	}
}

// GetUser 获取用户信息
func (m *UserManager) GetUser(userID string) *UserInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if user, ok := m.users[userID]; ok {
		// 返回副本
		info := *user
		info.IsDisabled = m.disabledUsers[userID]
		return &info
	}
	return nil
}

// ListUsers 获取用户列表
func (m *UserManager) ListUsers(page, pageSize int, search string, onlyDisabled bool) *UserListResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 收集符合条件的用户
	var filtered []*UserInfo
	for _, user := range m.users {
		// 搜索过滤
		if search != "" {
			if !contains(user.UserID, search) && !contains(user.Username, search) && !contains(user.Email, search) {
				continue
			}
		}

		// 禁用状态过滤
		isDisabled := m.disabledUsers[user.UserID]
		if onlyDisabled && !isDisabled {
			continue
		}

		info := *user
		info.IsDisabled = isDisabled
		filtered = append(filtered, &info)
	}

	total := len(filtered)
	totalPages := (total + pageSize - 1) / pageSize

	// 分页
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		return &UserListResponse{
			Users:      []*UserInfo{},
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		}
	}
	if end > total {
		end = total
	}

	return &UserListResponse{
		Users:      filtered[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// DisableUser 禁用用户
func (m *UserManager) DisableUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabledUsers[userID] = true
	if user, ok := m.users[userID]; ok {
		user.IsDisabled = true
	}
	return nil
}

// EnableUser 启用用户
func (m *UserManager) EnableUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.disabledUsers, userID)
	if user, ok := m.users[userID]; ok {
		user.IsDisabled = false
	}
	return nil
}

// IsUserDisabled 检查用户是否被禁用
func (m *UserManager) IsUserDisabled(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.disabledUsers[userID]
}

// SetUserAdmin 设置管理员权限
func (m *UserManager) SetUserAdmin(userID string, isAdmin bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, ok := m.users[userID]; ok {
		user.IsAdmin = isAdmin
		return nil
	}
	return nil
}

// GetActiveUsersCount 获取活跃用户数（最近24小时）
func (m *UserManager) GetActiveUsersCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	threshold := time.Now().Add(-24 * time.Hour)
	count := 0
	for _, user := range m.users {
		if user.LastActiveAt.After(threshold) {
			count++
		}
	}
	return count
}

// GetOnlineUsersCount 获取在线用户数（最近5分钟）
func (m *UserManager) GetOnlineUsersCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	threshold := time.Now().Add(-5 * time.Minute)
	count := 0
	for _, user := range m.users {
		if user.LastActiveAt.After(threshold) {
			count++
		}
	}
	return count
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
