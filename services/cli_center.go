package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CLIStatus represents the status of a single CLI tool
type CLIStatus struct {
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
	BaseURL    string `json:"base_url"`
	ConfigPath string `json:"config_path"`
	LastError  string `json:"last_error,omitempty"`
}

// AllCLIStatus represents the status of all CLI tools
type AllCLIStatus struct {
	Claude   CLIStatus `json:"claude"`
	Codex    CLIStatus `json:"codex"`
	Gemini   CLIStatus `json:"gemini"`
	PicoClaw CLIStatus `json:"picoclaw"`
}

// HealthStatus represents the health of a single CLI tool
type HealthStatus struct {
	Status       string `json:"status"` // healthy, warning, error
	Message      string `json:"message"`
	ConfigExists bool   `json:"config_exists"`
	ConfigValid  bool   `json:"config_valid"`
}

// HealthCheckResult represents the health check results for all CLIs
type HealthCheckResult struct {
	Claude             HealthStatus `json:"claude"`
	Codex              HealthStatus `json:"codex"`
	Gemini             HealthStatus `json:"gemini"`
	PicoClaw           HealthStatus `json:"picoclaw"`
	ProxyServerRunning bool         `json:"proxy_server_running"`
}

// ProxyConfig represents the proxy server configuration
type ProxyConfig struct {
	Port     int    `json:"port"`
	Host     string `json:"host"`
	Protocol string `json:"protocol"`
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Claude   string `json:"claude"`
	Codex    string `json:"codex"`
	Gemini   string `json:"gemini"`
	PicoClaw string `json:"picoclaw"`
}

// CLICenterService provides unified management for all CLI tools
type CLICenterService struct {
	claudeService   *ClaudeSettingsService
	codexService    *CodexSettingsService
	geminiService   *GeminiCLISettingsService
	picoClawService *PicoClawSettingsService
	relayAddr       string
	configDir       string
}

// NewCLICenterService creates a new CLI Center service
func NewCLICenterService(
	claudeService *ClaudeSettingsService,
	codexService *CodexSettingsService,
	geminiService *GeminiCLISettingsService,
	picoClawService *PicoClawSettingsService,
	relayAddr string,
) *CLICenterService {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".code-switch")

	return &CLICenterService{
		claudeService:   claudeService,
		codexService:    codexService,
		geminiService:   geminiService,
		picoClawService: picoClawService,
		relayAddr:       relayAddr,
		configDir:       configDir,
	}
}

// GetAllStatus returns the status of all CLI tools
func (s *CLICenterService) GetAllStatus() (*AllCLIStatus, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	result := &AllCLIStatus{}

	// Claude status
	claudeStatus, _ := s.claudeService.ProxyStatus()
	claudeConfigPath := filepath.Join(home, ".claude", "settings.json")
	result.Claude = CLIStatus{
		Enabled:    claudeStatus.Enabled,
		Configured: fileExists(claudeConfigPath),
		BaseURL:    claudeStatus.BaseURL,
		ConfigPath: claudeConfigPath,
	}

	// Codex status
	codexStatus, _ := s.codexService.ProxyStatus()
	codexConfigPath := filepath.Join(home, ".codex", "config.toml")
	result.Codex = CLIStatus{
		Enabled:    codexStatus.Enabled,
		Configured: fileExists(codexConfigPath),
		BaseURL:    codexStatus.BaseURL,
		ConfigPath: codexConfigPath,
	}

	// Gemini status
	geminiStatus, _ := s.geminiService.ProxyStatus()
	geminiScriptPath := s.getGeminiScriptPath()
	result.Gemini = CLIStatus{
		Enabled:    geminiStatus.Enabled,
		Configured: fileExists(geminiScriptPath),
		BaseURL:    geminiStatus.BaseURL,
		ConfigPath: geminiScriptPath,
	}

	// PicoClaw status
	picoClawStatus, _ := s.picoClawService.ProxyStatus()
	picoClawConfigPath := filepath.Join(home, ".picoclaw", "config.json")
	result.PicoClaw = CLIStatus{
		Enabled:    picoClawStatus.Enabled,
		Configured: fileExists(picoClawConfigPath),
		BaseURL:    picoClawStatus.BaseURL,
		ConfigPath: picoClawConfigPath,
	}

	return result, nil
}

// EnableAll enables proxy for all CLI tools
func (s *CLICenterService) EnableAll() (*BatchResult, error) {
	result := &BatchResult{}

	// Enable Claude
	if err := s.claudeService.EnableProxy(); err != nil {
		result.Claude = err.Error()
	} else {
		result.Claude = "success"
	}

	// Enable Codex
	if err := s.codexService.EnableProxy(); err != nil {
		result.Codex = err.Error()
	} else {
		result.Codex = "success"
	}

	// Enable Gemini (create script)
	if err := s.createGeminiScript(); err != nil {
		result.Gemini = err.Error()
	} else {
		result.Gemini = "success"
	}

	// Enable PicoClaw
	if err := s.picoClawService.EnableProxy(); err != nil {
		result.PicoClaw = err.Error()
	} else {
		result.PicoClaw = "success"
	}

	return result, nil
}

// DisableAll disables proxy for all CLI tools
func (s *CLICenterService) DisableAll() (*BatchResult, error) {
	result := &BatchResult{}

	// Disable Claude
	if err := s.claudeService.DisableProxy(); err != nil {
		result.Claude = err.Error()
	} else {
		result.Claude = "success"
	}

	// Disable Codex
	if err := s.codexService.DisableProxy(); err != nil {
		result.Codex = err.Error()
	} else {
		result.Codex = "success"
	}

	// Disable Gemini (remove script)
	if err := s.removeGeminiScript(); err != nil {
		result.Gemini = err.Error()
	} else {
		result.Gemini = "success"
	}

	// Disable PicoClaw
	if err := s.picoClawService.DisableProxy(); err != nil {
		result.PicoClaw = err.Error()
	} else {
		result.PicoClaw = "success"
	}

	return result, nil
}

// HealthCheck performs a health check on all CLI tools and proxy server
func (s *CLICenterService) HealthCheck() (*HealthCheckResult, error) {
	home, _ := os.UserHomeDir()
	result := &HealthCheckResult{}

	// Check proxy server
	result.ProxyServerRunning = s.isProxyServerRunning()

	// Check Claude
	claudeConfigPath := filepath.Join(home, ".claude", "settings.json")
	result.Claude = s.checkCLIHealth("Claude Code", claudeConfigPath, func() error {
		var settings struct {
			Env map[string]string `json:"env"`
		}
		data, err := os.ReadFile(claudeConfigPath)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, &settings)
	})

	// Check Codex
	codexConfigPath := filepath.Join(home, ".codex", "config.toml")
	result.Codex = s.checkCLIHealth("Codex", codexConfigPath, func() error {
		_, err := os.ReadFile(codexConfigPath)
		return err
	})

	// Check Gemini
	geminiScriptPath := s.getGeminiScriptPath()
	result.Gemini = s.checkCLIHealth("Gemini CLI", geminiScriptPath, func() error {
		_, err := os.ReadFile(geminiScriptPath)
		return err
	})

	// Check PicoClaw
	picoClawConfigPath := filepath.Join(home, ".picoclaw", "config.json")
	result.PicoClaw = s.checkCLIHealth("PicoClaw", picoClawConfigPath, func() error {
		data, err := os.ReadFile(picoClawConfigPath)
		if err != nil {
			return err
		}
		// Validate JSON syntax
		var js json.RawMessage
		return json.Unmarshal(data, &js)
	})

	return result, nil
}

// GetConfigPaths returns the configuration paths for all CLI tools
func (s *CLICenterService) GetConfigPaths() (map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"claude":   filepath.Join(home, ".claude"),
		"codex":    filepath.Join(home, ".codex"),
		"gemini":   s.getGeminiScriptDir(),
		"picoclaw": filepath.Join(home, ".picoclaw"),
	}, nil
}

// OpenConfigDir opens the configuration directory for a specific CLI tool
func (s *CLICenterService) OpenConfigDir(cli string) error {
	paths, err := s.GetConfigPaths()
	if err != nil {
		return err
	}

	path, ok := paths[strings.ToLower(cli)]
	if !ok {
		return errors.New("unknown CLI: " + cli)
	}

	// Ensure directory exists
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}

	// Open directory based on OS
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Start()
}

// GetProxyConfig returns the current proxy configuration
func (s *CLICenterService) GetProxyConfig() (*ProxyConfig, error) {
	config := &ProxyConfig{
		Port:     18100,
		Host:     "127.0.0.1",
		Protocol: "http",
	}

	// Load from config file if exists
	configPath := filepath.Join(s.configDir, "proxy_config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, config)
	}

	return config, nil
}

// SetProxyConfig sets the proxy configuration
func (s *CLICenterService) SetProxyConfig(config *ProxyConfig) error {
	if config.Port < 1 || config.Port > 65535 {
		return errors.New("invalid port number")
	}

	if err := os.MkdirAll(s.configDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(s.configDir, "proxy_config.json")
	return os.WriteFile(configPath, data, 0o600)
}

// Helper functions

func (s *CLICenterService) getGeminiScriptPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(s.getGeminiScriptDir(), "gemini-codeswitch.bat")
	}
	return filepath.Join(s.getGeminiScriptDir(), "gemini-codeswitch")
}

func (s *CLICenterService) getGeminiScriptDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".code-switch", "scripts")
}

func (s *CLICenterService) createGeminiScript() error {
	scriptDir := s.getGeminiScriptDir()
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return err
	}

	baseURL := s.getBaseURL()
	scriptPath := s.getGeminiScriptPath()

	var content string
	if runtime.GOOS == "windows" {
		content = fmt.Sprintf(`@echo off
REM Gemini CLI with CodeSwitch Proxy
REM Generated at: %s

set GEMINI_API_BASE=%s/v1beta
gemini %%*
`, time.Now().Format(time.RFC3339), baseURL)
	} else {
		content = fmt.Sprintf(`#!/bin/bash
# Gemini CLI with CodeSwitch Proxy
# Generated at: %s

export GEMINI_API_BASE="%s/v1beta"
gemini "$@"
`, time.Now().Format(time.RFC3339), baseURL)
	}

	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		return err
	}

	return nil
}

func (s *CLICenterService) removeGeminiScript() error {
	scriptPath := s.getGeminiScriptPath()
	if err := os.Remove(scriptPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *CLICenterService) getBaseURL() string {
	addr := strings.TrimSpace(s.relayAddr)
	if addr == "" {
		addr = ":18100"
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return host
}

func (s *CLICenterService) isProxyServerRunning() bool {
	// Try to connect to the proxy server
	addr := strings.TrimPrefix(s.getBaseURL(), "http://")
	addr = strings.TrimPrefix(addr, "https://")

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (s *CLICenterService) checkCLIHealth(name, configPath string, validate func() error) HealthStatus {
	status := HealthStatus{
		ConfigExists: fileExists(configPath),
	}

	if !status.ConfigExists {
		status.Status = "warning"
		status.Message = fmt.Sprintf("%s config not found", name)
		return status
	}

	if err := validate(); err != nil {
		status.Status = "error"
		status.Message = fmt.Sprintf("%s config invalid: %v", name, err)
		status.ConfigValid = false
		return status
	}

	status.Status = "healthy"
	status.Message = fmt.Sprintf("%s configured correctly", name)
	status.ConfigValid = true
	return status
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
