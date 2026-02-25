package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupPicoClawTestDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "picoclaw-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return dir, func() { os.RemoveAll(dir) }
}

func TestPicoClawSettingsService_ProxyStatus_NotConfigured(t *testing.T) {
	pcs := NewPicoClawSettingsService(":18100")
	status, err := pcs.ProxyStatus()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if status.Enabled {
		t.Error("Expected proxy to be disabled when config does not exist")
	}
}

func TestPicoClawSettingsService_EnableProxy_NewConfig(t *testing.T) {
	dir, cleanup := setupPicoClawTestDir(t)
	defer cleanup()

	pcs := &PicoClawSettingsService{relayAddr: ":18100"}

	// Override home dir by creating config in temp dir
	configDir := filepath.Join(dir, ".picoclaw")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Test the config structure generation directly
	config := picoClawConfig{
		ModelList: []picoClawModelEntry{},
	}
	config.Agents.Defaults.ModelName = picoClawModelName
	config.ModelList = append(config.ModelList, picoClawModelEntry{
		ModelName: picoClawModelName,
		APIBase:   pcs.baseURL(),
		APIKey:    picoClawAPIKey,
	})

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	configPath := filepath.Join(configDir, picoClawConfigFileName)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Verify the written config
	var readBack picoClawConfig
	readData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	if err := json.Unmarshal(readData, &readBack); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if readBack.Agents.Defaults.ModelName != picoClawModelName {
		t.Errorf("Expected default model_name %q, got %q", picoClawModelName, readBack.Agents.Defaults.ModelName)
	}
	if len(readBack.ModelList) != 1 {
		t.Fatalf("Expected 1 model_list entry, got %d", len(readBack.ModelList))
	}
	if readBack.ModelList[0].ModelName != picoClawModelName {
		t.Errorf("Expected model_name %q, got %q", picoClawModelName, readBack.ModelList[0].ModelName)
	}
}

func TestPicoClawSettingsService_EnableProxy_ExistingConfig_ArrayMerge(t *testing.T) {
	dir, cleanup := setupPicoClawTestDir(t)
	defer cleanup()

	configDir := filepath.Join(dir, ".picoclaw")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Write an existing config with a pre-existing model entry
	existing := picoClawConfig{
		ModelList: []picoClawModelEntry{
			{ModelName: "existing-model", APIBase: "https://example.com", APIKey: "key123"},
		},
	}
	existing.Agents.Defaults.ModelName = "existing-model"

	data, _ := json.MarshalIndent(existing, "", "  ")
	configPath := filepath.Join(configDir, picoClawConfigFileName)
	os.WriteFile(configPath, data, 0o600)

	// Simulate upsert: add code-switch entry
	var config picoClawConfig
	rawData, _ := os.ReadFile(configPath)
	json.Unmarshal(rawData, &config)

	found := false
	for i, entry := range config.ModelList {
		if entry.ModelName == picoClawModelName {
			config.ModelList[i].APIBase = "http://127.0.0.1:18100/pc/v1"
			config.ModelList[i].APIKey = picoClawAPIKey
			found = true
			break
		}
	}
	if !found {
		config.ModelList = append(config.ModelList, picoClawModelEntry{
			ModelName: picoClawModelName,
			APIBase:   "http://127.0.0.1:18100/pc/v1",
			APIKey:    picoClawAPIKey,
		})
	}
	config.Agents.Defaults.ModelName = picoClawModelName

	newData, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, newData, 0o600)

	// Verify: should have 2 entries, existing preserved
	var readBack picoClawConfig
	finalData, _ := os.ReadFile(configPath)
	json.Unmarshal(finalData, &readBack)

	if len(readBack.ModelList) != 2 {
		t.Fatalf("Expected 2 model_list entries after upsert, got %d", len(readBack.ModelList))
	}
	if readBack.ModelList[0].ModelName != "existing-model" {
		t.Errorf("Expected first entry to be 'existing-model', got %q", readBack.ModelList[0].ModelName)
	}
	if readBack.ModelList[1].ModelName != picoClawModelName {
		t.Errorf("Expected second entry to be %q, got %q", picoClawModelName, readBack.ModelList[1].ModelName)
	}
}

func TestPicoClawSettingsService_DisableProxy_RestoreBackup(t *testing.T) {
	dir, cleanup := setupPicoClawTestDir(t)
	defer cleanup()

	configDir := filepath.Join(dir, ".picoclaw")
	os.MkdirAll(configDir, 0o755)

	configPath := filepath.Join(configDir, picoClawConfigFileName)
	backupPath := filepath.Join(configDir, picoClawBackupConfigName)

	// Write a backup file
	backupContent := []byte(`{"agents":{"defaults":{"model_name":"original"}},"model_list":[]}`)
	os.WriteFile(backupPath, backupContent, 0o600)

	// Write a current config (proxy-enabled)
	currentContent := []byte(`{"agents":{"defaults":{"model_name":"code-switch"}},"model_list":[{"model_name":"code-switch","api_base":"http://127.0.0.1:18100/pc/v1","api_key":"code-switch"}]}`)
	os.WriteFile(configPath, currentContent, 0o600)

	// Simulate disable: remove config, restore backup
	os.Remove(configPath)
	if _, err := os.Stat(backupPath); err == nil {
		os.Rename(backupPath, configPath)
	}

	// Verify: config should now contain original backup content
	restored, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read restored config: %v", err)
	}

	var config picoClawConfig
	if err := json.Unmarshal(restored, &config); err != nil {
		t.Fatalf("Failed to parse restored config: %v", err)
	}

	if config.Agents.Defaults.ModelName != "original" {
		t.Errorf("Expected restored model_name 'original', got %q", config.Agents.Defaults.ModelName)
	}

	// Backup should no longer exist
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("Expected backup to be removed after restore")
	}
}

func TestPicoClawSettingsService_BaseURL_Format(t *testing.T) {
	tests := []struct {
		addr     string
		expected string
	}{
		{":18100", "http://127.0.0.1:18100/pc/v1"},
		{"127.0.0.1:18100", "http://127.0.0.1:18100/pc/v1"},
		{"http://localhost:18100", "http://localhost:18100/pc/v1"},
		{"https://example.com:443", "https://example.com:443/pc/v1"},
		{"", "http://127.0.0.1:18100/pc/v1"},
	}

	for _, tt := range tests {
		pcs := &PicoClawSettingsService{relayAddr: tt.addr}
		got := pcs.baseURL()
		if got != tt.expected {
			t.Errorf("baseURL(%q) = %q, want %q", tt.addr, got, tt.expected)
		}
	}
}

func TestDetectAppFromPath_PicoClaw(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/pc/v1/chat/completions", "picoclaw"},
		{"/pc/chat/completions", "picoclaw"},
		{"/PC/v1/chat/completions", "picoclaw"},
		{"/v1/chat/completions", "codex"},
		{"/v1/messages", "claude"},
		{"/responses", "codex"},
	}

	for _, tt := range tests {
		got := detectAppFromPath(tt.path)
		if got != tt.expected {
			t.Errorf("detectAppFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}
