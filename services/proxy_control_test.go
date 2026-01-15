package services

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDBForProxyControl creates a test database
func setupTestDBForProxyControl(t *testing.T) (*sql.DB, func()) {
	// Create temporary directory
	testDir, err := os.MkdirTemp("", "pc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(testDir, "test.db")

	// Create database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Create proxy_control table
	_, err = db.Exec(`
		CREATE TABLE proxy_control (
			app_name TEXT PRIMARY KEY,
			proxy_enabled INTEGER DEFAULT 1,
			proxy_mode TEXT DEFAULT 'shared',
			proxy_port INTEGER,
			intercept_domains TEXT,
			total_requests INTEGER DEFAULT 0,
			last_request_at DATETIME,
			last_toggled_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert default data
	_, err = db.Exec(`
		INSERT INTO proxy_control (app_name, proxy_enabled)
		VALUES ('claude', 1), ('codex', 1), ('gemini', 1)
	`)
	if err != nil {
		t.Fatalf("Failed to insert default data: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(testDir)
	}

	return db, cleanup
}

// Test: ProxyController creation
func TestProxyController_New(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	if pc == nil {
		t.Fatal("ProxyController should not be nil")
	}

	if pc.db != db {
		t.Fatal("ProxyController database mismatch")
	}
}

// Test: Load cache on initialization
func TestProxyController_LoadCache(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	// Check cache loaded correctly
	if !pc.IsProxyEnabled("claude") {
		t.Error("Expected claude proxy to be enabled by default")
	}
	if !pc.IsProxyEnabled("codex") {
		t.Error("Expected codex proxy to be enabled by default")
	}
	if !pc.IsProxyEnabled("gemini") {
		t.Error("Expected gemini proxy to be enabled by default")
	}
}

// Test: IsProxyEnabled with unknown app defaults to true
func TestProxyController_IsProxyEnabled_Unknown(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	// Unknown app should default to enabled
	if !pc.IsProxyEnabled("unknown") {
		t.Error("Expected unknown app to default to enabled")
	}
}

// Test: Toggle proxy enable/disable
func TestProxyController_ToggleProxy(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	// Initially enabled
	if !pc.IsProxyEnabled("claude") {
		t.Error("Expected claude proxy to be initially enabled")
	}

	// Disable
	err = pc.ToggleProxy("claude", false)
	if err != nil {
		t.Fatalf("Failed to toggle proxy: %v", err)
	}

	// Check cache updated
	if pc.IsProxyEnabled("claude") {
		t.Error("Expected claude proxy to be disabled after toggle")
	}

	// Check database updated
	var enabled int
	err = db.QueryRow("SELECT proxy_enabled FROM proxy_control WHERE app_name = 'claude'").Scan(&enabled)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}
	if enabled != 0 {
		t.Errorf("Expected database value to be 0, got %d", enabled)
	}

	// Re-enable
	err = pc.ToggleProxy("claude", true)
	if err != nil {
		t.Fatalf("Failed to toggle proxy back: %v", err)
	}

	// Check cache updated
	if !pc.IsProxyEnabled("claude") {
		t.Error("Expected claude proxy to be re-enabled")
	}
}

// Test: Get config for an app
func TestProxyController_GetConfig(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	config, err := pc.GetConfig("claude")
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.AppName != "claude" {
		t.Errorf("Expected app_name 'claude', got '%s'", config.AppName)
	}

	if !config.ProxyEnabled {
		t.Error("Expected proxy to be enabled")
	}

	if config.ProxyMode != ProxyModeShared {
		t.Errorf("Expected proxy mode 'shared', got '%s'", config.ProxyMode)
	}
}

// Test: Get all configs
func TestProxyController_GetAllConfigs(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	configs, err := pc.GetAllConfigs()
	if err != nil {
		t.Fatalf("Failed to get all configs: %v", err)
	}

	if len(configs) != 3 {
		t.Errorf("Expected 3 configs, got %d", len(configs))
	}

	// Check app names
	appNames := make(map[string]bool)
	for _, config := range configs {
		appNames[config.AppName] = true
	}

	expectedApps := []string{"claude", "codex", "gemini"}
	for _, app := range expectedApps {
		if !appNames[app] {
			t.Errorf("Expected app '%s' in configs", app)
		}
	}
}

// Test: Record request increments counter
func TestProxyController_RecordRequest(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	// Record 3 requests
	for i := 0; i < 3; i++ {
		err = pc.RecordRequest("claude")
		if err != nil {
			t.Fatalf("Failed to record request: %v", err)
		}
	}

	// Give a small delay for database write
	time.Sleep(10 * time.Millisecond)

	// Check database
	var totalRequests int
	var lastRequestAt sql.NullTime
	err = db.QueryRow(`
		SELECT total_requests, last_request_at
		FROM proxy_control
		WHERE app_name = 'claude'
	`).Scan(&totalRequests, &lastRequestAt)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if totalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", totalRequests)
	}

	if !lastRequestAt.Valid {
		t.Error("Expected last_request_at to be set")
	}
}

// Test: Get stats
func TestProxyController_GetStats(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	// Record some requests
	pc.RecordRequest("claude")
	pc.RecordRequest("claude")
	pc.RecordRequest("codex")

	// Give a small delay for database write
	time.Sleep(10 * time.Millisecond)

	stats, err := pc.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if len(stats) != 3 {
		t.Errorf("Expected 3 app stats, got %d", len(stats))
	}

	// Check claude stats
	claudeStats, exists := stats["claude"]
	if !exists {
		t.Fatal("Expected claude in stats")
	}

	if claudeStats.AppName != "claude" {
		t.Errorf("Expected app_name 'claude', got '%s'", claudeStats.AppName)
	}

	if claudeStats.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests for claude, got %d", claudeStats.TotalRequests)
	}

	if !claudeStats.Enabled {
		t.Error("Expected claude to be enabled")
	}
}

// Test: Update config
func TestProxyController_UpdateConfig(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	// Update config
	config := ProxyControlConfig{
		AppName:          "claude",
		ProxyEnabled:     false,
		ProxyMode:        ProxyModeDedicated,
		ProxyPort:        18101,
		InterceptDomains: []string{"api.anthropic.com", "claude.ai"},
	}

	err = pc.UpdateConfig(config)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Check cache updated
	if pc.IsProxyEnabled("claude") {
		t.Error("Expected claude proxy to be disabled after update")
	}

	// Check database
	updatedConfig, err := pc.GetConfig("claude")
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	if updatedConfig.ProxyEnabled {
		t.Error("Expected proxy to be disabled")
	}

	if updatedConfig.ProxyMode != ProxyModeDedicated {
		t.Errorf("Expected proxy mode 'dedicated', got '%s'", updatedConfig.ProxyMode)
	}

	if updatedConfig.ProxyPort != 18101 {
		t.Errorf("Expected proxy port 18101, got %d", updatedConfig.ProxyPort)
	}

	if len(updatedConfig.InterceptDomains) != 2 {
		t.Errorf("Expected 2 intercept domains, got %d", len(updatedConfig.InterceptDomains))
	}
}

// Test: Refresh cache
func TestProxyController_RefreshCache(t *testing.T) {
	db, cleanup := setupTestDBForProxyControl(t)
	defer cleanup()

	pc, err := NewProxyController(db)
	if err != nil {
		t.Fatalf("Failed to create ProxyController: %v", err)
	}

	// Manually update database (bypassing controller)
	_, err = db.Exec("UPDATE proxy_control SET proxy_enabled = 0 WHERE app_name = 'claude'")
	if err != nil {
		t.Fatalf("Failed to update database: %v", err)
	}

	// Cache should still have old value
	if !pc.IsProxyEnabled("claude") {
		t.Error("Expected cache to still have old value (enabled)")
	}

	// Refresh cache
	err = pc.RefreshCache()
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	// Cache should now reflect database
	if pc.IsProxyEnabled("claude") {
		t.Error("Expected cache to be updated after refresh (disabled)")
	}
}

// Test: Normalize app name
func TestNormalizeAppName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"claude", "claude"},
		{"Claude", "claude"},
		{"CLAUDE", "claude"},
		{"claude-code", "claude"},
		{"claude_code", "claude"},
		{"codex", "codex"},
		{"gemini", "gemini"},
		{"gemini-cli", "gemini"},
		{"gemini_cli", "gemini"},
		{"unknown", "unknown"},
	}

	for _, test := range tests {
		result := normalizeAppName(test.input)
		if result != test.expected {
			t.Errorf("normalizeAppName(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

// Benchmark: IsProxyEnabled lookup
func BenchmarkProxyController_IsProxyEnabled(b *testing.B) {
	db, cleanup := setupTestDBForProxyControl(&testing.T{})
	defer cleanup()

	pc, _ := NewProxyController(db)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = pc.IsProxyEnabled("claude")
	}
}

// Benchmark: ToggleProxy
func BenchmarkProxyController_ToggleProxy(b *testing.B) {
	db, cleanup := setupTestDBForProxyControl(&testing.T{})
	defer cleanup()

	pc, _ := NewProxyController(db)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = pc.ToggleProxy("claude", i%2 == 0)
	}
}
