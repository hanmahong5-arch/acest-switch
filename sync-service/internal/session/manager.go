package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/internal/nats"
	"github.com/aspect-code/codeswitch/sync-service/pkg/models"
	"github.com/google/uuid"
)

// Manager 会话管理器
type Manager struct {
	natsClient *nats.Client
	logger     *slog.Logger

	// 内存存储 (生产环境应使用 PostgreSQL)
	sessions sync.Map // map[sessionID]*models.Session
	userSessions sync.Map // map[userID][]string (sessionIDs)
	messages sync.Map // map[sessionID][]*models.Message
}

// NewManager 创建会话管理器
func NewManager(natsClient *nats.Client, logger *slog.Logger) *Manager {
	return &Manager{
		natsClient: natsClient,
		logger:     logger,
	}
}

// CreateSession 创建会话
func (m *Manager) CreateSession(ctx context.Context, userID, title string) (*models.Session, error) {
	session := &models.Session{
		ID:           uuid.New().String(),
		UserID:       userID,
		Title:        title,
		MessageCount: 0,
		TokenCount:   0,
		Cost:         0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 存储会话
	m.sessions.Store(session.ID, session)

	// 更新用户会话列表
	m.addUserSession(userID, session.ID)

	// 发布会话创建事件
	event := &models.SessionEvent{
		BaseEvent: models.NewBaseEvent(models.EventSessionCreated),
		UserID:    userID,
		SessionID: session.ID,
		Session:   session,
	}

	if err := m.natsClient.Publish(nats.UserSessionsSubject(userID), event); err != nil {
		m.logger.Error("Failed to publish session created event", "error", err)
	}

	m.logger.Info("Session created", "session_id", session.ID, "user_id", userID)
	return session, nil
}

// GetSession 获取会话
func (m *Manager) GetSession(sessionID string) (*models.Session, error) {
	if v, ok := m.sessions.Load(sessionID); ok {
		return v.(*models.Session), nil
	}
	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// GetUserSessions 获取用户的所有会话
func (m *Manager) GetUserSessions(userID string) []*models.Session {
	var sessions []*models.Session

	if v, ok := m.userSessions.Load(userID); ok {
		sessionIDs := v.([]string)
		for _, id := range sessionIDs {
			if s, ok := m.sessions.Load(id); ok {
				sessions = append(sessions, s.(*models.Session))
			}
		}
	}

	return sessions
}

// UpdateSession 更新会话
func (m *Manager) UpdateSession(ctx context.Context, session *models.Session) error {
	session.UpdatedAt = time.Now()
	m.sessions.Store(session.ID, session)

	// 发布会话更新事件
	event := &models.SessionEvent{
		BaseEvent: models.NewBaseEvent(models.EventSessionUpdated),
		UserID:    session.UserID,
		SessionID: session.ID,
		Session:   session,
	}

	if err := m.natsClient.Publish(nats.UserSessionsSubject(session.UserID), event); err != nil {
		m.logger.Error("Failed to publish session updated event", "error", err)
	}

	return nil
}

// DeleteSession 删除会话
func (m *Manager) DeleteSession(ctx context.Context, userID, sessionID string) error {
	m.sessions.Delete(sessionID)
	m.messages.Delete(sessionID)
	m.removeUserSession(userID, sessionID)

	// 发布会话删除事件
	event := &models.SessionEvent{
		BaseEvent: models.NewBaseEvent(models.EventSessionDeleted),
		UserID:    userID,
		SessionID: sessionID,
	}

	if err := m.natsClient.Publish(nats.UserSessionsSubject(userID), event); err != nil {
		m.logger.Error("Failed to publish session deleted event", "error", err)
	}

	m.logger.Info("Session deleted", "session_id", sessionID, "user_id", userID)
	return nil
}

// ArchiveSession 归档会话
func (m *Manager) ArchiveSession(ctx context.Context, userID, sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.IsArchived = true
	session.UpdatedAt = time.Now()
	m.sessions.Store(sessionID, session)

	// 发布会话归档事件
	event := &models.SessionEvent{
		BaseEvent: models.NewBaseEvent(models.EventSessionArchived),
		UserID:    userID,
		SessionID: sessionID,
		Session:   session,
	}

	if err := m.natsClient.Publish(nats.UserSessionsSubject(userID), event); err != nil {
		m.logger.Error("Failed to publish session archived event", "error", err)
	}

	return nil
}

// UpdateSessionStatus 更新会话状态
func (m *Manager) UpdateSessionStatus(ctx context.Context, userID, sessionID string, status models.SessionStatusType, model string) error {
	event := &models.SessionStatusEvent{
		SessionID: sessionID,
		UserID:    userID,
		Status:    status,
		Model:     model,
		Timestamp: time.Now(),
	}

	subject := nats.SessionSubject(userID, sessionID, "status")
	return m.natsClient.Publish(subject, event)
}

// SendTypingEvent 发送输入状态事件
func (m *Manager) SendTypingEvent(ctx context.Context, userID, sessionID, deviceID string, isTyping bool) error {
	event := &models.TypingEvent{
		SessionID: sessionID,
		UserID:    userID,
		DeviceID:  deviceID,
		IsTyping:  isTyping,
		Timestamp: time.Now(),
	}

	subject := nats.SessionSubject(userID, sessionID, "typing")
	return m.natsClient.Publish(subject, event)
}

// 辅助方法
func (m *Manager) addUserSession(userID, sessionID string) {
	var sessionIDs []string
	if v, ok := m.userSessions.Load(userID); ok {
		sessionIDs = v.([]string)
	}
	sessionIDs = append(sessionIDs, sessionID)
	m.userSessions.Store(userID, sessionIDs)
}

func (m *Manager) removeUserSession(userID, sessionID string) {
	if v, ok := m.userSessions.Load(userID); ok {
		sessionIDs := v.([]string)
		newIDs := make([]string, 0, len(sessionIDs))
		for _, id := range sessionIDs {
			if id != sessionID {
				newIDs = append(newIDs, id)
			}
		}
		m.userSessions.Store(userID, newIDs)
	}
}
