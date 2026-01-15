package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// TestMCPService_UnifiedArchitecture tests the unified MCP architecture with 3 platforms
func TestMCPService_UnifiedArchitecture(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	if originalHome == "" {
		originalHome = os.Getenv("USERPROFILE")
	}
	defer func() {
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
			os.Setenv("USERPROFILE", originalHome)
		}
	}()

	// Set temporary home directory
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	// Create test directories
	mcpDir := filepath.Join(tmpDir, mcpStoreDir)
	geminiDir := filepath.Join(tmpDir, geminiDirName)
	codexDir := filepath.Join(tmpDir, codexDirName)

	os.MkdirAll(mcpDir, 0o755)
	os.MkdirAll(geminiDir, 0o755)
	os.MkdirAll(codexDir, 0o755)

	ms := NewMCPService()

	// Test 1: Save servers with all 3 types and 3 platforms
	servers := []MCPServer{
		{
			Name:           "test-stdio",
			Type:           "stdio",
			Command:        "node",
			Args:           []string{"server.js"},
			Env:            map[string]string{"PATH": "/usr/bin"},
			EnablePlatform: []string{"claude-code", "codex", "gemini-cli"},
		},
		{
			Name:           "test-http",
			Type:           "http",
			URL:            "https://api.example.com/mcp",
			EnablePlatform: []string{"claude-code", "gemini-cli"},
		},
		{
			Name:           "test-sse",
			Type:           "sse",
			URL:            "https://api.example.com/mcp/sse",
			EnablePlatform: []string{"gemini-cli"},
		},
	}

	err := ms.SaveServers(servers)
	if err != nil {
		t.Fatalf("Failed to save servers: %v", err)
	}

	// Test 2: Verify config files were created correctly

	// Verify main config
	mcpConfigPath := filepath.Join(mcpDir, mcpStoreFile)
	if _, err := os.Stat(mcpConfigPath); os.IsNotExist(err) {
		t.Errorf("Main MCP config file was not created")
	}

	// Verify Claude config
	claudeConfigPath := filepath.Join(tmpDir, claudeMcpFile)
	if _, err := os.Stat(claudeConfigPath); os.IsNotExist(err) {
		t.Errorf("Claude config file was not created")
	}

	// Verify Codex config
	codexConfigPath := filepath.Join(codexDir, codexConfigFile)
	if _, err := os.Stat(codexConfigPath); os.IsNotExist(err) {
		t.Errorf("Codex config file was not created")
	}

	// Verify Gemini config
	geminiConfigPath := filepath.Join(geminiDir, geminiConfigFile)
	if _, err := os.Stat(geminiConfigPath); os.IsNotExist(err) {
		t.Errorf("Gemini config file was not created")
	}

	// Test 3: Verify Claude config content
	claudeData, _ := os.ReadFile(claudeConfigPath)
	var claudePayload struct {
		MCPServers map[string]claudeDesktopServer `json:"mcpServers"`
	}
	json.Unmarshal(claudeData, &claudePayload)

	if len(claudePayload.MCPServers) != 2 {
		t.Errorf("Expected 2 servers in Claude config, got %d", len(claudePayload.MCPServers))
	}

	if claudePayload.MCPServers["test-stdio"].Type != "stdio" {
		t.Errorf("Expected stdio type in Claude config")
	}

	if claudePayload.MCPServers["test-http"].Type != "http" {
		t.Errorf("Expected http type in Claude config")
	}

	// Test 4: Verify Codex config content
	codexData, _ := os.ReadFile(codexConfigPath)
	var codexPayload struct {
		MCPServers map[string]map[string]any `toml:"mcp_servers"`
	}
	toml.Unmarshal(codexData, &codexPayload)

	if len(codexPayload.MCPServers) != 1 {
		t.Errorf("Expected 1 server in Codex config, got %d", len(codexPayload.MCPServers))
	}

	if codexPayload.MCPServers["test-stdio"]["type"] != "stdio" {
		t.Errorf("Expected stdio type in Codex config")
	}

	// Test 5: Verify Gemini config content
	geminiData, _ := os.ReadFile(geminiConfigPath)
	var geminiPayload struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	json.Unmarshal(geminiData, &geminiPayload)

	if len(geminiPayload.MCPServers) != 3 {
		t.Errorf("Expected 3 servers in Gemini config, got %d", len(geminiPayload.MCPServers))
	}

	if geminiPayload.MCPServers["test-stdio"]["type"] != "stdio" {
		t.Errorf("Expected stdio type in Gemini config")
	}

	if geminiPayload.MCPServers["test-http"]["type"] != "http" {
		t.Errorf("Expected http type in Gemini config")
	}

	if geminiPayload.MCPServers["test-sse"]["type"] != "sse" {
		t.Errorf("Expected sse type in Gemini config")
	}

	// Verify SSE has URL
	if geminiPayload.MCPServers["test-sse"]["url"] == "" {
		t.Errorf("SSE server should have URL")
	}
}

// TestMCPService_SSETypeSupport tests SSE type validation and normalization
func TestMCPService_SSETypeSupport(t *testing.T) {
	// Test normalizeServerType with SSE
	tests := []struct {
		input    string
		expected string
	}{
		{"stdio", "stdio"},
		{"STDIO", "stdio"},
		{"http", "http"},
		{"HTTP", "http"},
		{"sse", "sse"},
		{"SSE", "sse"},
		{"", "stdio"},          // default
		{"unknown", "stdio"},   // fallback
		{"  sse  ", "sse"},     // with spaces
	}

	for _, test := range tests {
		result := normalizeServerType(test.input)
		if result != test.expected {
			t.Errorf("normalizeServerType(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// TestMCPService_GeminiPlatformSupport tests Gemini platform validation
func TestMCPService_GeminiPlatformSupport(t *testing.T) {
	// Test normalizePlatform with Gemini variations
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		{"gemini", "gemini-cli", true},
		{"GEMINI", "gemini-cli", true},
		{"gemini-cli", "gemini-cli", true},
		{"gemini_cli", "gemini-cli", true},
		{"  gemini  ", "gemini-cli", true},
		{"claude-code", "claude-code", true},
		{"codex", "codex", true},
		{"unknown", "", false},
	}

	for _, test := range tests {
		result, valid := normalizePlatform(test.input)
		if valid != test.valid {
			t.Errorf("normalizePlatform(%q) valid = %v, expected %v", test.input, valid, test.valid)
		}
		if valid && result != test.expected {
			t.Errorf("normalizePlatform(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// TestMCPService_SSERequiresURL tests that SSE servers require URL
func TestMCPService_SSERequiresURL(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	mcpDir := filepath.Join(tmpDir, mcpStoreDir)
	os.MkdirAll(mcpDir, 0o755)

	ms := NewMCPService()

	// Test: SSE without URL should fail
	servers := []MCPServer{
		{
			Name:           "invalid-sse",
			Type:           "sse",
			URL:            "", // Missing URL
			EnablePlatform: []string{"gemini-cli"},
		},
	}

	err := ms.SaveServers(servers)
	if err == nil {
		t.Errorf("Expected error for SSE server without URL, got nil")
	}
	if err != nil && err.Error() != "invalid-sse 需要提供 url" {
		t.Errorf("Expected URL validation error, got: %v", err)
	}

	// Test: SSE with URL should succeed
	servers[0].URL = "https://api.example.com/sse"
	err = ms.SaveServers(servers)
	if err != nil {
		t.Errorf("SSE server with URL should succeed, got error: %v", err)
	}
}

// TestMCPService_GeminiConfigFormat tests Gemini config file format
func TestMCPService_GeminiConfigFormat(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	geminiDir := filepath.Join(tmpDir, geminiDirName)
	os.MkdirAll(geminiDir, 0o755)

	ms := NewMCPService()

	servers := []MCPServer{
		{
			Name:           "gemini-test",
			Type:           "stdio",
			Command:        "python",
			Args:           []string{"-m", "mcp_server"},
			Env:            map[string]string{"PYTHON_PATH": "/usr/bin/python"},
			EnablePlatform: []string{"gemini-cli"},
		},
	}

	err := ms.SaveServers(servers)
	if err != nil {
		t.Fatalf("Failed to save servers: %v", err)
	}

	// Read and verify Gemini config
	geminiConfigPath := filepath.Join(geminiDir, geminiConfigFile)
	data, err := os.ReadFile(geminiConfigPath)
	if err != nil {
		t.Fatalf("Failed to read Gemini config: %v", err)
	}

	var payload struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	err = json.Unmarshal(data, &payload)
	if err != nil {
		t.Fatalf("Failed to parse Gemini config: %v", err)
	}

	server := payload.MCPServers["gemini-test"]
	if server == nil {
		t.Fatalf("Server 'gemini-test' not found in Gemini config")
	}

	// Verify structure
	if server["type"] != "stdio" {
		t.Errorf("Expected type 'stdio', got %v", server["type"])
	}

	if server["command"] != "python" {
		t.Errorf("Expected command 'python', got %v", server["command"])
	}

	args := server["args"].([]interface{})
	if len(args) != 2 || args[0] != "-m" || args[1] != "mcp_server" {
		t.Errorf("Args mismatch: %v", args)
	}

	env := server["env"].(map[string]interface{})
	if env["PYTHON_PATH"] != "/usr/bin/python" {
		t.Errorf("Env mismatch: %v", env)
	}
}

// TestMCPService_MultiPlatformSync tests syncing to all 3 platforms
func TestMCPService_MultiPlatformSync(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	// Create all directories
	mcpDir := filepath.Join(tmpDir, mcpStoreDir)
	geminiDir := filepath.Join(tmpDir, geminiDirName)
	codexDir := filepath.Join(tmpDir, codexDirName)

	os.MkdirAll(mcpDir, 0o755)
	os.MkdirAll(geminiDir, 0o755)
	os.MkdirAll(codexDir, 0o755)

	ms := NewMCPService()

	// Server enabled for all 3 platforms
	servers := []MCPServer{
		{
			Name:           "universal-server",
			Type:           "http",
			URL:            "https://api.universal.com/mcp",
			EnablePlatform: []string{"claude-code", "codex", "gemini-cli"},
		},
	}

	err := ms.SaveServers(servers)
	if err != nil {
		t.Fatalf("Failed to save servers: %v", err)
	}

	// Verify presence in all 3 configs
	claudePath := filepath.Join(tmpDir, claudeMcpFile)
	codexPath := filepath.Join(codexDir, codexConfigFile)
	geminiPath := filepath.Join(geminiDir, geminiConfigFile)

	// Check Claude
	claudeData, _ := os.ReadFile(claudePath)
	var claudePayload map[string]any
	json.Unmarshal(claudeData, &claudePayload)
	claudeServers := claudePayload["mcpServers"].(map[string]any)
	if claudeServers["universal-server"] == nil {
		t.Errorf("Server not found in Claude config")
	}

	// Check Codex
	codexData, _ := os.ReadFile(codexPath)
	var codexPayload map[string]any
	toml.Unmarshal(codexData, &codexPayload)
	codexServers := codexPayload["mcp_servers"].(map[string]any)
	if codexServers["universal-server"] == nil {
		t.Errorf("Server not found in Codex config")
	}

	// Check Gemini
	geminiData, _ := os.ReadFile(geminiPath)
	var geminiPayload map[string]any
	json.Unmarshal(geminiData, &geminiPayload)
	geminiServers := geminiPayload["mcpServers"].(map[string]any)
	if geminiServers["universal-server"] == nil {
		t.Errorf("Server not found in Gemini config")
	}
}
