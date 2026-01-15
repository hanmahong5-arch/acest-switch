package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Client NATS 客户端
type Client struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	config *Config
	mu     sync.RWMutex
	subs   map[string]*nats.Subscription
	logger *slog.Logger
}

// Config NATS 配置
type Config struct {
	URL                 string        `yaml:"url"`
	ReconnectWait       time.Duration `yaml:"reconnect_wait"`
	MaxReconnects       int           `yaml:"max_reconnects"`
	ReconnectBufferSize int           `yaml:"reconnect_buffer_size"`
}

// NewClient 创建 NATS 客户端
func NewClient(cfg *Config, logger *slog.Logger) *Client {
	return &Client{
		config: cfg,
		subs:   make(map[string]*nats.Subscription),
		logger: logger,
	}
}

// Connect 连接 NATS 服务器
func (c *Client) Connect(ctx context.Context) error {
	opts := []nats.Option{
		nats.MaxReconnects(c.config.MaxReconnects),
		nats.ReconnectWait(c.config.ReconnectWait),
		nats.ReconnectBufSize(c.config.ReconnectBufferSize),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			c.logger.Warn("NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			c.logger.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			c.logger.Info("NATS connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			c.logger.Error("NATS error", "subject", sub.Subject, "error", err)
		}),
	}

	nc, err := nats.Connect(c.config.URL, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	c.nc = nc
	c.logger.Info("Connected to NATS", "url", nc.ConnectedUrl())

	// 初始化 JetStream (可选，用于持久化消息)
	js, err := nc.JetStream()
	if err != nil {
		c.logger.Warn("JetStream not available, using core NATS only", "error", err)
	} else {
		c.js = js
		c.logger.Info("JetStream enabled")
	}

	return nil
}

// Close 关闭连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 取消所有订阅
	for _, sub := range c.subs {
		sub.Unsubscribe()
	}
	c.subs = make(map[string]*nats.Subscription)

	if c.nc != nil {
		c.nc.Close()
	}
	return nil
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	return c.nc != nil && c.nc.IsConnected()
}

// Publish 发布消息
func (c *Client) Publish(subject string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := c.nc.Publish(subject, payload); err != nil {
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}

	c.logger.Debug("Published message", "subject", subject, "size", len(payload))
	return nil
}

// PublishWithAck 发布消息并等待确认 (JetStream)
func (c *Client) PublishWithAck(subject string, data interface{}) error {
	if c.js == nil {
		return c.Publish(subject, data)
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	ack, err := c.js.Publish(subject, payload)
	if err != nil {
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}

	c.logger.Debug("Published message with ack", "subject", subject, "seq", ack.Sequence)
	return nil
}

// Subscribe 订阅消息
func (c *Client) Subscribe(subject string, handler func(msg *nats.Msg)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Unsubscribe existing subscription to prevent leak
	if existingSub, exists := c.subs[subject]; exists {
		existingSub.Unsubscribe()
		delete(c.subs, subject)
		c.logger.Debug("Unsubscribed existing subscription", "subject", subject)
	}

	sub, err := c.nc.Subscribe(subject, handler)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	c.subs[subject] = sub
	c.logger.Info("Subscribed to subject", "subject", subject)
	return nil
}

// QueueSubscribe 队列订阅 (负载均衡)
func (c *Client) QueueSubscribe(subject, queue string, handler func(msg *nats.Msg)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s:%s", subject, queue)

	// Unsubscribe existing subscription to prevent leak
	if existingSub, exists := c.subs[key]; exists {
		existingSub.Unsubscribe()
		delete(c.subs, key)
		c.logger.Debug("Unsubscribed existing queue subscription", "subject", subject, "queue", queue)
	}

	sub, err := c.nc.QueueSubscribe(subject, queue, handler)
	if err != nil {
		return fmt.Errorf("failed to queue subscribe to %s: %w", subject, err)
	}

	c.subs[key] = sub
	c.logger.Info("Queue subscribed", "subject", subject, "queue", queue)
	return nil
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(subject string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if sub, ok := c.subs[subject]; ok {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
		delete(c.subs, subject)
	}
	return nil
}

// Request 请求-响应模式
func (c *Client) Request(subject string, data interface{}, timeout time.Duration) (*nats.Msg, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	return c.nc.Request(subject, payload, timeout)
}

// --- Subject Helpers ---

// UserSubject 用户主题
func UserSubject(userID, event string) string {
	return fmt.Sprintf("user.%s.%s", userID, event)
}

// SessionSubject 会话主题
func SessionSubject(userID, sessionID, event string) string {
	return fmt.Sprintf("chat.%s.%s.%s", userID, sessionID, event)
}

// UserSessionsSubject 用户会话列表主题
func UserSessionsSubject(userID string) string {
	return fmt.Sprintf("chat.%s.sessions", userID)
}

// AdminSubject 管理主题
func AdminSubject(event string) string {
	return fmt.Sprintf("admin.%s", event)
}
