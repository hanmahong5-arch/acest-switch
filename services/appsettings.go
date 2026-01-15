package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	appSettingsDir  = ".codex-switch"
	appSettingsFile = "app.json"
)

type AppSettings struct {
	ShowHeatmap   bool `json:"show_heatmap"`
	ShowHomeTitle bool `json:"show_home_title"`
	AutoStart     bool `json:"auto_start"`
	EnableBodyLog bool `json:"enable_body_log"` // 上下行日志开关

	// NEW-API 统一网关配置
	NewAPIEnabled bool   `json:"new_api_enabled"` // 是否启用 new-api 统一网关模式
	NewAPIURL     string `json:"new_api_url"`     // new-api 服务地址，默认 http://localhost:3000
	NewAPIToken   string `json:"new_api_token"`   // new-api API Token (sk-xxx)
}

type AppSettingsService struct {
	path             string
	mu               sync.Mutex
	autoStartService *AutoStartService
}

func NewAppSettingsService(autoStartService *AutoStartService) *AppSettingsService {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	path := filepath.Join(home, appSettingsDir, appSettingsFile)
	return &AppSettingsService{
		path:             path,
		autoStartService: autoStartService,
	}
}

func (as *AppSettingsService) defaultSettings() AppSettings {
	// 检查当前开机自启动状态
	autoStartEnabled := false
	if as.autoStartService != nil {
		if enabled, err := as.autoStartService.IsEnabled(); err == nil {
			autoStartEnabled = enabled
		}
	}

	return AppSettings{
		ShowHeatmap:   true,
		ShowHomeTitle: true,
		AutoStart:     autoStartEnabled,
		// NEW-API 默认配置
		NewAPIEnabled: false,                      // 默认禁用，需要用户手动开启
		NewAPIURL:     "http://api.lurus.cn",      // 生产环境地址
		NewAPIToken:   "",                         // 需要用户配置
	}
}

// GetAppSettings returns the persisted app settings or defaults if the file does not exist.
func (as *AppSettingsService) GetAppSettings() (AppSettings, error) {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.loadLocked()
}

// SaveAppSettings persists the provided settings to disk.
func (as *AppSettingsService) SaveAppSettings(settings AppSettings) (AppSettings, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	// 同步开机自启动状态
	if as.autoStartService != nil {
		if settings.AutoStart {
			if err := as.autoStartService.Enable(); err != nil {
				return settings, err
			}
		} else {
			if err := as.autoStartService.Disable(); err != nil {
				return settings, err
			}
		}
	}

	if err := as.saveLocked(settings); err != nil {
		return settings, err
	}
	return settings, nil
}

func (as *AppSettingsService) loadLocked() (AppSettings, error) {
	settings := as.defaultSettings()
	data, err := os.ReadFile(as.path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return settings, err
	}
	if len(data) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}
	return settings, nil
}

func (as *AppSettingsService) saveLocked(settings AppSettings) error {
	dir := filepath.Dir(as.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(as.path, data, 0o644)
}

// ConnectionStatus represents the result of a NEW-API connection test.
type ConnectionStatus struct {
	Success bool       `json:"success"`
	User    *UserInfo  `json:"user,omitempty"`
	Quota   *QuotaInfo `json:"quota,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// UserInfo represents user information from NEW-API.
type UserInfo struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"`
}

// QuotaInfo represents quota information from NEW-API.
type QuotaInfo struct {
	QuotaTotal   int `json:"quotaTotal"`
	QuotaUsed    int `json:"quotaUsed"`
	QuotaRemain  int `json:"quotaRemain"`
	RequestCount int `json:"requestCount,omitempty"`
}

// TestNewAPIConnection tests connection to a NEW-API server.
func (as *AppSettingsService) TestNewAPIConnection(url, token string) ConnectionStatus {
	if url == "" || token == "" {
		return ConnectionStatus{
			Success: false,
			Error:   "URL and token are required",
		}
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/user/self", url), nil)
	if err != nil {
		return ConnectionStatus{
			Success: false,
			Error:   fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return ConnectionStatus{
			Success: false,
			Error:   fmt.Sprintf("Connection failed: %v", err),
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ConnectionStatus{
			Success: false,
			Error:   fmt.Sprintf("Failed to read response: %v", err),
		}
	}

	if resp.StatusCode != http.StatusOK {
		return ConnectionStatus{
			Success: false,
			Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}
	}

	// Parse NEW-API response format
	var apiResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			ID           int    `json:"id"`
			Username     string `json:"username"`
			Email        string `json:"email"`
			DisplayName  string `json:"display_name"`
			Quota        int    `json:"quota"`
			UsedQuota    int    `json:"used_quota"`
			RequestCount int    `json:"request_count"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return ConnectionStatus{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse response: %v", err),
		}
	}

	if !apiResp.Success {
		return ConnectionStatus{
			Success: false,
			Error:   apiResp.Message,
		}
	}

	return ConnectionStatus{
		Success: true,
		User: &UserInfo{
			ID:          apiResp.Data.ID,
			Username:    apiResp.Data.Username,
			Email:       apiResp.Data.Email,
			DisplayName: apiResp.Data.DisplayName,
		},
		Quota: &QuotaInfo{
			QuotaTotal:   apiResp.Data.Quota,
			QuotaUsed:    apiResp.Data.UsedQuota,
			QuotaRemain:  apiResp.Data.Quota - apiResp.Data.UsedQuota,
			RequestCount: apiResp.Data.RequestCount,
		},
	}
}
