package models

import (
	"time"
)

// EventType 事件类型
type EventType string

const (
	// 用户事件
	EventUserLogin      EventType = "user.login"
	EventUserLogout     EventType = "user.logout"
	EventUserPresence   EventType = "user.presence"
	EventUserQuotaLow   EventType = "user.quota_low"

	// 会话事件
	EventSessionCreated  EventType = "session.created"
	EventSessionUpdated  EventType = "session.updated"
	EventSessionDeleted  EventType = "session.deleted"
	EventSessionArchived EventType = "session.archived"

	// 消息事件
	EventMessageCreated EventType = "message.created"
	EventMessageUpdated EventType = "message.updated"
	EventMessageDeleted EventType = "message.deleted"

	// 状态事件
	EventSessionStatus EventType = "session.status"
	EventTyping        EventType = "typing"

	// 系统事件
	EventSystemBroadcast EventType = "system.broadcast"
	EventSystemError     EventType = "system.error"
)

// BaseEvent 基础事件结构
type BaseEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	TraceID   string    `json:"trace_id,omitempty"`
}

// NewBaseEvent 创建基础事件
func NewBaseEvent(eventType EventType) BaseEvent {
	return BaseEvent{
		ID:        GenerateID(),
		Type:      eventType,
		Timestamp: time.Now(),
	}
}

// UserEvent 用户事件
type UserEvent struct {
	BaseEvent
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id,omitempty"`
	Data     any    `json:"data,omitempty"`
}

// SessionEvent 会话事件
type SessionEvent struct {
	BaseEvent
	UserID    string   `json:"user_id"`
	SessionID string   `json:"session_id"`
	Session   *Session `json:"session,omitempty"`
}

// MessageEvent 消息事件
type MessageEvent struct {
	BaseEvent
	UserID    string   `json:"user_id"`
	SessionID string   `json:"session_id"`
	MessageID string   `json:"message_id"`
	Message   *Message `json:"message,omitempty"`
}

// SyncRequest 同步请求
type SyncRequest struct {
	UserID       string `json:"user_id"`
	DeviceID     string `json:"device_id"`
	LastSyncTime int64  `json:"last_sync_time"` // Unix timestamp
	LastMsgID    string `json:"last_msg_id,omitempty"`
}

// SyncResponse 同步响应
type SyncResponse struct {
	Sessions      []Session  `json:"sessions"`
	Messages      []Message  `json:"messages"`
	DeletedIDs    []string   `json:"deleted_ids,omitempty"`
	ServerTime    int64      `json:"server_time"`
	HasMore       bool       `json:"has_more"`
	NextCursor    string     `json:"next_cursor,omitempty"`
}

// AuthRequest 认证请求
type AuthRequest struct {
	Token      string `json:"token"`       // NEW-API Token
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
	ClientVersion string `json:"client_version"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	User         *User  `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// GenerateID 生成 UUID
func GenerateID() string {
	// 简单实现，实际使用 uuid 包
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
