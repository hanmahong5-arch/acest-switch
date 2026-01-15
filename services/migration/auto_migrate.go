package migration

import (
	"fmt"
	"os"
	"path/filepath"
)

// AutoMigrate automatically detects and runs migration if needed
// This is called on application startup
func AutoMigrate() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".code-switch")
	dbPath := filepath.Join(configDir, "app.db")
	schemaFile := "deploy/sqlite/schema_v2.sql"

	// Create migration config
	cfg := MigrationConfig{
		DryRun:         false,
		SchemaFile:     schemaFile,
		DBPath:         dbPath,
		ConfigDir:      configDir,
		VerboseLogging: true,
	}

	// Create and execute migration
	m, err := NewSSOTMigration(cfg)
	if err != nil {
		return fmt.Errorf("failed to create migration: %w", err)
	}

	return m.Execute()
}

// CheckMigrationStatus checks if migration has been completed
func CheckMigrationStatus() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbPath := filepath.Join(home, ".code-switch", "app.db")

	cfg := MigrationConfig{
		DBPath: dbPath,
	}

	m, err := NewSSOTMigration(cfg)
	if err != nil {
		return false, err
	}

	needed, err := m.NeedsMigration()
	if err != nil {
		return false, err
	}

	// Return true if migration is NOT needed (i.e., already migrated)
	return !needed, nil
}

// RollbackMigration provides a safe way to rollback the migration
func RollbackMigration() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".code-switch")

	// Find latest backup directory
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return fmt.Errorf("failed to read config directory: %w", err)
	}

	var latestBackup string
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 7 && entry.Name()[:7] == "backup_" {
			if latestBackup == "" || entry.Name() > latestBackup {
				latestBackup = entry.Name()
			}
		}
	}

	if latestBackup == "" {
		return fmt.Errorf("no backup directory found")
	}

	backupDir := filepath.Join(configDir, latestBackup)

	cfg := MigrationConfig{
		ConfigDir: configDir,
		BackupDir: backupDir,
	}

	m, err := NewSSOTMigration(cfg)
	if err != nil {
		return err
	}

	return m.Rollback()
}
