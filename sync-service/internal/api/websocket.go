package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// WSClient represents a connected WebSocket client
type WSClient struct {
	conn      *websocket.Conn
	userID    string
	deviceID  string
	send      chan []byte
	hub       *WSHub
	mu        sync.Mutex
	closed    bool
	natsSubs  []*nats.Subscription
}

// WSHub manages all WebSocket connections
type WSHub struct {
	clients    map[string]map[string]*WSClient // userID -> deviceID -> client
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan *WSMessage
	natsConn   *nats.Conn
	logger     *zap.Logger
	mu         sync.RWMutex
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      string          `json:"type"`
	UserID    string          `json:"user_id,omitempty"`
	DeviceID  string          `json:"device_id,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// NewWSHub creates a new WebSocket hub
func NewWSHub(natsConn *nats.Conn, logger *zap.Logger) *WSHub {
	return &WSHub{
		clients:    make(map[string]map[string]*WSClient),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		broadcast:  make(chan *WSMessage, 256),
		natsConn:   natsConn,
		logger:     logger,
	}
}

// Run starts the WebSocket hub
func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

func (h *WSHub) registerClient(client *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.userID] == nil {
		h.clients[client.userID] = make(map[string]*WSClient)
	}
	h.clients[client.userID][client.deviceID] = client

	h.logger.Info("WebSocket client registered",
		zap.String("user_id", client.userID),
		zap.String("device_id", client.deviceID),
	)

	// Subscribe to user's NATS subjects
	h.subscribeClientToNATS(client)
}

func (h *WSHub) unregisterClient(client *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if devices, ok := h.clients[client.userID]; ok {
		if _, exists := devices[client.deviceID]; exists {
			delete(devices, client.deviceID)
			close(client.send)

			// Unsubscribe from NATS
			for _, sub := range client.natsSubs {
				sub.Unsubscribe()
			}

			h.logger.Info("WebSocket client unregistered",
				zap.String("user_id", client.userID),
				zap.String("device_id", client.deviceID),
			)

			if len(devices) == 0 {
				delete(h.clients, client.userID)
			}
		}
	}
}

func (h *WSHub) broadcastMessage(message *WSMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if message.UserID != "" {
		// Send to specific user's devices
		if devices, ok := h.clients[message.UserID]; ok {
			data, _ := json.Marshal(message)
			for _, client := range devices {
				select {
				case client.send <- data:
				default:
					// Client buffer full, skip
				}
			}
		}
	}
}

func (h *WSHub) subscribeClientToNATS(client *WSClient) {
	subjects := []string{
		fmt.Sprintf("billing.%s", client.userID),
		fmt.Sprintf("user.%s.*", client.userID),
		fmt.Sprintf("chat.%s.>", client.userID),
	}

	for _, subject := range subjects {
		sub, err := h.natsConn.Subscribe(subject, func(msg *nats.Msg) {
			wsMsg := &WSMessage{
				Type:      "nats",
				UserID:    client.userID,
				Timestamp: time.Now(),
				Data:      msg.Data,
			}
			data, _ := json.Marshal(wsMsg)

			client.mu.Lock()
			if !client.closed {
				select {
				case client.send <- data:
				default:
				}
			}
			client.mu.Unlock()
		})

		if err != nil {
			h.logger.Error("Failed to subscribe to NATS",
				zap.Error(err),
				zap.String("subject", subject),
			)
			continue
		}

		client.natsSubs = append(client.natsSubs, sub)
	}
}

// SendToUser sends a message to all devices of a user
func (h *WSHub) SendToUser(userID string, msgType string, data interface{}) {
	dataBytes, _ := json.Marshal(data)
	message := &WSMessage{
		Type:      msgType,
		UserID:    userID,
		Timestamp: time.Now(),
		Data:      dataBytes,
	}
	h.broadcast <- message
}

// GetOnlineDevices returns the number of online devices for a user
func (h *WSHub) GetOnlineDevices(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID])
}

// WSHandler handles WebSocket connections
type WSHandler struct {
	hub    *WSHub
	logger *zap.Logger
}

// NewWSHandler creates a new WebSocket handler
func NewWSHandler(hub *WSHub, logger *zap.Logger) *WSHandler {
	return &WSHandler{
		hub:    hub,
		logger: logger,
	}
}

// HandleWebSocket handles WebSocket upgrade and connection
func (h *WSHandler) HandleWebSocket(c *gin.Context) {
	// Get user info from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	deviceID, _ := c.Get("device_id")
	if deviceID == nil {
		deviceID = "unknown"
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket", zap.Error(err))
		return
	}

	client := &WSClient{
		conn:     conn,
		userID:   userID.(string),
		deviceID: deviceID.(string),
		send:     make(chan []byte, 256),
		hub:      h.hub,
	}

	h.hub.register <- client

	// Start read and write pumps
	go client.writePump()
	go client.readPump()
}

func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("WebSocket read error", zap.Error(err))
			}
			break
		}

		// Handle incoming message
		c.handleMessage(message)
	}
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *WSClient) handleMessage(message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		c.hub.logger.Error("Failed to unmarshal WebSocket message", zap.Error(err))
		return
	}

	switch msg.Type {
	case "ping":
		// Respond with pong
		response := WSMessage{
			Type:      "pong",
			Timestamp: time.Now(),
		}
		data, _ := json.Marshal(response)
		c.send <- data

	case "subscribe":
		// Handle additional subscription requests
		c.hub.logger.Debug("Subscribe request",
			zap.String("user_id", c.userID),
			zap.ByteString("data", msg.Data),
		)

	case "sync_request":
		// Handle sync request - client wants latest state
		c.hub.logger.Debug("Sync request",
			zap.String("user_id", c.userID),
		)
		// TODO: Fetch and send latest state

	default:
		c.hub.logger.Debug("Unknown message type",
			zap.String("type", msg.Type),
			zap.String("user_id", c.userID),
		)
	}
}

// Close marks the client as closed
func (c *WSClient) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}
