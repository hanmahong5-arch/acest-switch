package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ProviderServiceV2 uses SQLite as SSOT for provider configuration
// Maintains backward compatibility with JSON-based ProviderService
type ProviderServiceV2 struct {
	db              *sql.DB
	mu              sync.RWMutex
	dbPath          string
	useDatabase     bool // Flag to enable/disable database mode
	fallbackService *ProviderService
}

// NewProviderServiceV2 creates a new database-backed provider service
func NewProviderServiceV2(dbPath string) (*ProviderServiceV2, error) {
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = filepath.Join(home, ".code-switch", "app.db")
	}

	// Check if database exists and has the v2 schema
	useDB := false
	if _, err := os.Stat(dbPath); err == nil {
		// Database file exists, check for schema version
		db, err := sql.Open("sqlite", dbPath)
		if err == nil {
			defer db.Close()
			var version int
			err = db.QueryRow("SELECT version FROM schema_version WHERE version = 2").Scan(&version)
			if err == nil && version == 2 {
				useDB = true
			}
		}
	}

	ps := &ProviderServiceV2{
		dbPath:          dbPath,
		useDatabase:     useDB,
		fallbackService: NewProviderService(),
	}

	// If database mode is enabled, open connection
	if useDB {
		db, err := ps.openDB()
		if err != nil {
			// Fall back to JSON mode on error
			ps.useDatabase = false
			return ps, nil
		}
		ps.db = db
	}

	return ps, nil
}

// openDB opens a connection to the SQLite database
func (ps *ProviderServiceV2) openDB() (*sql.DB, error) {
	// Connection string with optimizations
	dsn := ps.dbPath + "?cache=shared&mode=rwc&_busy_timeout=10000&_journal_mode=WAL&_sync=NORMAL"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// Start initializes the service
func (ps *ProviderServiceV2) Start() error {
	if ps.useDatabase && ps.db == nil {
		db, err := ps.openDB()
		if err != nil {
			return err
		}
		ps.db = db
	}
	return nil
}

// Stop closes database connections
func (ps *ProviderServiceV2) Stop() error {
	if ps.db != nil {
		return ps.db.Close()
	}
	return nil
}

// LoadProviders loads providers for a given platform
// Maintains backward compatibility - falls back to JSON if database unavailable
func (ps *ProviderServiceV2) LoadProviders(kind string) ([]Provider, error) {
	// Normalize platform name
	platform := normalizePlatformV2(kind)

	// Try database first if enabled
	if ps.useDatabase {
		providers, err := ps.loadFromDatabase(platform)
		if err == nil {
			return providers, nil
		}
		// Fall through to JSON fallback on error
	}

	// Fallback to JSON-based service
	return ps.fallbackService.LoadProviders(kind)
}

// loadFromDatabase loads providers from SQLite
func (ps *ProviderServiceV2) loadFromDatabase(platform string) ([]Provider, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	rows, err := ps.db.Query(`
		SELECT id, name, api_url, api_key, official_site, icon, tint, accent,
		       enabled, supported_models, model_mapping, priority_level
		FROM provider_config
		WHERE platform = ?
		ORDER BY priority_level ASC, id ASC
	`, platform)

	if err != nil {
		return nil, fmt.Errorf("failed to query providers: %w", err)
	}
	defer rows.Close()

	providers := []Provider{}
	for rows.Next() {
		var p Provider
		var supportedModelsJSON, modelMappingJSON sql.NullString
		var site, icon, tint, accent sql.NullString
		var enabled int

		err := rows.Scan(
			&p.ID, &p.Name, &p.APIURL, &p.APIKey,
			&site, &icon, &tint, &accent,
			&enabled, &supportedModelsJSON, &modelMappingJSON, &p.Level,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan provider row: %w", err)
		}

		// Handle nullable fields
		p.Site = site.String
		p.Icon = icon.String
		p.Tint = tint.String
		p.Accent = accent.String
		p.Enabled = (enabled == 1)

		// Parse JSON fields
		if supportedModelsJSON.Valid && supportedModelsJSON.String != "" {
			if err := json.Unmarshal([]byte(supportedModelsJSON.String), &p.SupportedModels); err != nil {
				return nil, fmt.Errorf("failed to parse supported_models for %s: %w", p.Name, err)
			}
		}

		if modelMappingJSON.Valid && modelMappingJSON.String != "" {
			if err := json.Unmarshal([]byte(modelMappingJSON.String), &p.ModelMapping); err != nil {
				return nil, fmt.Errorf("failed to parse model_mapping for %s: %w", p.Name, err)
			}
		}

		providers = append(providers, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating provider rows: %w", err)
	}

	return providers, nil
}

// SaveProviders saves providers for a given platform
func (ps *ProviderServiceV2) SaveProviders(kind string, providers []Provider) error {
	platform := normalizePlatformV2(kind)

	// Validate providers first
	existingProviders, err := ps.LoadProviders(kind)
	if err != nil {
		return err
	}

	nameByID := make(map[int]string, len(existingProviders))
	for _, p := range existingProviders {
		nameByID[p.ID] = p.Name
	}

	validationErrors := make([]string, 0)
	for _, p := range providers {
		// Rule 1: name cannot be modified
		if oldName, ok := nameByID[p.ID]; ok && oldName != p.Name {
			return fmt.Errorf("provider id %d 的 name 不可修改", p.ID)
		}

		// Rule 2: validate model configuration
		if errs := p.ValidateConfiguration(); len(errs) > 0 {
			for _, errMsg := range errs {
				validationErrors = append(validationErrors, fmt.Sprintf("[%s] %s", p.Name, errMsg))
			}
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("配置验证失败：\n  - %s", strings.Join(validationErrors, "\n  - "))
	}

	// Save to database if enabled
	if ps.useDatabase {
		if err := ps.saveToDatabase(platform, providers); err != nil {
			return err
		}
	}

	// Also save to JSON for backward compatibility during transition
	// This can be removed after full migration
	return ps.fallbackService.SaveProviders(kind, providers)
}

// saveToDatabase saves providers to SQLite
func (ps *ProviderServiceV2) saveToDatabase(platform string, providers []Provider) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Start transaction
	tx, err := ps.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing providers for this platform
	_, err = tx.Exec("DELETE FROM provider_config WHERE platform = ?", platform)
	if err != nil {
		return fmt.Errorf("failed to delete existing providers: %w", err)
	}

	// Insert new providers
	stmt, err := tx.Prepare(`
		INSERT INTO provider_config
		(platform, name, api_url, api_key, official_site, icon,
		 tint, accent, enabled, supported_models, model_mapping, priority_level)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for _, p := range providers {
		supportedModelsJSON, _ := json.Marshal(p.SupportedModels)
		modelMappingJSON, _ := json.Marshal(p.ModelMapping)

		enabled := 0
		if p.Enabled {
			enabled = 1
		}

		_, err := stmt.Exec(
			platform, p.Name, p.APIURL, p.APIKey, p.Site, p.Icon,
			p.Tint, p.Accent, enabled,
			string(supportedModelsJSON), string(modelMappingJSON), p.Level,
		)
		if err != nil {
			return fmt.Errorf("failed to insert provider %s: %w", p.Name, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// AddProvider adds a new provider
func (ps *ProviderServiceV2) AddProvider(kind string, provider Provider) error {
	providers, err := ps.LoadProviders(kind)
	if err != nil {
		return err
	}

	// Assign new ID
	maxID := 0
	for _, p := range providers {
		if p.ID > maxID {
			maxID = p.ID
		}
	}
	provider.ID = maxID + 1

	// Set defaults
	if provider.Tint == "" {
		provider.Tint = "#f0f0f0"
	}
	if provider.Accent == "" {
		provider.Accent = "#0a84ff"
	}
	if provider.Level == 0 {
		provider.Level = 1
	}

	providers = append(providers, provider)
	return ps.SaveProviders(kind, providers)
}

// UpdateProvider updates an existing provider
func (ps *ProviderServiceV2) UpdateProvider(kind string, provider Provider) error {
	providers, err := ps.LoadProviders(kind)
	if err != nil {
		return err
	}

	found := false
	for i, p := range providers {
		if p.ID == provider.ID {
			providers[i] = provider
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("provider with ID %d not found", provider.ID)
	}

	return ps.SaveProviders(kind, providers)
}

// DeleteProvider deletes a provider by ID
func (ps *ProviderServiceV2) DeleteProvider(kind string, providerID int) error {
	providers, err := ps.LoadProviders(kind)
	if err != nil {
		return err
	}

	filtered := make([]Provider, 0, len(providers))
	for _, p := range providers {
		if p.ID != providerID {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == len(providers) {
		return fmt.Errorf("provider with ID %d not found", providerID)
	}

	return ps.SaveProviders(kind, filtered)
}

// GetProviderByID retrieves a single provider by ID
func (ps *ProviderServiceV2) GetProviderByID(kind string, providerID int) (*Provider, error) {
	providers, err := ps.LoadProviders(kind)
	if err != nil {
		return nil, err
	}

	for _, p := range providers {
		if p.ID == providerID {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("provider with ID %d not found", providerID)
}

// GetAvailableProviders returns all enabled providers with healthy circuit state
func (ps *ProviderServiceV2) GetAvailableProviders(kind string) ([]Provider, error) {
	if !ps.useDatabase {
		// Fallback: return all enabled providers
		providers, err := ps.LoadProviders(kind)
		if err != nil {
			return nil, err
		}

		available := make([]Provider, 0)
		for _, p := range providers {
			if p.Enabled {
				available = append(available, p)
			}
		}
		return available, nil
	}

	// Use database view for available providers
	platform := normalizePlatformV2(kind)

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	rows, err := ps.db.Query(`
		SELECT id, name, api_url, api_key, official_site, icon, tint, accent,
		       enabled, supported_models, model_mapping, priority_level
		FROM available_providers
		WHERE platform = ?
		ORDER BY priority_level ASC, id ASC
	`, platform)

	if err != nil {
		return nil, fmt.Errorf("failed to query available providers: %w", err)
	}
	defer rows.Close()

	providers := []Provider{}
	for rows.Next() {
		var p Provider
		var supportedModelsJSON, modelMappingJSON sql.NullString
		var site, icon, tint, accent sql.NullString
		var enabled int

		err := rows.Scan(
			&p.ID, &p.Name, &p.APIURL, &p.APIKey,
			&site, &icon, &tint, &accent,
			&enabled, &supportedModelsJSON, &modelMappingJSON, &p.Level,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan provider row: %w", err)
		}

		p.Site = site.String
		p.Icon = icon.String
		p.Tint = tint.String
		p.Accent = accent.String
		p.Enabled = (enabled == 1)

		if supportedModelsJSON.Valid && supportedModelsJSON.String != "" {
			json.Unmarshal([]byte(supportedModelsJSON.String), &p.SupportedModels)
		}

		if modelMappingJSON.Valid && modelMappingJSON.String != "" {
			json.Unmarshal([]byte(modelMappingJSON.String), &p.ModelMapping)
		}

		providers = append(providers, p)
	}

	return providers, nil
}

// InvalidateCache invalidates any cached provider data
// This is a placeholder for future caching implementation
func (ps *ProviderServiceV2) InvalidateCache() {
	// TODO: Implement cache invalidation when caching is added in Phase 6
}

// normalizePlatformV2 normalizes platform name to standard values for database storage
func normalizePlatformV2(kind string) string {
	switch strings.ToLower(kind) {
	case "claude", "claude-code", "claude_code":
		return "claude"
	case "codex":
		return "codex"
	case "gemini-cli", "gemini_cli", "gemini":
		return "gemini"
	default:
		return kind
	}
}
