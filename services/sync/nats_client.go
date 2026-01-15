package sync

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSClient NATS 客户端
type NATSClient struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	config *NATSConfig
	mu     sync.RWMutex
	subs   map[string]*nats.Subscription
}

// NATSConfig NATS 配置
type NATSConfig struct {
	URL                 string
	ReconnectWait       time.Duration
	MaxReconnects       int
	ReconnectBufferSize int
	Enabled             bool
}

// DefaultNATSConfig 默认配置
func DefaultNATSConfig() *NATSConfig {
	return &NATSConfig{
		URL:                 "nats://localhost:4222",
		ReconnectWait:       2 * time.Second,
		MaxReconnects:       -1, // 无限重连
		ReconnectBufferSize: 8 * 1024 * 1024,
		Enabled:             false, // 默认禁用
	}
}

// NewNATSClient 创建 NATS 客户端
func NewNATSClient(cfg *NATSConfig) *NATSClient {
	if cfg == nil {
		cfg = DefaultNATSConfig()
	}
	return &NATSClient{
		config: cfg,
		subs:   make(map[string]*nats.Subscription),
	}
}

// Connect 连接 NATS 服务器
func (c *NATSClient) Connect() error {
	if !c.config.Enabled {
		fmt.Println("[Sync] NATS sync is disabled")
		return nil
	}

	opts := []nats.Option{
		nats.MaxReconnects(c.config.MaxReconnects),
		nats.ReconnectWait(c.config.ReconnectWait),
		nats.ReconnectBufSize(c.config.ReconnectBufferSize),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			fmt.Printf("[Sync] NATS disconnected: %v\n", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			fmt.Printf("[Sync] NATS reconnected to %s\n", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			fmt.Println("[Sync] NATS connection closed")
		}),
	}

	nc, err := nats.Connect(c.config.URL, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	c.nc = nc
	fmt.Printf("[Sync] Connected to NATS at %s\n", nc.ConnectedUrl())

	// 初始化 JetStream
	js, err := nc.JetStream()
	if err != nil {
		fmt.Printf("[Sync] JetStream not available: %v\n", err)
	} else {
		c.js = js
		fmt.Println("[Sync] JetStream enabled")
	}

	return nil
}

// Close 关闭连接
func (c *NATSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

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
func (c *NATSClient) IsConnected() bool {
	return c.nc != nil && c.nc.IsConnected()
}

// IsEnabled 检查是否启用
func (c *NATSClient) IsEnabled() bool {
	return c.config.Enabled
}

// Publish 发布消息
func (c *NATSClient) Publish(subject string, data interface{}) error {
	if !c.IsConnected() {
		return nil // 静默失败
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return c.nc.Publish(subject, payload)
}

// PublishWithAck 发布消息并等待确认 (JetStream)
func (c *NATSClient) PublishWithAck(subject string, data interface{}) error {
	if !c.IsConnected() {
		return nil
	}

	if c.js == nil {
		return c.Publish(subject, data)
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	_, err = c.js.Publish(subject, payload)
	return err
}

// Subscribe 订阅消息
func (c *NATSClient) Subscribe(subject string, handler func(msg *nats.Msg)) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Unsubscribe existing subscription to prevent leak
	if existingSub, exists := c.subs[subject]; exists {
		existingSub.Unsubscribe()
		delete(c.subs, subject)
	}

	sub, err := c.nc.Subscribe(subject, handler)
	if err != nil {
		return err
	}

	c.subs[subject] = sub
	return nil
}

// SubscribeWithHandler 订阅消息并使用简化处理器
// handler 接收原始数据，返回响应数据（用于 request/reply 模式）
func (c *NATSClient) SubscribeWithHandler(subject string, handler func(data []byte) []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Unsubscribe existing subscription to prevent leak
	if existingSub, exists := c.subs[subject]; exists {
		existingSub.Unsubscribe()
		delete(c.subs, subject)
	}

	sub, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		resp := handler(msg.Data)
		if msg.Reply != "" && resp != nil {
			msg.Respond(resp)
		}
	})
	if err != nil {
		return err
	}

	c.subs[subject] = sub
	return nil
}

// QueueSubscribe 使用队列组订阅（负载均衡，同一消息只被组内一个消费者处理）
func (c *NATSClient) QueueSubscribe(subject, queue string, handler func(msg *nats.Msg)) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Use subject+queue as key for queue subscriptions
	subKey := subject + ":" + queue
	if existingSub, exists := c.subs[subKey]; exists {
		existingSub.Unsubscribe()
		delete(c.subs, subKey)
	}

	sub, err := c.nc.QueueSubscribe(subject, queue, handler)
	if err != nil {
		return err
	}

	c.subs[subKey] = sub
	return nil
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

// LLMRequestSubject LLM 请求主题
func LLMRequestSubject(provider string) string {
	return fmt.Sprintf("llm.request.%s", provider)
}

// LLMResponseSubject LLM 响应主题
func LLMResponseSubject(traceID string) string {
	return fmt.Sprintf("llm.response.%s", traceID)
}

// QuotaSubject 配额主题
func QuotaSubject(userID string) string {
	return fmt.Sprintf("user.%s.quota", userID)
}
