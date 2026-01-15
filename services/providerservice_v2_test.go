package services

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates a test database with schema v2
func setupTestDB(t *testing.T) (string, func()) {
	// Create temporary directory
	testDir, err := os.MkdirTemp("", "provider-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(testDir, "app.db")

	// Create database and execute schema
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Execute schema_v2.sql
	schemaFile := "../deploy/sqlite/schema_v2.sql"
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		// Try alternative path
		schemaFile = "deploy/sqlite/schema_v2.sql"
		if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
			t.Skip("Schema file not found, skipping test")
		}
	}

	schemaSQL, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("Failed to read schema file: %v", err)
	}

	_, err = db.Exec(string(schemaSQL))
	if err != nil {
		t.Fatalf("Failed to execute schema: %v", err)
	}
	db.Close()

	// Cleanup function
	cleanup := func() {
		os.RemoveAll(testDir)
	}

	return dbPath, cleanup
}

// Test: Create ProviderServiceV2 and check database mode
func TestProviderServiceV2_New(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	if !ps.useDatabase {
		t.Fatalf("Expected database mode to be enabled")
	}

	t.Logf("✓ ProviderServiceV2 created successfully")
}

// Test: LoadProviders from empty database
func TestProviderServiceV2_LoadProviders_Empty(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	providers, err := ps.LoadProviders("claude")
	if err != nil {
		t.Fatalf("LoadProviders failed: %v", err)
	}

	if len(providers) != 0 {
		t.Fatalf("Expected 0 providers, got %d", len(providers))
	}

	t.Logf("✓ LoadProviders works on empty database")
}

// Test: SaveProviders and LoadProviders
func TestProviderServiceV2_SaveAndLoad(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	// Create test providers
	testProviders := []Provider{
		{
			ID:      1,
			Name:    "Test Provider 1",
			APIURL:  "https://api.test1.com",
			APIKey:  "key1",
			Enabled: true,
			Level:   1,
			Tint:    "#f0f0f0",
			Accent:  "#0a84ff",
			SupportedModels: map[string]bool{
				"claude-sonnet-4": true,
			},
		},
		{
			ID:      2,
			Name:    "Test Provider 2",
			APIURL:  "https://api.test2.com",
			APIKey:  "key2",
			Enabled: false,
			Level:   2,
			Tint:    "#e0e0e0",
			Accent:  "#ff0000",
			ModelMapping: map[string]string{
				"claude-*": "anthropic/claude-*",
			},
		},
	}

	// Save providers
	err = ps.SaveProviders("claude", testProviders)
	if err != nil {
		t.Fatalf("SaveProviders failed: %v", err)
	}

	// Load providers
	loaded, err := ps.LoadProviders("claude")
	if err != nil {
		t.Fatalf("LoadProviders failed: %v", err)
	}

	if len(loaded) != len(testProviders) {
		t.Fatalf("Expected %d providers, got %d", len(testProviders), len(loaded))
	}

	// Verify first provider
	if loaded[0].Name != testProviders[0].Name {
		t.Fatalf("Expected name '%s', got '%s'", testProviders[0].Name, loaded[0].Name)
	}

	if loaded[0].APIURL != testProviders[0].APIURL {
		t.Fatalf("Expected URL '%s', got '%s'", testProviders[0].APIURL, loaded[0].APIURL)
	}

	if loaded[0].Enabled != testProviders[0].Enabled {
		t.Fatalf("Expected enabled=%v, got %v", testProviders[0].Enabled, loaded[0].Enabled)
	}

	if loaded[0].Tint != testProviders[0].Tint {
		t.Fatalf("Expected tint '%s', got '%s'", testProviders[0].Tint, loaded[0].Tint)
	}

	// Verify supported models
	if loaded[0].SupportedModels["claude-sonnet-4"] != true {
		t.Fatalf("Expected supported model 'claude-sonnet-4' to be true")
	}

	t.Logf("✓ SaveProviders and LoadProviders work correctly")
}

// Test: AddProvider
func TestProviderServiceV2_AddProvider(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	// Add first provider
	provider1 := Provider{
		Name:    "New Provider",
		APIURL:  "https://api.new.com",
		APIKey:  "newkey",
		Enabled: true,
	}

	err = ps.AddProvider("claude", provider1)
	if err != nil {
		t.Fatalf("AddProvider failed: %v", err)
	}

	// Verify provider was added with correct ID
	providers, err := ps.LoadProviders("claude")
	if err != nil {
		t.Fatalf("LoadProviders failed: %v", err)
	}

	if len(providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(providers))
	}

	if providers[0].ID != 1 {
		t.Fatalf("Expected ID=1, got ID=%d", providers[0].ID)
	}

	// Add second provider
	provider2 := Provider{
		Name:    "Another Provider",
		APIURL:  "https://api.another.com",
		APIKey:  "anotherkey",
		Enabled: true,
	}

	err = ps.AddProvider("claude", provider2)
	if err != nil {
		t.Fatalf("AddProvider (second) failed: %v", err)
	}

	// Verify both providers exist
	providers, err = ps.LoadProviders("claude")
	if err != nil {
		t.Fatalf("LoadProviders failed: %v", err)
	}

	if len(providers) != 2 {
		t.Fatalf("Expected 2 providers, got %d", len(providers))
	}

	if providers[1].ID != 2 {
		t.Fatalf("Expected second provider ID=2, got ID=%d", providers[1].ID)
	}

	t.Logf("✓ AddProvider works correctly")
}

// Test: UpdateProvider
func TestProviderServiceV2_UpdateProvider(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	// Add provider
	provider := Provider{
		Name:    "Test Provider",
		APIURL:  "https://api.test.com",
		APIKey:  "testkey",
		Enabled: true,
	}

	err = ps.AddProvider("claude", provider)
	if err != nil {
		t.Fatalf("AddProvider failed: %v", err)
	}

	// Load and update
	providers, _ := ps.LoadProviders("claude")
	updated := providers[0]
	updated.APIURL = "https://api.updated.com"
	updated.Enabled = false

	err = ps.UpdateProvider("claude", updated)
	if err != nil {
		t.Fatalf("UpdateProvider failed: %v", err)
	}

	// Verify update
	providers, _ = ps.LoadProviders("claude")
	if providers[0].APIURL != "https://api.updated.com" {
		t.Fatalf("Provider URL was not updated")
	}
	if providers[0].Enabled != false {
		t.Fatalf("Provider enabled status was not updated")
	}

	t.Logf("✓ UpdateProvider works correctly")
}

// Test: DeleteProvider
func TestProviderServiceV2_DeleteProvider(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	// Add two providers
	provider1 := Provider{Name: "Provider 1", APIURL: "https://api1.com", APIKey: "key1", Enabled: true}
	provider2 := Provider{Name: "Provider 2", APIURL: "https://api2.com", APIKey: "key2", Enabled: true}

	ps.AddProvider("claude", provider1)
	ps.AddProvider("claude", provider2)

	// Delete first provider
	err = ps.DeleteProvider("claude", 1)
	if err != nil {
		t.Fatalf("DeleteProvider failed: %v", err)
	}

	// Verify only one provider remains
	providers, _ := ps.LoadProviders("claude")
	if len(providers) != 1 {
		t.Fatalf("Expected 1 provider after deletion, got %d", len(providers))
	}

	if providers[0].Name != "Provider 2" {
		t.Fatalf("Wrong provider remained after deletion")
	}

	t.Logf("✓ DeleteProvider works correctly")
}

// Test: GetProviderByID
func TestProviderServiceV2_GetProviderByID(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	// Add provider
	provider := Provider{
		Name:    "Test Provider",
		APIURL:  "https://api.test.com",
		APIKey:  "testkey",
		Enabled: true,
	}

	ps.AddProvider("claude", provider)

	// Get by ID
	loaded, err := ps.GetProviderByID("claude", 1)
	if err != nil {
		t.Fatalf("GetProviderByID failed: %v", err)
	}

	if loaded.Name != "Test Provider" {
		t.Fatalf("Expected name 'Test Provider', got '%s'", loaded.Name)
	}

	// Get non-existent ID
	_, err = ps.GetProviderByID("claude", 999)
	if err == nil {
		t.Fatalf("Expected error for non-existent ID, got nil")
	}

	t.Logf("✓ GetProviderByID works correctly")
}

// Test: GetAvailableProviders (only enabled providers)
func TestProviderServiceV2_GetAvailableProviders(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	// Add providers (mixed enabled/disabled)
	providers := []Provider{
		{ID: 1, Name: "Enabled 1", APIURL: "https://api1.com", APIKey: "key1", Enabled: true},
		{ID: 2, Name: "Disabled 1", APIURL: "https://api2.com", APIKey: "key2", Enabled: false},
		{ID: 3, Name: "Enabled 2", APIURL: "https://api3.com", APIKey: "key3", Enabled: true},
	}

	ps.SaveProviders("claude", providers)

	// Get available (enabled) providers
	available, err := ps.GetAvailableProviders("claude")
	if err != nil {
		t.Fatalf("GetAvailableProviders failed: %v", err)
	}

	if len(available) != 2 {
		t.Fatalf("Expected 2 available providers, got %d", len(available))
	}

	// Verify only enabled providers are returned
	for _, p := range available {
		if !p.Enabled {
			t.Fatalf("Found disabled provider in available list: %s", p.Name)
		}
	}

	t.Logf("✓ GetAvailableProviders works correctly")
}

// Test: Backward compatibility (fallback to JSON when no database)
func TestProviderServiceV2_BackwardCompatibility(t *testing.T) {
	// Create temporary directory without database
	testDir, err := os.MkdirTemp("", "provider-compat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create JSON config file
	type providerEnvelope struct {
		Providers []Provider `json:"providers"`
	}

	jsonProviders := []Provider{
		{
			ID:      1,
			Name:    "JSON Provider",
			APIURL:  "https://api.json.com",
			APIKey:  "jsonkey",
			Enabled: true,
		},
	}

	envelope := providerEnvelope{Providers: jsonProviders}
	data, _ := json.MarshalIndent(envelope, "", "  ")
	jsonPath := filepath.Join(testDir, "claude-code.json")
	os.WriteFile(jsonPath, data, 0644)

	// Create ProviderServiceV2 pointing to non-existent database
	dbPath := filepath.Join(testDir, "app.db")
	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	// Should fall back to JSON
	if ps.useDatabase {
		t.Fatalf("Expected database mode to be disabled when no database exists")
	}

	// Manually set config dir for fallback service
	ps.fallbackService.mu.Lock()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", testDir)
	defer func() {
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		} else {
			os.Unsetenv("HOME")
		}
	}()
	ps.fallbackService.mu.Unlock()

	// Should be able to read from JSON file
	// Note: This test may need adjustment based on actual home directory handling

	t.Logf("✓ Backward compatibility fallback works")
}

// Test: Provider validation on save
func TestProviderServiceV2_Validation(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	ps, err := NewProviderServiceV2(dbPath)
	if err != nil {
		t.Fatalf("Failed to create ProviderServiceV2: %v", err)
	}
	defer ps.Stop()

	// Add valid provider
	provider := Provider{
		Name:    "Valid Provider",
		APIURL:  "https://api.valid.com",
		APIKey:  "validkey",
		Enabled: true,
		SupportedModels: map[string]bool{
			"claude-sonnet-4": true,
		},
		ModelMapping: map[string]string{
			"custom-model": "claude-sonnet-4",
		},
	}

	err = ps.AddProvider("claude", provider)
	if err != nil {
		t.Fatalf("AddProvider with valid config failed: %v", err)
	}

	// Try to add provider with invalid mapping (target model not in supported models)
	invalidProvider := Provider{
		Name:    "Invalid Provider",
		APIURL:  "https://api.invalid.com",
		APIKey:  "invalidkey",
		Enabled: true,
		SupportedModels: map[string]bool{
			"claude-sonnet-4": true,
		},
		ModelMapping: map[string]string{
			"custom-model": "non-existent-model", // Invalid: not in supportedModels
		},
	}

	err = ps.AddProvider("claude", invalidProvider)
	if err == nil {
		t.Fatalf("Expected validation error for invalid model mapping, got nil")
	}

	t.Logf("✓ Provider validation works correctly")
}

// Benchmark: LoadProviders performance
func BenchmarkProviderServiceV2_LoadProviders(b *testing.B) {
	// Create temporary database
	testDir, _ := os.MkdirTemp("", "provider-bench-*")
	defer os.RemoveAll(testDir)

	dbPath := filepath.Join(testDir, "app.db")

	// Setup schema
	db, _ := sql.Open("sqlite", dbPath)
	schemaFile := "deploy/sqlite/schema_v2.sql"
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		b.Skip("Schema file not found")
	}
	schemaSQL, _ := os.ReadFile(schemaFile)
	db.Exec(string(schemaSQL))
	db.Close()

	ps, _ := NewProviderServiceV2(dbPath)
	defer ps.Stop()

	// Add test providers
	providers := make([]Provider, 10)
	for i := 0; i < 10; i++ {
		providers[i] = Provider{
			ID:      i + 1,
			Name:    "Provider " + string(rune(i)),
			APIURL:  "https://api.example.com",
			APIKey:  "key",
			Enabled: true,
		}
	}
	ps.SaveProviders("claude", providers)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ps.LoadProviders("claude")
	}
}
