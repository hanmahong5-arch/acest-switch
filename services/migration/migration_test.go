package migration

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// Test provider data for testing
var testProviders = []struct {
	Name   string
	APIURL string
	APIKey string
}{
	{Name: "Provider 1", APIURL: "https://api.provider1.com", APIKey: "key1"},
	{Name: "Provider 2", APIURL: "https://api.provider2.com", APIKey: "key2"},
	{Name: "Provider 3", APIURL: "https://api.provider3.com", APIKey: "key3"},
}

// setupTestEnvironment creates a temporary test directory
func setupTestEnvironment(t *testing.T) (string, func()) {
	// Create temporary directory
	testDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		os.RemoveAll(testDir)
	}

	return testDir, cleanup
}

// createTestJSONConfig creates test JSON configuration files
func createTestJSONConfig(t *testing.T, configDir string, platform string) {
	filename := map[string]string{
		"claude": "claude-code.json",
		"codex":  "codex.json",
		"gemini": "gemini-cli.json",
	}[platform]

	if filename == "" {
		t.Fatalf("Unknown platform: %s", platform)
	}

	// Create provider envelope
	type providerEnvelope struct {
		Providers []map[string]interface{} `json:"providers"`
	}

	providers := make([]map[string]interface{}, len(testProviders))
	for i, p := range testProviders {
		providers[i] = map[string]interface{}{
			"id":      i + 1,
			"name":    p.Name,
			"apiUrl":  p.APIURL,
			"apiKey":  p.APIKey,
			"enabled": true,
			"level":   1,
		}
	}

	envelope := providerEnvelope{Providers: providers}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	path := filepath.Join(configDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write JSON file: %v", err)
	}
}

// Test: Migration on empty configuration
func TestMigration_EmptyConfig(t *testing.T) {
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	dbPath := filepath.Join(testDir, "app.db")
	schemaFile := "../../deploy/sqlite/schema_v2.sql"

	// Check if schema file exists
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		t.Skip("Schema file not found, skipping test")
	}

	cfg := MigrationConfig{
		DryRun:         false,
		SchemaFile:     schemaFile,
		DBPath:         dbPath,
		ConfigDir:      testDir,
		SkipBackup:     true, // Skip backup for empty config
		VerboseLogging: false,
	}

	m, err := NewSSOTMigration(cfg)
	if err != nil {
		t.Fatalf("Failed to create migration: %v", err)
	}

	// Execute migration
	if err := m.Execute(); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify database was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("Database file was not created")
	}

	// Verify schema version
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var version int
	err = db.QueryRow("SELECT version FROM schema_version WHERE version = 2").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query schema version: %v", err)
	}

	if version != 2 {
		t.Fatalf("Expected schema version 2, got %d", version)
	}

	t.Logf("✓ Migration succeeded on empty configuration")
}

// Test: Migration with existing JSON configuration
func TestMigration_WithExistingConfig(t *testing.T) {
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test JSON files
	createTestJSONConfig(t, testDir, "claude")
	createTestJSONConfig(t, testDir, "codex")

	dbPath := filepath.Join(testDir, "app.db")
	schemaFile := "../../deploy/sqlite/schema_v2.sql"

	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		t.Skip("Schema file not found, skipping test")
	}

	cfg := MigrationConfig{
		DryRun:         false,
		SchemaFile:     schemaFile,
		DBPath:         dbPath,
		ConfigDir:      testDir,
		VerboseLogging: false,
	}

	m, err := NewSSOTMigration(cfg)
	if err != nil {
		t.Fatalf("Failed to create migration: %v", err)
	}

	// Execute migration
	if err := m.Execute(); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify database has providers
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Count total providers
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM provider_config").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count providers: %v", err)
	}

	expectedCount := len(testProviders) * 2 // claude + codex
	if count != expectedCount {
		t.Fatalf("Expected %d providers, got %d", expectedCount, count)
	}

	// Verify platform-specific providers
	err = db.QueryRow("SELECT COUNT(*) FROM provider_config WHERE platform = 'claude'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count claude providers: %v", err)
	}
	if count != len(testProviders) {
		t.Fatalf("Expected %d claude providers, got %d", len(testProviders), count)
	}

	// Verify backup was created
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read test directory: %v", err)
	}

	backupFound := false
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 7 && entry.Name()[:7] == "backup_" {
			backupFound = true
			break
		}
	}

	if !backupFound {
		t.Fatalf("Backup directory was not created")
	}

	// Verify JSON files were renamed to .migrated
	if _, err := os.Stat(filepath.Join(testDir, "claude-code.json.migrated")); os.IsNotExist(err) {
		t.Fatalf("JSON file was not renamed to .migrated")
	}

	t.Logf("✓ Migration succeeded with existing configuration")
}

// Test: Migration is idempotent (running twice should not cause errors)
func TestMigration_Idempotent(t *testing.T) {
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	createTestJSONConfig(t, testDir, "claude")

	dbPath := filepath.Join(testDir, "app.db")
	schemaFile := "../../deploy/sqlite/schema_v2.sql"

	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		t.Skip("Schema file not found, skipping test")
	}

	cfg := MigrationConfig{
		DryRun:         false,
		SchemaFile:     schemaFile,
		DBPath:         dbPath,
		ConfigDir:      testDir,
		VerboseLogging: false,
	}

	// First migration
	m1, err := NewSSOTMigration(cfg)
	if err != nil {
		t.Fatalf("Failed to create migration: %v", err)
	}

	if err := m1.Execute(); err != nil {
		t.Fatalf("First migration failed: %v", err)
	}

	// Second migration (should detect already migrated)
	m2, err := NewSSOTMigration(cfg)
	if err != nil {
		t.Fatalf("Failed to create second migration: %v", err)
	}

	if err := m2.Execute(); err != nil {
		t.Fatalf("Second migration failed: %v", err)
	}

	// Verify providers were not duplicated
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM provider_config").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count providers: %v", err)
	}

	if count != len(testProviders) {
		t.Fatalf("Expected %d providers after second migration, got %d", len(testProviders), count)
	}

	t.Logf("✓ Migration is idempotent")
}

// Test: Rollback functionality
func TestMigration_Rollback(t *testing.T) {
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	createTestJSONConfig(t, testDir, "claude")

	dbPath := filepath.Join(testDir, "app.db")
	schemaFile := "../../deploy/sqlite/schema_v2.sql"

	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		t.Skip("Schema file not found, skipping test")
	}

	cfg := MigrationConfig{
		DryRun:         false,
		SchemaFile:     schemaFile,
		DBPath:         dbPath,
		ConfigDir:      testDir,
		VerboseLogging: false,
	}

	m, err := NewSSOTMigration(cfg)
	if err != nil {
		t.Fatalf("Failed to create migration: %v", err)
	}

	// Execute migration
	if err := m.Execute(); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify JSON file was renamed
	migratedPath := filepath.Join(testDir, "claude-code.json.migrated")
	if _, err := os.Stat(migratedPath); os.IsNotExist(err) {
		t.Fatalf("Migrated JSON file not found")
	}

	// Execute rollback
	if err := m.Rollback(); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Verify JSON file was restored
	originalPath := filepath.Join(testDir, "claude-code.json")
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		t.Fatalf("JSON file was not restored after rollback")
	}

	// Verify .migrated file was removed
	if _, err := os.Stat(migratedPath); !os.IsNotExist(err) {
		t.Fatalf("Migrated JSON file was not removed after rollback")
	}

	t.Logf("✓ Rollback succeeded")
}

// Test: Dry-run mode
func TestMigration_DryRun(t *testing.T) {
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	createTestJSONConfig(t, testDir, "claude")

	dbPath := filepath.Join(testDir, "app.db")
	schemaFile := "../../deploy/sqlite/schema_v2.sql"

	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		t.Skip("Schema file not found, skipping test")
	}

	cfg := MigrationConfig{
		DryRun:         true, // Dry-run mode
		SchemaFile:     schemaFile,
		DBPath:         dbPath,
		ConfigDir:      testDir,
		VerboseLogging: false,
	}

	m, err := NewSSOTMigration(cfg)
	if err != nil {
		t.Fatalf("Failed to create migration: %v", err)
	}

	// Execute dry-run migration
	if err := m.Execute(); err != nil {
		t.Fatalf("Dry-run migration failed: %v", err)
	}

	// Verify database was NOT created
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("Database file should not be created in dry-run mode")
	}

	// Verify JSON file was NOT renamed
	originalPath := filepath.Join(testDir, "claude-code.json")
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		t.Fatalf("JSON file should not be modified in dry-run mode")
	}

	t.Logf("✓ Dry-run mode works correctly")
}

// Benchmark: Migration performance with large dataset
func BenchmarkMigration_LargeDataset(b *testing.B) {
	testDir, err := os.MkdirTemp("", "migration-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create large test dataset (100 providers)
	type providerEnvelope struct {
		Providers []map[string]interface{} `json:"providers"`
	}

	providers := make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		providers[i] = map[string]interface{}{
			"id":      i + 1,
			"name":    "Provider " + string(rune(i)),
			"apiUrl":  "https://api.example.com",
			"apiKey":  "key" + string(rune(i)),
			"enabled": true,
			"level":   1,
		}
	}

	envelope := providerEnvelope{Providers: providers}
	data, _ := json.Marshal(envelope)
	os.WriteFile(filepath.Join(testDir, "claude-code.json"), data, 0644)

	dbPath := filepath.Join(testDir, "app.db")
	schemaFile := "../../deploy/sqlite/schema_v2.sql"

	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		b.Skip("Schema file not found, skipping benchmark")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Remove database between runs
		os.Remove(dbPath)

		cfg := MigrationConfig{
			DryRun:         false,
			SchemaFile:     schemaFile,
			DBPath:         dbPath,
			ConfigDir:      testDir,
			SkipBackup:     true,
			VerboseLogging: false,
		}

		m, _ := NewSSOTMigration(cfg)
		if err := m.Execute(); err != nil {
			b.Fatalf("Migration failed: %v", err)
		}
	}
}

// Test: NeedsMigration detection
func TestMigration_NeedsMigration(t *testing.T) {
	testDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	dbPath := filepath.Join(testDir, "app.db")

	cfg := MigrationConfig{
		DBPath: dbPath,
	}

	m, err := NewSSOTMigration(cfg)
	if err != nil {
		t.Fatalf("Failed to create migration: %v", err)
	}

	// Test 1: No database exists - should need migration
	needed, err := m.NeedsMigration()
	if err != nil {
		t.Fatalf("NeedsMigration failed: %v", err)
	}
	if !needed {
		t.Fatalf("Expected migration needed when no database exists")
	}

	// Create database with old schema (version 1)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE schema_version (
			version INTEGER PRIMARY KEY,
			description TEXT,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO schema_version (version, description) VALUES (1, 'Initial schema');
	`)
	if err != nil {
		t.Fatalf("Failed to create old schema: %v", err)
	}
	db.Close()

	// Test 2: Old schema (version 1) - should need migration
	m2, _ := NewSSOTMigration(cfg)
	needed, err = m2.NeedsMigration()
	if err != nil {
		t.Fatalf("NeedsMigration failed: %v", err)
	}
	if !needed {
		t.Fatalf("Expected migration needed with version 1 schema")
	}

	// Update to version 2
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	_, err = db.Exec("INSERT INTO schema_version (version, description) VALUES (2, 'SSOT schema')")
	db.Close()

	// Test 3: Current schema (version 2) - should NOT need migration
	m3, _ := NewSSOTMigration(cfg)
	needed, err = m3.NeedsMigration()
	if err != nil {
		t.Fatalf("NeedsMigration failed: %v", err)
	}
	if needed {
		t.Fatalf("Expected migration NOT needed with version 2 schema")
	}

	t.Logf("✓ NeedsMigration detection works correctly")
}
