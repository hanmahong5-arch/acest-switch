package services

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB creates a temporary test database with schema
func setupTestDB(t *testing.T) (*sql.DB, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath+"?cache=shared&mode=rwc&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create tables
	schema := `
	CREATE TABLE IF NOT EXISTS provider_config (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		name TEXT NOT NULL,
		api_url TEXT NOT NULL,
		api_key TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		supported_models TEXT,
		model_mapping TEXT,
		priority_level INTEGER DEFAULT 1,
		tint TEXT,
		accent TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(platform, name)
	);

	CREATE TABLE IF NOT EXISTS proxy_live_backup (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		backup_type TEXT NOT NULL,
		backup_data TEXT NOT NULL,
		trigger_event TEXT,
		backup_time DATETIME DEFAULT CURRENT_TIMESTAMP,
		restored INTEGER DEFAULT 0,
		restored_at DATETIME
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db, tmpDir
}

// TestConfigRecovery_CreateAndRemoveCrashMarker tests crash marker lifecycle
func TestConfigRecovery_CreateAndRemoveCrashMarker(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Test marker creation
	err := cr.CreateCrashMarker()
	if err != nil {
		t.Fatalf("Failed to create crash marker: %v", err)
	}

	// Verify marker file exists
	if _, err := os.Stat(cr.crashMarker); os.IsNotExist(err) {
		t.Errorf("Crash marker file was not created")
	}

	// Test marker removal
	err = cr.RemoveCrashMarker()
	if err != nil {
		t.Fatalf("Failed to remove crash marker: %v", err)
	}

	// Verify marker file is removed
	if _, err := os.Stat(cr.crashMarker); !os.IsNotExist(err) {
		t.Errorf("Crash marker file was not removed")
	}
}

// TestConfigRecovery_DetectAbnormalShutdown tests crash detection
func TestConfigRecovery_DetectAbnormalShutdown(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// First startup - no crash
	crashed, err := cr.DetectAbnormalShutdown()
	if err != nil {
		t.Fatalf("Failed to detect shutdown: %v", err)
	}
	if crashed {
		t.Errorf("Expected no crash on first startup, got crashed=true")
	}

	// Verify marker was created
	if _, err := os.Stat(cr.crashMarker); os.IsNotExist(err) {
		t.Errorf("Crash marker should be created on first startup")
	}

	// Simulate crash by not removing marker
	// Second startup - crash detected
	cr2 := NewConfigRecovery(db, tmpDir)
	crashed, err = cr2.DetectAbnormalShutdown()
	if err != nil {
		t.Fatalf("Failed to detect shutdown: %v", err)
	}
	if !crashed {
		t.Errorf("Expected crash detection, got crashed=false")
	}
}

// TestConfigRecovery_CreateBackup tests manual backup creation
func TestConfigRecovery_CreateBackup(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create test backup data
	backupData := map[string]interface{}{
		"provider_id": 1,
		"snapshot": map[string]interface{}{
			"name":    "Test Provider",
			"api_url": "https://api.test.com",
			"enabled": 1,
		},
	}

	// Create backup
	err := cr.CreateBackup(BackupTypeProvider, backupData, "manual_test")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Verify backup was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM proxy_live_backup WHERE backup_type = ?", string(BackupTypeProvider)).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query backup: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 backup, got %d", count)
	}
}

// TestConfigRecovery_GetLatestBackups tests backup retrieval
func TestConfigRecovery_GetLatestBackups(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create multiple backups
	backupTypes := []BackupType{BackupTypeProvider, BackupTypeAppSetting, BackupTypeMCP}
	for _, bt := range backupTypes {
		data := map[string]interface{}{
			"test_field": "test_value",
		}
		err := cr.CreateBackup(bt, data, "test")
		if err != nil {
			t.Fatalf("Failed to create backup: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Get latest backups
	backups, err := cr.GetLatestBackups()
	if err != nil {
		t.Fatalf("Failed to get latest backups: %v", err)
	}

	// Verify we got one backup per type
	if len(backups) != 3 {
		t.Errorf("Expected 3 backups, got %d", len(backups))
	}

	// Verify backup types
	types := make(map[string]bool)
	for _, b := range backups {
		types[b.BackupType] = true
	}
	for _, bt := range backupTypes {
		if !types[string(bt)] {
			t.Errorf("Missing backup type: %s", bt)
		}
	}
}

// TestConfigRecovery_RestoreProviderConfig tests provider config restoration
func TestConfigRecovery_RestoreProviderConfig(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Insert a provider
	_, err := db.Exec(`
		INSERT INTO provider_config (id, platform, name, api_url, api_key, enabled)
		VALUES (1, 'claude', 'Test Provider', 'https://api.test.com', 'test-key', 1)
	`)
	if err != nil {
		t.Fatalf("Failed to insert provider: %v", err)
	}

	// Create backup data with snapshot
	backupData := map[string]interface{}{
		"provider_id": 1,
		"snapshot": map[string]interface{}{
			"id":      1,
			"name":    "Test Provider",
			"api_url": "https://api.test.com",
			"api_key": "test-key",
			"enabled": 0, // Changed to disabled
		},
	}
	backupJSON, _ := json.Marshal(backupData)

	// Restore from backup
	err = cr.restoreProviderConfig(string(backupJSON))
	if err != nil {
		t.Fatalf("Failed to restore provider config: %v", err)
	}

	// Verify provider was updated
	var enabled int
	err = db.QueryRow("SELECT enabled FROM provider_config WHERE id = 1").Scan(&enabled)
	if err != nil {
		t.Fatalf("Failed to query provider: %v", err)
	}
	if enabled != 0 {
		t.Errorf("Expected enabled=0, got %d", enabled)
	}
}

// TestConfigRecovery_RestoreAppSettings tests app settings restoration
func TestConfigRecovery_RestoreAppSettings(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create backup data
	settings := map[string]interface{}{
		"auto_start":      true,
		"show_heatmap":    true,
		"new_api_enabled": false,
	}
	settingsJSON, _ := json.Marshal(settings)
	backupData := map[string]interface{}{
		"settings": string(settingsJSON),
	}
	backupJSON, _ := json.Marshal(backupData)

	// Restore from backup
	err := cr.restoreAppSettings(string(backupJSON))
	if err != nil {
		t.Fatalf("Failed to restore app settings: %v", err)
	}

	// Verify file was created
	appSettingsPath := filepath.Join(tmpDir, "app.json")
	if _, err := os.Stat(appSettingsPath); os.IsNotExist(err) {
		t.Errorf("App settings file was not created")
	}

	// Verify content
	data, err := os.ReadFile(appSettingsPath)
	if err != nil {
		t.Fatalf("Failed to read app settings: %v", err)
	}
	if string(data) != string(settingsJSON) {
		t.Errorf("App settings content mismatch")
	}
}

// TestConfigRecovery_RestoreMCPConfig tests MCP config restoration
func TestConfigRecovery_RestoreMCPConfig(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create backup data
	mcpServers := map[string]interface{}{
		"server1": map[string]interface{}{
			"command": "node",
			"args":    []string{"server.js"},
		},
	}
	mcpJSON, _ := json.Marshal(mcpServers)
	backupData := map[string]interface{}{
		"mcp_servers": string(mcpJSON),
	}
	backupJSON, _ := json.Marshal(backupData)

	// Restore from backup
	err := cr.restoreMCPConfig(string(backupJSON))
	if err != nil {
		t.Fatalf("Failed to restore MCP config: %v", err)
	}

	// Verify file was created
	mcpPath := filepath.Join(tmpDir, "mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		t.Errorf("MCP config file was not created")
	}

	// Verify content
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("Failed to read MCP config: %v", err)
	}
	if string(data) != string(mcpJSON) {
		t.Errorf("MCP config content mismatch")
	}
}

// TestConfigRecovery_RecoverFromCrash tests full crash recovery
func TestConfigRecovery_RecoverFromCrash(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create test backups for all types
	backupTypes := []BackupType{BackupTypeProvider, BackupTypeAppSetting, BackupTypeMCP}
	for _, bt := range backupTypes {
		var data map[string]interface{}
		switch bt {
		case BackupTypeProvider:
			data = map[string]interface{}{
				"provider_id": 1,
				"snapshot": map[string]interface{}{
					"name":    "Test Provider",
					"api_url": "https://api.test.com",
					"enabled": 1,
				},
			}
		case BackupTypeAppSetting:
			settings := map[string]interface{}{"auto_start": true}
			settingsJSON, _ := json.Marshal(settings)
			data = map[string]interface{}{
				"settings": string(settingsJSON),
			}
		case BackupTypeMCP:
			mcp := map[string]interface{}{"server1": map[string]interface{}{}}
			mcpJSON, _ := json.Marshal(mcp)
			data = map[string]interface{}{
				"mcp_servers": string(mcpJSON),
			}
		}

		err := cr.CreateBackup(bt, data, "test")
		if err != nil {
			t.Fatalf("Failed to create backup: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Perform recovery
	err := cr.RecoverFromCrash()
	if err != nil {
		t.Fatalf("Failed to recover from crash: %v", err)
	}

	// Verify backups were marked as restored
	var restoredCount int
	err = db.QueryRow("SELECT COUNT(*) FROM proxy_live_backup WHERE restored = 1").Scan(&restoredCount)
	if err != nil {
		t.Fatalf("Failed to query restored backups: %v", err)
	}
	if restoredCount != 3 {
		t.Errorf("Expected 3 restored backups, got %d", restoredCount)
	}

	// Verify config files were created
	appSettingsPath := filepath.Join(tmpDir, "app.json")
	if _, err := os.Stat(appSettingsPath); os.IsNotExist(err) {
		t.Errorf("App settings file was not created during recovery")
	}

	mcpPath := filepath.Join(tmpDir, "mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		t.Errorf("MCP config file was not created during recovery")
	}
}

// TestConfigRecovery_RestoreFromBackup tests manual recovery by backup ID
func TestConfigRecovery_RestoreFromBackup(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create a backup
	settings := map[string]interface{}{"test": true}
	settingsJSON, _ := json.Marshal(settings)
	backupData := map[string]interface{}{
		"settings": string(settingsJSON),
	}
	err := cr.CreateBackup(BackupTypeAppSetting, backupData, "manual_test")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Get backup ID
	var backupID int
	err = db.QueryRow("SELECT id FROM proxy_live_backup WHERE backup_type = ? ORDER BY backup_time DESC LIMIT 1",
		string(BackupTypeAppSetting)).Scan(&backupID)
	if err != nil {
		t.Fatalf("Failed to get backup ID: %v", err)
	}

	// Restore from specific backup
	err = cr.RestoreFromBackup(backupID)
	if err != nil {
		t.Fatalf("Failed to restore from backup: %v", err)
	}

	// Verify backup was marked as restored
	var restored int
	err = db.QueryRow("SELECT restored FROM proxy_live_backup WHERE id = ?", backupID).Scan(&restored)
	if err != nil {
		t.Fatalf("Failed to query backup: %v", err)
	}
	if restored != 1 {
		t.Errorf("Expected restored=1, got %d", restored)
	}

	// Verify file was created
	appSettingsPath := filepath.Join(tmpDir, "app.json")
	if _, err := os.Stat(appSettingsPath); os.IsNotExist(err) {
		t.Errorf("App settings file was not created")
	}
}

// TestConfigRecovery_GetBackupHistory tests backup history retrieval
func TestConfigRecovery_GetBackupHistory(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create multiple backups
	for i := 0; i < 5; i++ {
		data := map[string]interface{}{"index": i}
		err := cr.CreateBackup(BackupTypeProvider, data, "test")
		if err != nil {
			t.Fatalf("Failed to create backup: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Get history with limit
	history, err := cr.GetBackupHistory(BackupTypeProvider, 3)
	if err != nil {
		t.Fatalf("Failed to get backup history: %v", err)
	}

	// Verify limit works
	if len(history) != 3 {
		t.Errorf("Expected 3 backups in history, got %d", len(history))
	}

	// Verify order (newest first)
	for i := 0; i < len(history)-1; i++ {
		if history[i].BackupTime.Before(history[i+1].BackupTime) {
			t.Errorf("Backup history is not sorted by time (newest first)")
		}
	}
}

// TestConfigRecovery_CleanupOldBackups tests backup cleanup
func TestConfigRecovery_CleanupOldBackups(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create 10 backups
	for i := 0; i < 10; i++ {
		data := map[string]interface{}{"index": i}
		err := cr.CreateBackup(BackupTypeProvider, data, "test")
		if err != nil {
			t.Fatalf("Failed to create backup: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Keep only 3
	err := cr.CleanupOldBackups(3)
	if err != nil {
		t.Fatalf("Failed to cleanup old backups: %v", err)
	}

	// Verify only 3 backups remain
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM proxy_live_backup WHERE backup_type = ?", string(BackupTypeProvider)).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query backups: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 backups after cleanup, got %d", count)
	}
}

// TestConfigRecovery_NilDatabase tests behavior with nil database
func TestConfigRecovery_NilDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	cr := NewConfigRecovery(nil, tmpDir)

	// Test operations with nil db
	_, err := cr.GetLatestBackups()
	if err == nil {
		t.Errorf("Expected error with nil database, got nil")
	}

	err = cr.CreateBackup(BackupTypeProvider, map[string]interface{}{}, "test")
	if err == nil {
		t.Errorf("Expected error with nil database, got nil")
	}
}

// TestConfigRecovery_RecreateDeletedProvider tests provider recreation
func TestConfigRecovery_RecreateDeletedProvider(t *testing.T) {
	db, tmpDir := setupTestDB(t)
	defer db.Close()

	cr := NewConfigRecovery(db, tmpDir)

	// Create backup data for a provider that doesn't exist
	backupData := map[string]interface{}{
		"provider_id": 999,
		"snapshot": map[string]interface{}{
			"name":    "Deleted Provider",
			"api_url": "https://api.deleted.com",
			"api_key": "deleted-key",
			"enabled": 1,
		},
	}
	backupJSON, _ := json.Marshal(backupData)

	// Restore should recreate the provider
	err := cr.restoreProviderConfig(string(backupJSON))
	if err != nil {
		t.Fatalf("Failed to restore deleted provider: %v", err)
	}

	// Verify provider was recreated
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM provider_config WHERE name = ?", "Deleted Provider").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query provider: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected provider to be recreated, got count=%d", count)
	}
}
