package migration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// MigrationConfig holds migration configuration
type MigrationConfig struct {
	DryRun         bool   // Dry-run mode, no actual changes
	BackupDir      string // Backup directory for JSON files
	SchemaFile     string // Path to schema_v2.sql
	DBPath         string // Path to SQLite database
	ConfigDir      string // Path to config directory (~/.code-switch)
	SkipBackup     bool   // Skip backup step (for testing)
	VerboseLogging bool   // Enable detailed logging
}

// SSOTMigration manages the migration process
type SSOTMigration struct {
	db     *sql.DB
	cfg    MigrationConfig
	logger func(format string, args ...interface{})
}

// Provider represents a provider configuration
type Provider struct {
	ID              int               `json:"id,omitempty"`
	Name            string            `json:"name"`
	APIURL          string            `json:"apiUrl"`
	APIKey          string            `json:"apiKey"`
	Site            string            `json:"officialSite,omitempty"`
	Icon            string            `json:"icon,omitempty"`
	Tint            string            `json:"tint,omitempty"`
	Accent          string            `json:"accent,omitempty"`
	Enabled         bool              `json:"enabled"`
	SupportedModels map[string]bool   `json:"supportedModels,omitempty"`
	ModelMapping    map[string]string `json:"modelMapping,omitempty"`
	Level           int               `json:"level"` // Priority level
}

// ProviderEnvelope wraps the provider list in JSON files
type ProviderEnvelope struct {
	Providers []Provider `json:"providers"`
}

// NewSSOTMigration creates a new migration instance
func NewSSOTMigration(cfg MigrationConfig) (*SSOTMigration, error) {
	// Set defaults
	if cfg.ConfigDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cfg.ConfigDir = filepath.Join(home, ".code-switch")
	}

	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(cfg.ConfigDir, "app.db")
	}

	if cfg.SchemaFile == "" {
		// Assume schema file is in deploy/sqlite relative to current directory
		cfg.SchemaFile = "deploy/sqlite/schema_v2.sql"
	}

	if cfg.BackupDir == "" {
		timestamp := time.Now().Format("20060102")
		cfg.BackupDir = filepath.Join(cfg.ConfigDir, fmt.Sprintf("backup_%s", timestamp))
	}

	m := &SSOTMigration{
		cfg: cfg,
		logger: func(format string, args ...interface{}) {
			if cfg.VerboseLogging {
				fmt.Printf("[Migration] "+format+"\n", args...)
			}
		},
	}

	return m, nil
}

// Execute runs the full migration process
func (m *SSOTMigration) Execute() error {
	m.logger("Starting SSOT migration")

	// Step 1: Check if migration is needed
	needed, err := m.NeedsMigration()
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if !needed {
		m.logger("Migration not needed, schema version is up to date")
		return nil
	}

	m.logger("Migration needed, proceeding...")

	// Step 2: Backup existing configuration
	if !m.cfg.SkipBackup {
		if err := m.BackupExistingConfig(); err != nil {
			return fmt.Errorf("failed to backup configuration: %w", err)
		}
		m.logger("Configuration backed up to: %s", m.cfg.BackupDir)
	}

	// Step 3: Execute schema upgrade
	if err := m.UpgradeSchema(); err != nil {
		return fmt.Errorf("failed to upgrade schema: %w", err)
	}
	m.logger("Schema upgraded successfully")

	// Step 4: Migrate data (JSON → SQLite)
	if err := m.MigrateData(); err != nil {
		return fmt.Errorf("failed to migrate data: %w", err)
	}
	m.logger("Data migrated successfully")

	// Step 5: Cleanup and mark complete
	if err := m.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup: %w", err)
	}
	m.logger("Migration completed successfully")

	fmt.Println("✓ SSOT Migration completed successfully")
	return nil
}

// NeedsMigration checks if migration is needed
func (m *SSOTMigration) NeedsMigration() (bool, error) {
	// Open database connection
	db, err := sql.Open("sqlite", m.cfg.DBPath)
	if err != nil {
		return false, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	m.db = db

	// Check if schema_version table exists
	var tableName string
	err = db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='schema_version'
	`).Scan(&tableName)

	if err == sql.ErrNoRows {
		// Table doesn't exist, migration needed
		return true, nil
	}

	if err != nil {
		return false, fmt.Errorf("failed to check schema_version table: %w", err)
	}

	// Check current schema version
	var version int
	err = db.QueryRow(`
		SELECT version FROM schema_version
		ORDER BY version DESC LIMIT 1
	`).Scan(&version)

	if err == sql.ErrNoRows {
		// No version records, migration needed
		return true, nil
	}

	if err != nil {
		return false, fmt.Errorf("failed to query schema version: %w", err)
	}

	// Version 2 is the SSOT architecture
	return version < 2, nil
}

// BackupExistingConfig backs up JSON configuration files
func (m *SSOTMigration) BackupExistingConfig() error {
	if m.cfg.DryRun {
		m.logger("[DRY-RUN] Would backup config to: %s", m.cfg.BackupDir)
		return nil
	}

	// Create backup directory
	if err := os.MkdirAll(m.cfg.BackupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// List of files to backup
	filesToBackup := []string{
		"claude-code.json",
		"codex.json",
		"gemini-cli.json",
		"mcp.json",
		"app.json",
		"sync-settings.json",
	}

	backedUpCount := 0
	for _, filename := range filesToBackup {
		srcPath := filepath.Join(m.cfg.ConfigDir, filename)
		dstPath := filepath.Join(m.cfg.BackupDir, filename)

		// Check if source file exists
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			m.logger("Skipping %s (does not exist)", filename)
			continue
		}

		// Copy file to backup
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filename, err)
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write backup %s: %w", filename, err)
		}

		backedUpCount++
		m.logger("Backed up: %s", filename)
	}

	// Also backup database if it exists
	dbBackupPath := filepath.Join(m.cfg.BackupDir, "app.db")
	if _, err := os.Stat(m.cfg.DBPath); err == nil {
		data, err := os.ReadFile(m.cfg.DBPath)
		if err == nil {
			os.WriteFile(dbBackupPath, data, 0644)
			backedUpCount++
			m.logger("Backed up: app.db")
		}
	}

	fmt.Printf("✓ Backed up %d file(s) to: %s\n", backedUpCount, m.cfg.BackupDir)
	return nil
}

// UpgradeSchema executes the schema_v2.sql file
func (m *SSOTMigration) UpgradeSchema() error {
	if m.cfg.DryRun {
		m.logger("[DRY-RUN] Would execute schema from: %s", m.cfg.SchemaFile)
		return nil
	}

	// Read schema file
	schemaSQL, err := os.ReadFile(m.cfg.SchemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Execute schema
	_, err = m.db.Exec(string(schemaSQL))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	m.logger("Schema executed successfully")
	return nil
}

// MigrateData migrates provider data from JSON files to SQLite
func (m *SSOTMigration) MigrateData() error {
	if m.cfg.DryRun {
		m.logger("[DRY-RUN] Would migrate data from JSON files")
		return nil
	}

	platforms := map[string]string{
		"claude": "claude-code.json",
		"codex":  "codex.json",
		"gemini": "gemini-cli.json",
	}

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	totalMigrated := 0

	for platform, filename := range platforms {
		path := filepath.Join(m.cfg.ConfigDir, filename)

		// Check if file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			m.logger("Skipping %s (does not exist)", filename)
			continue
		}

		// Read JSON file
		data, err := os.ReadFile(path)
		if err != nil {
			m.logger("Warning: failed to read %s: %v", filename, err)
			continue
		}

		// Parse JSON
		var envelope ProviderEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			m.logger("Warning: failed to parse %s: %v", filename, err)
			continue
		}

		// Insert providers into database
		for _, p := range envelope.Providers {
			supportedModelsJSON, _ := json.Marshal(p.SupportedModels)
			modelMappingJSON, _ := json.Marshal(p.ModelMapping)

			// Set defaults for new fields
			if p.Tint == "" {
				p.Tint = "#f0f0f0"
			}
			if p.Accent == "" {
				p.Accent = "#0a84ff"
			}

			enabled := 0
			if p.Enabled {
				enabled = 1
			}

			_, err := tx.Exec(`
				INSERT INTO provider_config
				(platform, name, api_url, api_key, official_site, icon,
				 tint, accent, enabled, supported_models, model_mapping, priority_level)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, platform, p.Name, p.APIURL, p.APIKey, p.Site, p.Icon,
				p.Tint, p.Accent, enabled,
				string(supportedModelsJSON), string(modelMappingJSON), p.Level)

			if err != nil {
				m.logger("Warning: failed to insert provider %s: %v", p.Name, err)
				continue
			}

			totalMigrated++
			m.logger("Migrated provider: %s (%s)", p.Name, platform)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("✓ Migrated %d provider(s) to database\n", totalMigrated)
	return nil
}

// Cleanup renames old JSON files and marks migration complete
func (m *SSOTMigration) Cleanup() error {
	if m.cfg.DryRun {
		m.logger("[DRY-RUN] Would rename JSON files to .migrated")
		return nil
	}

	// Rename JSON files to .migrated
	filesToRename := []string{
		"claude-code.json",
		"codex.json",
		"gemini-cli.json",
	}

	for _, filename := range filesToRename {
		oldPath := filepath.Join(m.cfg.ConfigDir, filename)
		newPath := filepath.Join(m.cfg.ConfigDir, filename+".migrated")

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			m.logger("Warning: failed to rename %s: %v", filename, err)
		} else {
			m.logger("Renamed: %s → %s", filename, filename+".migrated")
		}
	}

	return nil
}

// Rollback restores configuration from backup
func (m *SSOTMigration) Rollback() error {
	m.logger("Rolling back migration...")

	if _, err := os.Stat(m.cfg.BackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup directory not found: %s", m.cfg.BackupDir)
	}

	// List all files in backup directory
	entries, err := os.ReadDir(m.cfg.BackupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	restoredCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(m.cfg.BackupDir, entry.Name())
		dstPath := filepath.Join(m.cfg.ConfigDir, entry.Name())

		data, err := os.ReadFile(srcPath)
		if err != nil {
			m.logger("Warning: failed to read backup %s: %v", entry.Name(), err)
			continue
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			m.logger("Warning: failed to restore %s: %v", entry.Name(), err)
			continue
		}

		restoredCount++
		m.logger("Restored: %s", entry.Name())
	}

	// Remove .migrated files
	migratedFiles, _ := filepath.Glob(filepath.Join(m.cfg.ConfigDir, "*.migrated"))
	for _, f := range migratedFiles {
		os.Remove(f)
	}

	fmt.Printf("✓ Rolled back %d file(s) from backup\n", restoredCount)
	return nil
}

// boolToInt converts boolean to integer for SQLite
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
