package models

import (
	"time"
)

// User 用户信息
type User struct {
	ID           string    `json:"id"`
	NewAPIUserID string    `json:"newapi_user_id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Plan         string    `json:"plan"`
	QuotaTotal   float64   `json:"quota_total"`
	QuotaUsed    float64   `json:"quota_used"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
}

// Device 设备信息
type Device struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	DeviceID      string    `json:"device_id"`
	DeviceName    string    `json:"device_name"`
	DeviceType    string    `json:"device_type"` // desktop, mobile, cli, web
	ClientVersion string    `json:"client_version"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	LastIP        string    `json:"last_ip"`
}

// Session 会话
type Session struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary,omitempty"`
	Model         string    `json:"model,omitempty"`
	Provider      string    `json:"provider,omitempty"`
	MessageCount  int       `json:"message_count"`
	TokenCount    int       `json:"token_count"`
	Cost          float64   `json:"cost"`
	IsPinned      bool      `json:"is_pinned"`
	IsArchived    bool      `json:"is_archived"`
	LastMessageAt time.Time `json:"last_message_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Message 消息
type Message struct {
	ID              string                 `json:"id"`
	SessionID       string                 `json:"session_id"`
	UserID          string                 `json:"user_id"`
	Role            string                 `json:"role"` // user, assistant, system
	Content         string                 `json:"content"`
	ContentType     string                 `json:"content_type"` // text, markdown, code
	Model           string                 `json:"model,omitempty"`
	Provider        string                 `json:"provider,omitempty"`
	TokensInput     int                    `json:"tokens_input,omitempty"`
	TokensOutput    int                    `json:"tokens_output,omitempty"`
	TokensReasoning int                    `json:"tokens_reasoning,omitempty"`
	Cost            float64                `json:"cost,omitempty"`
	DurationMs      int                    `json:"duration_ms,omitempty"`
	FinishReason    string                 `json:"finish_reason,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// PresenceStatus 在线状态
type PresenceStatus string

const (
	PresenceOnline  PresenceStatus = "online"
	PresenceOffline PresenceStatus = "offline"
	PresenceAway    PresenceStatus = "away"
)

// Presence 在线状态信息
type Presence struct {
	UserID        string         `json:"user_id"`
	DeviceID      string         `json:"device_id"`
	DeviceType    string         `json:"device_type"`
	Status        PresenceStatus `json:"status"`
	ClientVersion string         `json:"client_version"`
	LastSeenAt    time.Time      `json:"last_seen_at"`
}

// SessionStatus 会话状态
type SessionStatusType string

const (
	SessionIdle      SessionStatusType = "idle"
	SessionThinking  SessionStatusType = "thinking"
	SessionStreaming SessionStatusType = "streaming"
	SessionError     SessionStatusType = "error"
	SessionCompleted SessionStatusType = "completed"
)

// SessionStatusEvent 会话状态事件
type SessionStatusEvent struct {
	SessionID string            `json:"session_id"`
	UserID    string            `json:"user_id"`
	Status    SessionStatusType `json:"status"`
	Model     string            `json:"model,omitempty"`
	Progress  int               `json:"progress,omitempty"` // 0-100
	Timestamp time.Time         `json:"timestamp"`
}

// TypingEvent 输入状态事件
type TypingEvent struct {
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	IsTyping  bool      `json:"is_typing"`
	Timestamp time.Time `json:"timestamp"`
}
