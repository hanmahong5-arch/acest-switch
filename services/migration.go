package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daodao97/xgo/xdb"
	"github.com/daodao97/xgo/xlog"
)

// MigrateGeminiProvider 将 Google Gemini 从 codex 迁移到 gemini-cli
// 此函数应在应用启动时调用，自动检测并执行迁移
func MigrateGeminiProvider() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	codexPath := filepath.Join(home, ".code-switch", "codex.json")
	geminiPath := filepath.Join(home, ".code-switch", "gemini-cli.json")

	// 如果 gemini-cli.json 已存在，跳过迁移
	if _, err := os.Stat(geminiPath); err == nil {
		xlog.Info("[Migration] gemini-cli.json 已存在，跳过迁移")
		return nil
	}

	// 如果 codex.json 不存在，跳过迁移
	if _, err := os.Stat(codexPath); os.IsNotExist(err) {
		xlog.Info("[Migration] codex.json 不存在，跳过迁移")
		return nil
	}

	// 读取 codex.json
	codexData, err := os.ReadFile(codexPath)
	if err != nil {
		return fmt.Errorf("读取 codex.json 失败: %w", err)
	}

	var codexEnvelope providerEnvelope
	if err := json.Unmarshal(codexData, &codexEnvelope); err != nil {
		return fmt.Errorf("解析 codex.json 失败: %w", err)
	}

	// 查找并移除 Google Gemini
	var geminiProvider *Provider
	remainingProviders := make([]Provider, 0)
	for i, p := range codexEnvelope.Providers {
		if p.Name == "Google Gemini" {
			geminiProvider = &codexEnvelope.Providers[i]
			xlog.Info("[Migration] 找到 Google Gemini 供应商，准备迁移")
		} else {
			remainingProviders = append(remainingProviders, p)
		}
	}

	// 如果未找到 Google Gemini，跳过迁移
	if geminiProvider == nil {
		xlog.Info("[Migration] codex.json 中未找到 Google Gemini 供应商，跳过迁移")
		return nil
	}

	// 更新 codex.json（移除 Google Gemini）
	codexEnvelope.Providers = remainingProviders
	codexBytes, err := json.MarshalIndent(codexEnvelope, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 codex.json 失败: %w", err)
	}

	// 备份原 codex.json
	backupPath := codexPath + ".backup-before-gemini-migration"
	if err := os.WriteFile(backupPath, codexData, 0644); err != nil {
		xlog.Warn("[Migration] 创建备份失败: %v", err)
	} else {
		xlog.Info("[Migration] 已创建备份: %s", backupPath)
	}

	// 写入更新后的 codex.json
	if err := os.WriteFile(codexPath, codexBytes, 0644); err != nil {
		return fmt.Errorf("写入 codex.json 失败: %w", err)
	}

	// 创建 gemini-cli.json
	geminiEnvelope := providerEnvelope{
		Providers: []Provider{*geminiProvider},
	}
	geminiBytes, err := json.MarshalIndent(geminiEnvelope, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 gemini-cli.json 失败: %w", err)
	}
	if err := os.WriteFile(geminiPath, geminiBytes, 0644); err != nil {
		return fmt.Errorf("写入 gemini-cli.json 失败: %w", err)
	}

	xlog.Info("✓ [Migration] Google Gemini 已成功从 Codex 迁移到 Gemini-CLI 页签")
	return nil
}

// MigratePicoClawProxyControl seeds picoclaw into proxy_control for existing installations
func MigratePicoClawProxyControl() {
	db, err := xdb.DB("default")
	if err != nil {
		return
	}
	db.Exec(`INSERT OR IGNORE INTO proxy_control (app_name, proxy_enabled) VALUES ('picoclaw', 1)`)
}

// 在 ProviderRelayService 启动时调用此函数
func (prs *ProviderRelayService) RunMigrations() {
	if err := MigrateGeminiProvider(); err != nil {
		xlog.Error("[Migration] 迁移失败: %v", err)
	}
	MigratePicoClawProxyControl()
}
