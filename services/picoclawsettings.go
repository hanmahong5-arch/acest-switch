package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	picoClawSettingsDir      = ".picoclaw"
	picoClawConfigFileName   = "config.json"
	picoClawBackupConfigName = "cc-studio.back.config.json"
	picoClawModelName        = "code-switch"
	picoClawAPIKey           = "code-switch"
	picoClawPathPrefix       = "/pc/v1"
)

// PicoClawSettingsService manages PicoClaw CLI proxy configuration.
// PicoClaw uses ~/.picoclaw/config.json with a model_list array format.
type PicoClawSettingsService struct {
	relayAddr string
}

func NewPicoClawSettingsService(relayAddr string) *PicoClawSettingsService {
	return &PicoClawSettingsService{relayAddr: relayAddr}
}

func (pcs *PicoClawSettingsService) ProxyStatus() (ClaudeProxyStatus, error) {
	status := ClaudeProxyStatus{Enabled: false, BaseURL: pcs.baseURL()}

	config, err := pcs.readConfig()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return status, nil
		}
		return status, err
	}

	baseURL := pcs.baseURL()
	for _, entry := range config.ModelList {
		if entry.ModelName == picoClawModelName && strings.EqualFold(entry.APIBase, baseURL) {
			// Also check if defaults point to our model
			if config.Agents.Defaults.ModelName == picoClawModelName {
				status.Enabled = true
			}
			break
		}
	}

	return status, nil
}

func (pcs *PicoClawSettingsService) EnableProxy() error {
	settingsPath, backupPath, err := pcs.paths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	var config picoClawConfig
	if _, statErr := os.Stat(settingsPath); statErr == nil {
		content, readErr := os.ReadFile(settingsPath)
		if readErr != nil {
			return readErr
		}
		// Backup existing config
		if err := os.WriteFile(backupPath, content, 0o600); err != nil {
			return err
		}
		if err := json.Unmarshal(content, &config); err != nil {
			return err
		}
	} else {
		// Create skeleton config
		config = picoClawConfig{
			ModelList: []picoClawModelEntry{},
		}
	}
	if config.ModelList == nil {
		config.ModelList = []picoClawModelEntry{}
	}

	// Upsert the code-switch entry in model_list
	baseURL := pcs.baseURL()
	found := false
	for i, entry := range config.ModelList {
		if entry.ModelName == picoClawModelName {
			config.ModelList[i].APIBase = baseURL
			config.ModelList[i].APIKey = picoClawAPIKey
			found = true
			break
		}
	}
	if !found {
		config.ModelList = append(config.ModelList, picoClawModelEntry{
			ModelName: picoClawModelName,
			APIBase:   baseURL,
			APIKey:    picoClawAPIKey,
		})
	}

	// Set defaults to use our model
	config.Agents.Defaults.ModelName = picoClawModelName

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, data, 0o600)
}

func (pcs *PicoClawSettingsService) DisableProxy() error {
	settingsPath, backupPath, err := pcs.paths()
	if err != nil {
		return err
	}

	if err := os.Remove(settingsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Restore from backup if available
	if _, err := os.Stat(backupPath); err == nil {
		if err := os.Rename(backupPath, settingsPath); err != nil {
			return err
		}
	}
	return nil
}

func (pcs *PicoClawSettingsService) readConfig() (*picoClawConfig, error) {
	settingsPath, _, err := pcs.paths()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}
	var cfg picoClawConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (pcs *PicoClawSettingsService) paths() (settingsPath string, backupPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, picoClawSettingsDir)
	return filepath.Join(dir, picoClawConfigFileName), filepath.Join(dir, picoClawBackupConfigName), nil
}

func (pcs *PicoClawSettingsService) baseURL() string {
	addr := strings.TrimSpace(pcs.relayAddr)
	if addr == "" {
		addr = ":18100"
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/") + picoClawPathPrefix
	}
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return strings.TrimRight(host, "/") + picoClawPathPrefix
}

// picoClawConfig represents ~/.picoclaw/config.json
type picoClawConfig struct {
	Agents    picoClawAgents       `json:"agents"`
	ModelList []picoClawModelEntry `json:"model_list"`
}

type picoClawAgents struct {
	Defaults picoClawDefaults `json:"defaults"`
}

type picoClawDefaults struct {
	ModelName string `json:"model_name"`
}

type picoClawModelEntry struct {
	ModelName string `json:"model_name"`
	APIBase   string `json:"api_base"`
	APIKey    string `json:"api_key"`
}
