package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ProxyControlMode represents the proxy mode
type ProxyControlMode string

const (
	ProxyModeShared    ProxyControlMode = "shared"    // Shared port (18100)
	ProxyModeDedicated ProxyControlMode = "dedicated" // Dedicated port per app
)

// ProxyControlConfig holds proxy control configuration for an app
type ProxyControlConfig struct {
	AppName          string           `json:"app_name"`
	ProxyEnabled     bool             `json:"proxy_enabled"`
	ProxyMode        ProxyControlMode `json:"proxy_mode"`
	ProxyPort        int              `json:"proxy_port,omitempty"`
	InterceptDomains []string         `json:"intercept_domains,omitempty"`
	TotalRequests    int64            `json:"total_requests"`
	LastRequestAt    *time.Time       `json:"last_request_at,omitempty"`
	LastToggledAt    *time.Time       `json:"last_toggled_at,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
}

// ProxyController manages per-application proxy control
type ProxyController struct {
	db    *sql.DB
	cache map[string]bool // appName -> enabled cache
	mu    sync.RWMutex
}

// NewProxyController creates a new proxy controller
func NewProxyController(db *sql.DB) (*ProxyController, error) {
	pc := &ProxyController{
		db:    db,
		cache: make(map[string]bool),
	}

	// Initialize cache
	if err := pc.loadCache(); err != nil {
		return nil, fmt.Errorf("failed to load proxy control cache: %w", err)
	}

	return pc, nil
}

// loadCache loads proxy control settings into cache
func (pc *ProxyController) loadCache() error {
	if pc.db == nil {
		return nil
	}

	rows, err := pc.db.Query("SELECT app_name, proxy_enabled FROM proxy_control")
	if err != nil {
		return err
	}
	defer rows.Close()

	pc.mu.Lock()
	defer pc.mu.Unlock()

	for rows.Next() {
		var appName string
		var enabled int
		if err := rows.Scan(&appName, &enabled); err != nil {
			return err
		}
		pc.cache[appName] = (enabled == 1)
	}

	return rows.Err()
}

// IsProxyEnabled checks if proxy is enabled for an app
func (pc *ProxyController) IsProxyEnabled(appName string) bool {
	// Normalize app name
	appName = normalizeAppName(appName)

	pc.mu.RLock()
	enabled, exists := pc.cache[appName]
	pc.mu.RUnlock()

	if !exists {
		// Default to enabled if not found
		return true
	}

	return enabled
}

// ToggleProxy toggles proxy enable/disable for an app
func (pc *ProxyController) ToggleProxy(appName string, enabled bool) error {
	appName = normalizeAppName(appName)

	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Update database
	_, err := pc.db.Exec(`
		UPDATE proxy_control
		SET proxy_enabled = ?,
		    last_toggled_at = CURRENT_TIMESTAMP
		WHERE app_name = ?
	`, boolToInt(enabled), appName)

	if err != nil {
		return fmt.Errorf("failed to update proxy control: %w", err)
	}

	// Update cache
	pc.cache[appName] = enabled

	fmt.Printf("[ProxyControl] %s proxy %s\n", appName, enabledStatusString(enabled))

	return nil
}

// GetConfig returns the full proxy control configuration for an app
func (pc *ProxyController) GetConfig(appName string) (*ProxyControlConfig, error) {
	appName = normalizeAppName(appName)

	if pc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var config ProxyControlConfig
	var enabled int
	var proxyMode string
	var proxyPort sql.NullInt64
	var interceptDomains sql.NullString
	var lastRequestAt, lastToggledAt sql.NullTime

	err := pc.db.QueryRow(`
		SELECT app_name, proxy_enabled, proxy_mode, proxy_port,
		       intercept_domains, total_requests, last_request_at,
		       last_toggled_at, created_at
		FROM proxy_control
		WHERE app_name = ?
	`, appName).Scan(
		&config.AppName, &enabled, &proxyMode, &proxyPort,
		&interceptDomains, &config.TotalRequests, &lastRequestAt,
		&lastToggledAt, &config.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("proxy control not found for app: %s", appName)
		}
		return nil, fmt.Errorf("failed to query proxy control: %w", err)
	}

	config.ProxyEnabled = (enabled == 1)
	config.ProxyMode = ProxyControlMode(proxyMode)

	if proxyPort.Valid {
		config.ProxyPort = int(proxyPort.Int64)
	}

	if interceptDomains.Valid && interceptDomains.String != "" {
		if err := json.Unmarshal([]byte(interceptDomains.String), &config.InterceptDomains); err != nil {
			return nil, fmt.Errorf("failed to parse intercept domains: %w", err)
		}
	}

	if lastRequestAt.Valid {
		config.LastRequestAt = &lastRequestAt.Time
	}

	if lastToggledAt.Valid {
		config.LastToggledAt = &lastToggledAt.Time
	}

	return &config, nil
}

// GetAllConfigs returns proxy control configurations for all apps
func (pc *ProxyController) GetAllConfigs() ([]ProxyControlConfig, error) {
	if pc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := pc.db.Query(`
		SELECT app_name, proxy_enabled, proxy_mode, proxy_port,
		       intercept_domains, total_requests, last_request_at,
		       last_toggled_at, created_at
		FROM proxy_control
		ORDER BY app_name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query proxy controls: %w", err)
	}
	defer rows.Close()

	configs := make([]ProxyControlConfig, 0)

	for rows.Next() {
		var config ProxyControlConfig
		var enabled int
		var proxyMode string
		var proxyPort sql.NullInt64
		var interceptDomains sql.NullString
		var lastRequestAt, lastToggledAt sql.NullTime

		err := rows.Scan(
			&config.AppName, &enabled, &proxyMode, &proxyPort,
			&interceptDomains, &config.TotalRequests, &lastRequestAt,
			&lastToggledAt, &config.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan proxy control: %w", err)
		}

		config.ProxyEnabled = (enabled == 1)
		config.ProxyMode = ProxyControlMode(proxyMode)

		if proxyPort.Valid {
			config.ProxyPort = int(proxyPort.Int64)
		}

		if interceptDomains.Valid && interceptDomains.String != "" {
			json.Unmarshal([]byte(interceptDomains.String), &config.InterceptDomains)
		}

		if lastRequestAt.Valid {
			config.LastRequestAt = &lastRequestAt.Time
		}

		if lastToggledAt.Valid {
			config.LastToggledAt = &lastToggledAt.Time
		}

		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// RecordRequest records a proxy request for an app
func (pc *ProxyController) RecordRequest(appName string) error {
	appName = normalizeAppName(appName)

	if pc.db == nil {
		return nil // Silently fail if DB not initialized
	}

	_, err := pc.db.Exec(`
		UPDATE proxy_control
		SET total_requests = total_requests + 1,
		    last_request_at = CURRENT_TIMESTAMP
		WHERE app_name = ?
	`, appName)

	return err
}

// UpdateConfig updates proxy control configuration
func (pc *ProxyController) UpdateConfig(config ProxyControlConfig) error {
	appName := normalizeAppName(config.AppName)

	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Serialize intercept domains
	var interceptDomainsJSON string
	if len(config.InterceptDomains) > 0 {
		data, err := json.Marshal(config.InterceptDomains)
		if err != nil {
			return fmt.Errorf("failed to serialize intercept domains: %w", err)
		}
		interceptDomainsJSON = string(data)
	}

	// Update database
	_, err := pc.db.Exec(`
		UPDATE proxy_control
		SET proxy_enabled = ?,
		    proxy_mode = ?,
		    proxy_port = ?,
		    intercept_domains = ?,
		    last_toggled_at = CURRENT_TIMESTAMP
		WHERE app_name = ?
	`, boolToInt(config.ProxyEnabled), string(config.ProxyMode),
		nullInt(config.ProxyPort), nullString(interceptDomainsJSON), appName)

	if err != nil {
		return fmt.Errorf("failed to update proxy control: %w", err)
	}

	// Update cache
	pc.cache[appName] = config.ProxyEnabled

	return nil
}

// RefreshCache refreshes the cache from database
func (pc *ProxyController) RefreshCache() error {
	return pc.loadCache()
}

// GetStats returns statistics for all apps
func (pc *ProxyController) GetStats() (map[string]ProxyControlStats, error) {
	configs, err := pc.GetAllConfigs()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]ProxyControlStats)
	for _, config := range configs {
		stats[config.AppName] = ProxyControlStats{
			AppName:       config.AppName,
			Enabled:       config.ProxyEnabled,
			TotalRequests: config.TotalRequests,
			LastRequestAt: config.LastRequestAt,
		}
	}

	return stats, nil
}

// ProxyControlStats holds statistics for an app
type ProxyControlStats struct {
	AppName       string     `json:"app_name"`
	Enabled       bool       `json:"enabled"`
	TotalRequests int64      `json:"total_requests"`
	LastRequestAt *time.Time `json:"last_request_at,omitempty"`
}

// Helper functions

// normalizeAppName normalizes app name to standard format
func normalizeAppName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "claude", "claude-code", "claude_code":
		return "claude"
	case "codex":
		return "codex"
	case "gemini", "gemini-cli", "gemini_cli":
		return "gemini"
	default:
		return name
	}
}

// enabledStatusString returns string representation of enabled status
func enabledStatusString(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// nullInt returns sql.NullInt64 from int
func nullInt(v int) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

// nullString returns sql.NullString from string
func nullString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: v, Valid: true}
}
