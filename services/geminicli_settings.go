package services

import (
	"fmt"
	"time"

	"github.com/daodao97/xgo/xdb"
)

type GeminiCLISettingsService struct{}

func NewGeminiCLISettingsService() *GeminiCLISettingsService {
	return &GeminiCLISettingsService{}
}

func (s *GeminiCLISettingsService) Start() error { return nil }
func (s *GeminiCLISettingsService) Stop() error  { return nil }

// ProxyStatus 检查是否有来自 gemini-cli 的请求（判断代理是否在使用中）
// 查询最近5分钟是否有 platform='gemini-cli' 的请求日志
func (s *GeminiCLISettingsService) ProxyStatus() (ClaudeProxyStatus, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return ClaudeProxyStatus{Enabled: false, BaseURL: "http://127.0.0.1:18100"},
			fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 查询最近5分钟的请求
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	query := `
		SELECT COUNT(*)
		FROM request_log
		WHERE platform = 'gemini-cli'
		AND created_at > ?
	`

	var count int
	err = db.QueryRow(query, fiveMinutesAgo.Format("2006-01-02 15:04:05")).Scan(&count)
	if err != nil {
		// 如果查询失败，默认返回 false（代理未启用）
		return ClaudeProxyStatus{Enabled: false, BaseURL: "http://127.0.0.1:18100"}, nil
	}

	return ClaudeProxyStatus{
		Enabled: count > 0,
		BaseURL: "http://127.0.0.1:18100",
	}, nil
}

// EnableProxy Gemini-CLI 代理通过启动脚本控制，返回友好提示
func (s *GeminiCLISettingsService) EnableProxy() error {
	return fmt.Errorf("Gemini-CLI 代理通过启动脚本控制。\n请使用: ./gemini-codeswitch 或 ./chat.sh 启动")
}

// DisableProxy Gemini-CLI 代理通过启动脚本控制，返回友好提示
func (s *GeminiCLISettingsService) DisableProxy() error {
	return fmt.Errorf("Gemini-CLI 代理通过启动脚本控制。\n使用普通 gemini 命令即可禁用代理")
}
