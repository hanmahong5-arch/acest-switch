package message

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

// Handler 消息处理器
type Handler struct {
	natsClient *nats.Client
	logger     *slog.Logger

	// 内存存储 (生产环境应使用 PostgreSQL + Redis)
	messages     sync.Map // map[sessionID][]*models.Message
	messageIndex sync.Map // map[messageID]*models.Message
}

// NewHandler 创建消息处理器
func NewHandler(natsClient *nats.Client, logger *slog.Logger) *Handler {
	return &Handler{
		natsClient: natsClient,
		logger:     logger,
	}
}

// CreateMessage 创建消息 (idempotent - skips if message ID already exists)
func (h *Handler) CreateMessage(ctx context.Context, msg *models.Message) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.ContentType == "" {
		msg.ContentType = "text"
	}

	// Deduplication: check if message already exists
	if _, exists := h.messageIndex.Load(msg.ID); exists {
		h.logger.Debug("Message already exists, skipping",
			"message_id", msg.ID,
			"session_id", msg.SessionID,
		)
		return nil
	}

	// 存储消息
	h.storeMessage(msg)

	// 发布消息事件到 NATS
	event := &models.MessageEvent{
		BaseEvent: models.NewBaseEvent(models.EventMessageCreated),
		UserID:    msg.UserID,
		SessionID: msg.SessionID,
		MessageID: msg.ID,
		Message:   msg,
	}

	subject := nats.SessionSubject(msg.UserID, msg.SessionID, "msg")
	if err := h.natsClient.PublishWithAck(subject, event); err != nil {
		h.logger.Error("Failed to publish message event", "error", err)
		return err
	}

	h.logger.Debug("Message created and published",
		"message_id", msg.ID,
		"session_id", msg.SessionID,
		"role", msg.Role,
	)

	return nil
}

// GetMessages 获取会话消息
func (h *Handler) GetMessages(sessionID string, limit, offset int) ([]*models.Message, error) {
	if v, ok := h.messages.Load(sessionID); ok {
		messages := v.([]*models.Message)

		// 分页
		start := offset
		end := offset + limit
		if start >= len(messages) {
			return nil, nil
		}
		if end > len(messages) {
			end = len(messages)
		}

		return messages[start:end], nil
	}
	return nil, nil
}

// GetMessage 获取单条消息
func (h *Handler) GetMessage(messageID string) (*models.Message, error) {
	if v, ok := h.messageIndex.Load(messageID); ok {
		return v.(*models.Message), nil
	}
	return nil, fmt.Errorf("message not found: %s", messageID)
}

// GetMessagesSince 获取指定时间之后的消息
func (h *Handler) GetMessagesSince(sessionID string, since time.Time, limit int) ([]*models.Message, error) {
	if v, ok := h.messages.Load(sessionID); ok {
		messages := v.([]*models.Message)
		var result []*models.Message

		for _, msg := range messages {
			if msg.CreatedAt.After(since) {
				result = append(result, msg)
				if len(result) >= limit {
					break
				}
			}
		}

		return result, nil
	}
	return nil, nil
}

// GetMessagesAfterID 获取指定消息之后的消息
func (h *Handler) GetMessagesAfterID(sessionID, lastMsgID string, limit int) ([]*models.Message, error) {
	if v, ok := h.messages.Load(sessionID); ok {
		messages := v.([]*models.Message)
		var result []*models.Message
		found := false

		for _, msg := range messages {
			if found {
				result = append(result, msg)
				if len(result) >= limit {
					break
				}
			}
			if msg.ID == lastMsgID {
				found = true
			}
		}

		// 如果没找到 lastMsgID，返回所有消息
		if !found && lastMsgID == "" {
			end := limit
			if end > len(messages) {
				end = len(messages)
			}
			return messages[:end], nil
		}

		return result, nil
	}
	return nil, nil
}

// DeleteMessage 删除消息
func (h *Handler) DeleteMessage(ctx context.Context, userID, sessionID, messageID string) error {
	h.removeMessage(sessionID, messageID)

	// 发布删除事件
	event := &models.MessageEvent{
		BaseEvent: models.NewBaseEvent(models.EventMessageDeleted),
		UserID:    userID,
		SessionID: sessionID,
		MessageID: messageID,
	}

	subject := nats.SessionSubject(userID, sessionID, "msg")
	return h.natsClient.Publish(subject, event)
}

// 辅助方法
func (h *Handler) storeMessage(msg *models.Message) {
	// 存储到索引
	h.messageIndex.Store(msg.ID, msg)

	// 存储到会话消息列表
	var messages []*models.Message
	if v, ok := h.messages.Load(msg.SessionID); ok {
		messages = v.([]*models.Message)
	}
	messages = append(messages, msg)
	h.messages.Store(msg.SessionID, messages)
}

func (h *Handler) removeMessage(sessionID, messageID string) {
	h.messageIndex.Delete(messageID)

	if v, ok := h.messages.Load(sessionID); ok {
		messages := v.([]*models.Message)
		newMessages := make([]*models.Message, 0, len(messages))
		for _, msg := range messages {
			if msg.ID != messageID {
				newMessages = append(newMessages, msg)
			}
		}
		h.messages.Store(sessionID, newMessages)
	}
}

// GetSessionStats 获取会话统计
func (h *Handler) GetSessionStats(sessionID string) (messageCount, tokenCount int, cost float64) {
	if v, ok := h.messages.Load(sessionID); ok {
		messages := v.([]*models.Message)
		for _, msg := range messages {
			messageCount++
			tokenCount += msg.TokensInput + msg.TokensOutput
			cost += msg.Cost
		}
	}
	return
}
