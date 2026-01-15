package services

import (
	"codeswitch/services/sync"
	"context"
	"fmt"
)

// SyncSettingsService 同步设置服务 (Wails 绑定)
type SyncSettingsService struct {
	settings    *sync.SettingsService
	syncService *sync.SyncService
}

// NewSyncSettingsService 创建同步设置服务
func NewSyncSettingsService() *SyncSettingsService {
	settings := sync.NewSettingsService()

	svc := &SyncSettingsService{
		settings: settings,
	}

	// 如果启用，初始化同步服务
	if settings.IsEnabled() {
		svc.initSyncService()
	}

	return svc
}

func (s *SyncSettingsService) initSyncService() {
	cfg := s.settings.GetSyncConfig()
	s.syncService = sync.NewSyncService(cfg)
	if err := s.syncService.Start(); err != nil {
		fmt.Printf("[SyncSettings] Failed to start sync service: %v\n", err)
	}
}

// ServiceName Wails 服务名
func (s *SyncSettingsService) ServiceName() string {
	return "SyncSettingsService"
}

// ServiceStartup Wails 启动回调
func (s *SyncSettingsService) ServiceStartup(ctx context.Context) error {
	return nil
}

// ServiceShutdown Wails 关闭回调
func (s *SyncSettingsService) ServiceShutdown() error {
	if s.syncService != nil {
		return s.syncService.Stop()
	}
	return nil
}

// --- 前端 API ---

// GetSettings 获取同步设置
func (s *SyncSettingsService) GetSettings() *sync.Settings {
	return s.settings.Get()
}

// UpdateSettings 更新同步设置
func (s *SyncSettingsService) UpdateSettings(settings *sync.Settings) error {
	// 更新设置
	if err := s.settings.Update(settings); err != nil {
		return err
	}

	// 重新初始化同步服务
	if s.syncService != nil {
		s.syncService.Stop()
		s.syncService = nil
	}

	if settings.Enabled {
		s.initSyncService()
	}

	return nil
}

// GetStatus 获取同步状态
func (s *SyncSettingsService) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled":   s.settings.IsEnabled(),
		"connected": false,
	}

	if s.syncService != nil {
		status["connected"] = s.syncService.IsEnabled()
	}

	return status
}

// TestConnection 测试连接
func (s *SyncSettingsService) TestConnection(natsURL string) (bool, string) {
	cfg := &sync.NATSConfig{
		URL:     natsURL,
		Enabled: true,
	}

	client := sync.NewNATSClient(cfg)
	if err := client.Connect(); err != nil {
		return false, err.Error()
	}

	defer client.Close()
	return true, "Connected successfully"
}

// GetSyncService 获取同步服务实例 (供其他服务使用)
func (s *SyncSettingsService) GetSyncService() *sync.SyncService {
	return s.syncService
}
