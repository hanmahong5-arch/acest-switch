package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings 同步设置
type Settings struct {
	Enabled       bool   `json:"enabled"`
	NATSURL       string `json:"nats_url"`
	SyncServerURL string `json:"sync_server_url"`
	UserID        string `json:"user_id"`
	SessionID     string `json:"session_id"`
	DeviceID      string `json:"device_id"`
	DeviceName    string `json:"device_name"`
	AccessToken   string `json:"access_token,omitempty"`
}

// DefaultSettings 默认设置
func DefaultSettings() *Settings {
	hostname, _ := os.Hostname()
	return &Settings{
		Enabled:       false,
		NATSURL:       "nats://localhost:4222",
		SyncServerURL: "http://localhost:8081",
		DeviceID:      generateDeviceID(),
		DeviceName:    hostname,
	}
}

// SettingsService 同步设置服务
type SettingsService struct {
	settings *Settings
	filePath string
}

// NewSettingsService 创建设置服务
func NewSettingsService() *SettingsService {
	home, _ := os.UserHomeDir()
	filePath := filepath.Join(home, ".code-switch", "sync.json")

	svc := &SettingsService{
		settings: DefaultSettings(),
		filePath: filePath,
	}

	// 尝试加载已有设置
	svc.Load()

	return svc
}

// Load 加载设置
func (s *SettingsService) Load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，使用默认值
		}
		return err
	}

	return json.Unmarshal(data, s.settings)
}

// Save 保存设置
func (s *SettingsService) Save() error {
	// 确保目录存在
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// Get 获取设置
func (s *SettingsService) Get() *Settings {
	return s.settings
}

// Update 更新设置
func (s *SettingsService) Update(settings *Settings) error {
	s.settings = settings
	return s.Save()
}

// IsEnabled 检查是否启用
func (s *SettingsService) IsEnabled() bool {
	return s.settings.Enabled
}

// GetNATSConfig 获取 NATS 配置
func (s *SettingsService) GetNATSConfig() *NATSConfig {
	return &NATSConfig{
		URL:                 s.settings.NATSURL,
		Enabled:             s.settings.Enabled,
		ReconnectWait:       2000000000, // 2s in nanoseconds
		MaxReconnects:       -1,
		ReconnectBufferSize: 8 * 1024 * 1024,
	}
}

// GetSyncConfig 获取同步配置
func (s *SettingsService) GetSyncConfig() *SyncConfig {
	return &SyncConfig{
		NATSConfig:    s.GetNATSConfig(),
		SyncServerURL: s.settings.SyncServerURL,
	}
}

// generateDeviceID 生成设备 ID
func generateDeviceID() string {
	// 简单实现，生产环境应使用 UUID
	return fmt.Sprintf("device-%d", os.Getpid())
}
