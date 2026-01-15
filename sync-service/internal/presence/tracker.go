package presence

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/internal/nats"
	"github.com/aspect-code/codeswitch/sync-service/pkg/models"
)

// Config 在线状态配置
type Config struct {
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	Timeout           time.Duration `yaml:"timeout"`
}

// Tracker 在线状态追踪器
type Tracker struct {
	natsClient *nats.Client
	config     *Config
	logger     *slog.Logger

	// 内存存储 (生产环境应使用 Redis)
	presence sync.Map // map[userID:deviceID]*models.Presence
	userDevices sync.Map // map[userID]map[deviceID]bool
}

// NewTracker 创建在线状态追踪器
func NewTracker(natsClient *nats.Client, cfg *Config, logger *slog.Logger) *Tracker {
	t := &Tracker{
		natsClient: natsClient,
		config:     cfg,
		logger:     logger,
	}

	// 启动过期检查
	go t.runExpirationChecker()

	return t
}

// Heartbeat 处理心跳
func (t *Tracker) Heartbeat(ctx context.Context, userID, deviceID, deviceType, clientVersion string) error {
	key := userID + ":" + deviceID
	now := time.Now()

	// 检查是否是新上线
	_, wasOnline := t.presence.Load(key)

	presence := &models.Presence{
		UserID:        userID,
		DeviceID:      deviceID,
		DeviceType:    deviceType,
		Status:        models.PresenceOnline,
		ClientVersion: clientVersion,
		LastSeenAt:    now,
	}

	t.presence.Store(key, presence)
	t.addUserDevice(userID, deviceID)

	// 如果是新上线，发布上线事件
	if !wasOnline {
		t.publishPresenceEvent(userID, deviceID, models.PresenceOnline)
		t.logger.Info("User came online",
			"user_id", userID,
			"device_id", deviceID,
			"device_type", deviceType,
		)
	}

	return nil
}

// SetOffline 设置离线
func (t *Tracker) SetOffline(userID, deviceID string) {
	key := userID + ":" + deviceID

	if _, ok := t.presence.Load(key); ok {
		t.presence.Delete(key)
		t.removeUserDevice(userID, deviceID)
		t.publishPresenceEvent(userID, deviceID, models.PresenceOffline)

		t.logger.Info("User went offline",
			"user_id", userID,
			"device_id", deviceID,
		)
	}
}

// SetAway 设置离开状态
func (t *Tracker) SetAway(userID, deviceID string) {
	key := userID + ":" + deviceID

	if v, ok := t.presence.Load(key); ok {
		presence := v.(*models.Presence)
		presence.Status = models.PresenceAway
		t.presence.Store(key, presence)
		t.publishPresenceEvent(userID, deviceID, models.PresenceAway)
	}
}

// GetUserPresence 获取用户在线状态
func (t *Tracker) GetUserPresence(userID string) []*models.Presence {
	var result []*models.Presence

	if v, ok := t.userDevices.Load(userID); ok {
		devices := v.(map[string]bool)
		for deviceID := range devices {
			key := userID + ":" + deviceID
			if p, ok := t.presence.Load(key); ok {
				result = append(result, p.(*models.Presence))
			}
		}
	}

	return result
}

// IsUserOnline 检查用户是否在线
func (t *Tracker) IsUserOnline(userID string) bool {
	if v, ok := t.userDevices.Load(userID); ok {
		devices := v.(map[string]bool)
		return len(devices) > 0
	}
	return false
}

// GetOnlineUsers 获取所有在线用户
func (t *Tracker) GetOnlineUsers() []string {
	var users []string
	t.userDevices.Range(func(key, value interface{}) bool {
		userID := key.(string)
		devices := value.(map[string]bool)
		if len(devices) > 0 {
			users = append(users, userID)
		}
		return true
	})
	return users
}

// GetOnlineDeviceCount 获取用户在线设备数
func (t *Tracker) GetOnlineDeviceCount(userID string) int {
	if v, ok := t.userDevices.Load(userID); ok {
		devices := v.(map[string]bool)
		return len(devices)
	}
	return 0
}

// 辅助方法
func (t *Tracker) addUserDevice(userID, deviceID string) {
	var devices map[string]bool
	if v, ok := t.userDevices.Load(userID); ok {
		devices = v.(map[string]bool)
	} else {
		devices = make(map[string]bool)
	}
	devices[deviceID] = true
	t.userDevices.Store(userID, devices)
}

func (t *Tracker) removeUserDevice(userID, deviceID string) {
	if v, ok := t.userDevices.Load(userID); ok {
		devices := v.(map[string]bool)
		delete(devices, deviceID)
		if len(devices) == 0 {
			t.userDevices.Delete(userID)
		} else {
			t.userDevices.Store(userID, devices)
		}
	}
}

func (t *Tracker) publishPresenceEvent(userID, deviceID string, status models.PresenceStatus) {
	event := &models.UserEvent{
		BaseEvent: models.NewBaseEvent(models.EventUserPresence),
		UserID:    userID,
		DeviceID:  deviceID,
		Data: map[string]interface{}{
			"status":    status,
			"timestamp": time.Now(),
		},
	}

	subject := nats.UserSubject(userID, "presence")
	if err := t.natsClient.Publish(subject, event); err != nil {
		t.logger.Error("Failed to publish presence event", "error", err)
	}
}

// runExpirationChecker 定期检查过期的在线状态
func (t *Tracker) runExpirationChecker() {
	ticker := time.NewTicker(t.config.HeartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		t.checkExpiredPresence()
	}
}

func (t *Tracker) checkExpiredPresence() {
	now := time.Now()
	threshold := now.Add(-t.config.Timeout)

	t.presence.Range(func(key, value interface{}) bool {
		presence := value.(*models.Presence)
		if presence.LastSeenAt.Before(threshold) {
			t.logger.Info("Presence expired",
				"user_id", presence.UserID,
				"device_id", presence.DeviceID,
				"last_seen", presence.LastSeenAt,
			)
			t.SetOffline(presence.UserID, presence.DeviceID)
		}
		return true
	})
}
