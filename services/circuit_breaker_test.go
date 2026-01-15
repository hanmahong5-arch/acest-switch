package services

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDBForCircuitBreaker creates a test database
func setupTestDBForCircuitBreaker(t *testing.T) (*sql.DB, func()) {
	// Create temporary directory
	testDir, err := os.MkdirTemp("", "cb-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(testDir, "test.db")

	// Create database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Create provider_health table
	_, err = db.Exec(`
		CREATE TABLE provider_health (
			provider_id INTEGER PRIMARY KEY,
			circuit_state TEXT DEFAULT 'closed',
			consecutive_fails INTEGER DEFAULT 0,
			total_requests INTEGER DEFAULT 0,
			total_failures INTEGER DEFAULT 0,
			last_failure_at DATETIME,
			circuit_opened_at DATETIME,
			last_checked_at DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(testDir)
	}

	return db, cleanup
}

// Test: CircuitBreaker creation
func TestCircuitBreaker_New(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(1, "TestProvider", db, config)

	if cb == nil {
		t.Fatal("CircuitBreaker should not be nil")
	}

	if cb.GetState() != StateClosed {
		t.Fatalf("Expected initial state to be %s, got %s", StateClosed, cb.GetState())
	}

	if cb.providerID != 1 {
		t.Fatalf("Expected provider ID 1, got %d", cb.providerID)
	}

	if cb.providerName != "TestProvider" {
		t.Fatalf("Expected provider name 'TestProvider', got '%s'", cb.providerName)
	}
}

// Test: Circuit breaker transitions from closed to open after threshold failures
func TestCircuitBreaker_TransitionToOpen(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  10 * time.Second,
		SuccessThreshold: 2,
		UpdateDB:         true,
	}

	cb := NewCircuitBreaker(1, "TestProvider", db, config)

	// Initial state should be closed
	if cb.GetState() != StateClosed {
		t.Fatal("Expected initial state to be closed")
	}

	// Simulate 2 failures (below threshold)
	cb.OnFailure()
	cb.OnFailure()

	if cb.GetState() != StateClosed {
		t.Fatal("Circuit should still be closed after 2 failures")
	}

	// 3rd failure should trigger circuit open
	cb.OnFailure()

	if cb.GetState() != StateOpen {
		t.Fatalf("Expected circuit to be open after %d failures, got %s", config.FailureThreshold, cb.GetState())
	}

	// Verify database was updated
	var state string
	err := db.QueryRow("SELECT circuit_state FROM provider_health WHERE provider_id = 1").Scan(&state)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if state != StateOpen {
		t.Fatalf("Expected database state to be %s, got %s", StateOpen, state)
	}
}

// Test: Circuit breaker allows recovery after timeout
func TestCircuitBreaker_RecoveryAfterTimeout(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond, // Short timeout for testing
		SuccessThreshold: 1,
		UpdateDB:         false,
	}

	cb := NewCircuitBreaker(1, "TestProvider", db, config)

	// Trigger circuit open
	cb.OnFailure()
	cb.OnFailure()

	if cb.GetState() != StateOpen {
		t.Fatal("Circuit should be open")
	}

	// Should not allow requests immediately
	if cb.AllowRequest() {
		t.Fatal("Circuit should not allow requests when open")
	}

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Should now allow one test request (half-open state)
	if !cb.AllowRequest() {
		t.Fatal("Circuit should allow test request after recovery timeout")
	}

	if cb.GetState() != StateHalfOpen {
		t.Fatalf("Expected state to be %s after recovery timeout, got %s", StateHalfOpen, cb.GetState())
	}
}

// Test: Circuit breaker closes after successful requests in half-open state
func TestCircuitBreaker_CloseAfterSuccess(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  50 * time.Millisecond,
		SuccessThreshold: 2,
		UpdateDB:         false,
	}

	cb := NewCircuitBreaker(1, "TestProvider", db, config)

	// Trigger circuit open
	cb.OnFailure()
	cb.OnFailure()

	// Wait for recovery
	time.Sleep(100 * time.Millisecond)

	// Transition to half-open
	cb.AllowRequest()

	if cb.GetState() != StateHalfOpen {
		t.Fatal("Expected state to be half-open")
	}

	// First successful request
	cb.OnSuccess()

	// Still in half-open (need 2 successes)
	if cb.GetState() != StateHalfOpen {
		t.Fatal("Should still be in half-open state after 1 success")
	}

	// Allow next test request
	cb.AllowRequest()

	// Second successful request should fully close circuit
	cb.OnSuccess()

	if cb.GetState() != StateClosed {
		t.Fatalf("Expected state to be %s after %d successes, got %s",
			StateClosed, config.SuccessThreshold, cb.GetState())
	}
}

// Test: Circuit breaker reopens on failure in half-open state
func TestCircuitBreaker_ReopenOnHalfOpenFailure(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  50 * time.Millisecond,
		SuccessThreshold: 2,
		UpdateDB:         false,
	}

	cb := NewCircuitBreaker(1, "TestProvider", db, config)

	// Trigger circuit open
	cb.OnFailure()
	cb.OnFailure()

	// Wait for recovery
	time.Sleep(100 * time.Millisecond)

	// Transition to half-open
	cb.AllowRequest()

	if cb.GetState() != StateHalfOpen {
		t.Fatal("Expected state to be half-open")
	}

	// Failure in half-open should reopen circuit
	cb.OnFailure()

	if cb.GetState() != StateOpen {
		t.Fatalf("Expected state to be %s after failure in half-open, got %s",
			StateOpen, cb.GetState())
	}
}

// Test: Circuit breaker Call wrapper
func TestCircuitBreaker_Call(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  10 * time.Second,
		SuccessThreshold: 1,
		UpdateDB:         false,
	}

	cb := NewCircuitBreaker(1, "TestProvider", db, config)

	// Test successful call
	err := cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("Successful call should not return error, got %v", err)
	}

	// Test failing call
	testError := errors.New("test error")
	err = cb.Call(func() error {
		return testError
	})

	if err != testError {
		t.Fatalf("Expected error to be propagated, got %v", err)
	}

	// Trigger circuit open
	cb.Call(func() error {
		return errors.New("failure")
	})

	// Call should now return ErrCircuitOpen
	err = cb.Call(func() error {
		t.Fatal("This function should not be called when circuit is open")
		return nil
	})

	if err != ErrCircuitOpen {
		t.Fatalf("Expected ErrCircuitOpen, got %v", err)
	}
}

// Test: GetMetrics returns correct metrics
func TestCircuitBreaker_GetMetrics(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(1, "TestProvider", db, config)

	// Simulate some requests
	cb.OnSuccess()
	cb.OnSuccess()
	cb.OnFailure()

	metrics := cb.GetMetrics()

	if metrics.ProviderID != 1 {
		t.Fatalf("Expected provider ID 1, got %d", metrics.ProviderID)
	}

	if metrics.ProviderName != "TestProvider" {
		t.Fatalf("Expected provider name 'TestProvider', got '%s'", metrics.ProviderName)
	}

	if metrics.TotalRequests != 3 {
		t.Fatalf("Expected 3 total requests, got %d", metrics.TotalRequests)
	}

	if metrics.TotalSuccesses != 2 {
		t.Fatalf("Expected 2 successes, got %d", metrics.TotalSuccesses)
	}

	if metrics.TotalFailures != 1 {
		t.Fatalf("Expected 1 failure, got %d", metrics.TotalFailures)
	}

	expectedRate := (2.0 / 3.0) * 100
	if metrics.SuccessRate < expectedRate-0.1 || metrics.SuccessRate > expectedRate+0.1 {
		t.Fatalf("Expected success rate ~%.1f%%, got %.1f%%", expectedRate, metrics.SuccessRate)
	}
}

// Test: Reset circuit breaker
func TestCircuitBreaker_Reset(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(1, "TestProvider", db, config)

	// Trigger failures
	for i := 0; i < 5; i++ {
		cb.OnFailure()
	}

	if cb.GetState() != StateOpen {
		t.Fatal("Circuit should be open after failures")
	}

	// Reset circuit breaker
	cb.Reset()

	if cb.GetState() != StateClosed {
		t.Fatalf("Expected state to be %s after reset, got %s", StateClosed, cb.GetState())
	}

	if cb.failCount.Load() != 0 {
		t.Fatalf("Expected fail count to be 0 after reset, got %d", cb.failCount.Load())
	}
}

// Test: CircuitBreakerManager
func TestCircuitBreakerManager_GetCircuitBreaker(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := DefaultCircuitBreakerConfig()
	manager := NewCircuitBreakerManager(db, config)

	// Get circuit breaker for provider 1
	cb1 := manager.GetCircuitBreaker(1, "Provider1")
	if cb1 == nil {
		t.Fatal("CircuitBreaker should not be nil")
	}

	// Get same circuit breaker again (should return cached instance)
	cb1Again := manager.GetCircuitBreaker(1, "Provider1")
	if cb1 != cb1Again {
		t.Fatal("Should return same circuit breaker instance")
	}

	// Get circuit breaker for different provider
	cb2 := manager.GetCircuitBreaker(2, "Provider2")
	if cb2 == nil {
		t.Fatal("CircuitBreaker should not be nil")
	}

	if cb1 == cb2 {
		t.Fatal("Different providers should have different circuit breakers")
	}
}

// Test: CircuitBreakerManager GetAllMetrics
func TestCircuitBreakerManager_GetAllMetrics(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := DefaultCircuitBreakerConfig()
	manager := NewCircuitBreakerManager(db, config)

	// Create circuit breakers for 3 providers
	cb1 := manager.GetCircuitBreaker(1, "Provider1")
	cb2 := manager.GetCircuitBreaker(2, "Provider2")
	cb3 := manager.GetCircuitBreaker(3, "Provider3")

	// Simulate some activity
	cb1.OnSuccess()
	cb2.OnFailure()
	cb3.OnSuccess()

	// Get all metrics
	metrics := manager.GetAllMetrics()

	if len(metrics) != 3 {
		t.Fatalf("Expected 3 metrics, got %d", len(metrics))
	}

	// Verify each metric has correct provider ID
	foundIDs := make(map[int]bool)
	for _, m := range metrics {
		foundIDs[m.ProviderID] = true
	}

	for i := 1; i <= 3; i++ {
		if !foundIDs[i] {
			t.Fatalf("Missing metrics for provider %d", i)
		}
	}
}

// Test: CircuitBreakerManager ResetCircuitBreaker
func TestCircuitBreakerManager_ResetCircuitBreaker(t *testing.T) {
	db, cleanup := setupTestDBForCircuitBreaker(t)
	defer cleanup()

	config := DefaultCircuitBreakerConfig()
	manager := NewCircuitBreakerManager(db, config)

	cb := manager.GetCircuitBreaker(1, "Provider1")

	// Trigger circuit open
	for i := 0; i < 5; i++ {
		cb.OnFailure()
	}

	if cb.GetState() != StateOpen {
		t.Fatal("Circuit should be open")
	}

	// Reset via manager
	err := manager.ResetCircuitBreaker(1)
	if err != nil {
		t.Fatalf("Failed to reset circuit breaker: %v", err)
	}

	if cb.GetState() != StateClosed {
		t.Fatalf("Expected state to be %s after reset, got %s", StateClosed, cb.GetState())
	}

	// Try to reset non-existent circuit breaker
	err = manager.ResetCircuitBreaker(999)
	if err == nil {
		t.Fatal("Expected error when resetting non-existent circuit breaker")
	}
}

// Benchmark: Circuit breaker Call overhead
func BenchmarkCircuitBreaker_Call(b *testing.B) {
	config := CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		SuccessThreshold: 2,
		UpdateDB:         false, // Disable DB updates for benchmarking
	}

	cb := NewCircuitBreaker(1, "TestProvider", nil, config)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cb.Call(func() error {
			return nil
		})
	}
}

// Benchmark: Circuit breaker concurrent access
func BenchmarkCircuitBreaker_ConcurrentAccess(b *testing.B) {
	config := CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		SuccessThreshold: 2,
		UpdateDB:         false,
	}

	cb := NewCircuitBreaker(1, "TestProvider", nil, config)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Call(func() error {
				return nil
			})
		}
	})
}
