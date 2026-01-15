package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tidwall/gjson"
)

// ConfigRecovery provides configuration backup and recovery functionality
type ConfigRecovery struct {
	db          *sql.DB
	configDir   string
	crashMarker string
}

// BackupType represents the type of configuration backup
type BackupType string

const (
	BackupTypeProvider   BackupType = "provider_config"
	BackupTypeAppSetting BackupType = "app_settings"
	BackupTypeMCP        BackupType = "mcp_config"
)

// BackupRecord represents a single backup record
type BackupRecord struct {
	ID           int        `json:"id"`
	BackupType   string     `json:"backup_type"`
	BackupData   string     `json:"backup_data"`
	TriggerEvent string     `json:"trigger_event"`
	BackupTime   time.Time  `json:"backup_time"`
	Restored     bool       `json:"restored"`
	RestoredAt   *time.Time `json:"restored_at,omitempty"`
}

// NewConfigRecovery creates a new configuration recovery service
func NewConfigRecovery(db *sql.DB, configDir string) *ConfigRecovery {
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".code-switch")
	}

	return &ConfigRecovery{
		db:          db,
		configDir:   configDir,
		crashMarker: filepath.Join(configDir, ".crash_marker"),
	}
}

// DetectAbnormalShutdown checks if the application crashed on the last run
func (cr *ConfigRecovery) DetectAbnormalShutdown() (bool, error) {
	// Check if crash marker file exists
	if _, err := os.Stat(cr.crashMarker); os.IsNotExist(err) {
		// No marker file, normal startup - create one
		if err := cr.CreateCrashMarker(); err != nil {
			return false, fmt.Errorf("failed to create crash marker: %w", err)
		}
		return false, nil
	}

	// Marker file exists, indicating abnormal shutdown
	return true, nil
}

// CreateCrashMarker creates a marker file to detect abnormal shutdowns
func (cr *ConfigRecovery) CreateCrashMarker() error {
	// Ensure config directory exists
	if err := os.MkdirAll(cr.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create marker file with timestamp
	timestamp := time.Now().Format(time.RFC3339)
	return os.WriteFile(cr.crashMarker, []byte(timestamp), 0644)
}

// RemoveCrashMarker removes the crash marker file on normal shutdown
func (cr *ConfigRecovery) RemoveCrashMarker() error {
	if err := os.Remove(cr.crashMarker); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove crash marker: %w", err)
	}
	return nil
}

// RecoverFromCrash attempts to recover configuration from the last backup
func (cr *ConfigRecovery) RecoverFromCrash() error {
	fmt.Println("[Recovery] Detecting abnormal shutdown, attempting recovery...")

	// Get latest backups for each type
	backups, err := cr.GetLatestBackups()
	if err != nil {
		return fmt.Errorf("failed to get latest backups: %w", err)
	}

	if len(backups) == 0 {
		fmt.Println("[Recovery] No backups found, skipping recovery")
		return nil
	}

	// Recover each backup type
	for _, backup := range backups {
		if err := cr.restoreBackup(backup); err != nil {
			fmt.Printf("[Recovery] Failed to restore %s backup (ID=%d): %v\n",
				backup.BackupType, backup.ID, err)
			continue
		}

		// Mark backup as restored
		if err := cr.markBackupAsRestored(backup.ID); err != nil {
			fmt.Printf("[Recovery] Failed to mark backup %d as restored: %v\n", backup.ID, err)
		}

		fmt.Printf("[Recovery] ✓ Restored %s from backup (ID=%d)\n", backup.BackupType, backup.ID)
	}

	return nil
}

// GetLatestBackups returns the most recent backup for each backup type
func (cr *ConfigRecovery) GetLatestBackups() ([]BackupRecord, error) {
	if cr.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT
			b.id,
			b.backup_type,
			b.backup_data,
			b.trigger_event,
			b.backup_time,
			b.restored,
			b.restored_at
		FROM proxy_live_backup b
		INNER JOIN (
			SELECT backup_type, MAX(backup_time) as max_time
			FROM proxy_live_backup
			WHERE restored = 0
			GROUP BY backup_type
		) latest ON b.backup_type = latest.backup_type AND b.backup_time = latest.max_time
		ORDER BY b.backup_time DESC
	`

	rows, err := cr.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query backups: %w", err)
	}
	defer rows.Close()

	var backups []BackupRecord
	for rows.Next() {
		var b BackupRecord
		var restored int
		var restoredAt sql.NullTime

		if err := rows.Scan(&b.ID, &b.BackupType, &b.BackupData, &b.TriggerEvent,
			&b.BackupTime, &restored, &restoredAt); err != nil {
			return nil, fmt.Errorf("failed to scan backup record: %w", err)
		}

		b.Restored = (restored == 1)
		if restoredAt.Valid {
			b.RestoredAt = &restoredAt.Time
		}

		backups = append(backups, b)
	}

	return backups, rows.Err()
}

// restoreBackup restores a single backup record
func (cr *ConfigRecovery) restoreBackup(backup BackupRecord) error {
	switch BackupType(backup.BackupType) {
	case BackupTypeProvider:
		return cr.restoreProviderConfig(backup.BackupData)
	case BackupTypeAppSetting:
		return cr.restoreAppSettings(backup.BackupData)
	case BackupTypeMCP:
		return cr.restoreMCPConfig(backup.BackupData)
	default:
		return fmt.Errorf("unknown backup type: %s", backup.BackupType)
	}
}

// restoreProviderConfig restores provider configuration from backup
func (cr *ConfigRecovery) restoreProviderConfig(backupData string) error {
	// Parse backup data
	providerID := gjson.Get(backupData, "provider_id").Int()
	snapshot := gjson.Get(backupData, "snapshot")

	if !snapshot.Exists() {
		return fmt.Errorf("snapshot not found in backup data")
	}

	// Check if provider still exists
	var exists int
	err := cr.db.QueryRow("SELECT COUNT(*) FROM provider_config WHERE id = ?", providerID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check provider existence: %w", err)
	}

	if exists == 0 {
		// Provider was deleted, recreate it
		return cr.recreateProvider(snapshot)
	}

	// Provider exists, update it
	return cr.updateProvider(providerID, snapshot)
}

// recreateProvider recreates a deleted provider from backup
func (cr *ConfigRecovery) recreateProvider(snapshot gjson.Result) error {
	name := snapshot.Get("name").String()
	apiURL := snapshot.Get("api_url").String()
	apiKey := snapshot.Get("api_key").String()
	enabled := snapshot.Get("enabled").Int()

	_, err := cr.db.Exec(`
		INSERT INTO provider_config (name, api_url, api_key, enabled, platform)
		SELECT ?, ?, ?, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM provider_config WHERE name = ?)
	`, name, apiURL, apiKey, enabled, "unknown", name)

	return err
}

// updateProvider updates an existing provider from backup
func (cr *ConfigRecovery) updateProvider(providerID int64, snapshot gjson.Result) error {
	enabled := snapshot.Get("enabled").Int()

	_, err := cr.db.Exec(`
		UPDATE provider_config
		SET enabled = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, enabled, providerID)

	return err
}

// restoreAppSettings restores application settings from backup
func (cr *ConfigRecovery) restoreAppSettings(backupData string) error {
	// Parse backup data
	settingsJSON := gjson.Get(backupData, "settings").String()
	if settingsJSON == "" {
		return fmt.Errorf("settings not found in backup data")
	}

	// Write to app.json
	appSettingsPath := filepath.Join(cr.configDir, "app.json")
	if err := os.WriteFile(appSettingsPath, []byte(settingsJSON), 0644); err != nil {
		return fmt.Errorf("failed to write app settings: %w", err)
	}

	return nil
}

// restoreMCPConfig restores MCP configuration from backup
func (cr *ConfigRecovery) restoreMCPConfig(backupData string) error {
	// Parse backup data
	mcpJSON := gjson.Get(backupData, "mcp_servers").String()
	if mcpJSON == "" {
		return fmt.Errorf("mcp_servers not found in backup data")
	}

	// Write to mcp.json
	mcpPath := filepath.Join(cr.configDir, "mcp.json")
	if err := os.WriteFile(mcpPath, []byte(mcpJSON), 0644); err != nil {
		return fmt.Errorf("failed to write MCP config: %w", err)
	}

	return nil
}

// markBackupAsRestored marks a backup as restored
func (cr *ConfigRecovery) markBackupAsRestored(backupID int) error {
	_, err := cr.db.Exec(`
		UPDATE proxy_live_backup
		SET restored = 1,
		    restored_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, backupID)

	return err
}

// CreateBackup manually creates a backup
func (cr *ConfigRecovery) CreateBackup(backupType BackupType, data interface{}, triggerEvent string) error {
	if cr.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Serialize data to JSON
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize backup data: %w", err)
	}

	// Insert backup record
	_, err = cr.db.Exec(`
		INSERT INTO proxy_live_backup (backup_type, backup_data, trigger_event)
		VALUES (?, ?, ?)
	`, string(backupType), string(dataJSON), triggerEvent)

	return err
}

// GetBackupHistory returns backup history for a specific type
func (cr *ConfigRecovery) GetBackupHistory(backupType BackupType, limit int) ([]BackupRecord, error) {
	if cr.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, backup_type, backup_data, trigger_event, backup_time, restored, restored_at
		FROM proxy_live_backup
		WHERE backup_type = ?
		ORDER BY backup_time DESC
		LIMIT ?
	`

	rows, err := cr.db.Query(query, string(backupType), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query backup history: %w", err)
	}
	defer rows.Close()

	var backups []BackupRecord
	for rows.Next() {
		var b BackupRecord
		var restored int
		var restoredAt sql.NullTime

		if err := rows.Scan(&b.ID, &b.BackupType, &b.BackupData, &b.TriggerEvent,
			&b.BackupTime, &restored, &restoredAt); err != nil {
			return nil, fmt.Errorf("failed to scan backup record: %w", err)
		}

		b.Restored = (restored == 1)
		if restoredAt.Valid {
			b.RestoredAt = &restoredAt.Time
		}

		backups = append(backups, b)
	}

	return backups, rows.Err()
}

// RestoreFromBackup restores configuration from a specific backup ID
func (cr *ConfigRecovery) RestoreFromBackup(backupID int) error {
	if cr.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Get backup record
	var backup BackupRecord
	var restored int
	var restoredAt sql.NullTime

	err := cr.db.QueryRow(`
		SELECT id, backup_type, backup_data, trigger_event, backup_time, restored, restored_at
		FROM proxy_live_backup
		WHERE id = ?
	`, backupID).Scan(&backup.ID, &backup.BackupType, &backup.BackupData,
		&backup.TriggerEvent, &backup.BackupTime, &restored, &restoredAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("backup with ID %d not found", backupID)
		}
		return fmt.Errorf("failed to query backup: %w", err)
	}

	backup.Restored = (restored == 1)
	if restoredAt.Valid {
		backup.RestoredAt = &restoredAt.Time
	}

	// Restore backup
	if err := cr.restoreBackup(backup); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	// Mark as restored
	if err := cr.markBackupAsRestored(backupID); err != nil {
		return fmt.Errorf("failed to mark backup as restored: %w", err)
	}

	fmt.Printf("[Recovery] ✓ Manually restored %s from backup (ID=%d)\n", backup.BackupType, backupID)

	return nil
}

// CleanupOldBackups removes old backup records (keep last N per type)
func (cr *ConfigRecovery) CleanupOldBackups(keepCount int) error {
	if cr.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Get distinct backup types
	rows, err := cr.db.Query("SELECT DISTINCT backup_type FROM proxy_live_backup")
	if err != nil {
		return fmt.Errorf("failed to query backup types: %w", err)
	}
	defer rows.Close()

	var backupTypes []string
	for rows.Next() {
		var bt string
		if err := rows.Scan(&bt); err != nil {
			return fmt.Errorf("failed to scan backup type: %w", err)
		}
		backupTypes = append(backupTypes, bt)
	}

	// Cleanup each backup type
	for _, bt := range backupTypes {
		_, err := cr.db.Exec(`
			DELETE FROM proxy_live_backup
			WHERE id IN (
				SELECT id FROM proxy_live_backup
				WHERE backup_type = ?
				ORDER BY backup_time DESC
				LIMIT -1 OFFSET ?
			)
		`, bt, keepCount)

		if err != nil {
			return fmt.Errorf("failed to cleanup backups for type %s: %w", bt, err)
		}
	}

	return nil
}
