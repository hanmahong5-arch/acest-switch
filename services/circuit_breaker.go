package services

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Circuit breaker states
const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half_open"
)

// Errors
var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrCircuitHalfOpen = errors.New("circuit breaker is in half-open state, only allowing one request")
)

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of consecutive failures to trigger open state
	RecoveryTimeout  time.Duration // Time to wait before attempting recovery (half-open)
	SuccessThreshold int           // Number of successes in half-open to fully close
	UpdateDB         bool          // Whether to persist state to database
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		SuccessThreshold: 2,
		UpdateDB:         true,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	providerID   int
	providerName string
	db           *sql.DB
	config       CircuitBreakerConfig

	// Atomic state management
	state        atomic.Value // string: "closed", "open", "half_open"
	failCount    atomic.Int32
	successCount atomic.Int32 // Used in half-open state
	lastFailTime atomic.Value // time.Time
	circuitOpenTime atomic.Value // time.Time

	// Metrics
	totalRequests atomic.Int64
	totalFailures atomic.Int64
	totalSuccesses atomic.Int64

	// Half-open state synchronization
	halfOpenMutex sync.Mutex
	halfOpenTest  bool // Only allow one test request in half-open state

	mu sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(providerID int, providerName string, db *sql.DB, config CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		providerID:   providerID,
		providerName: providerName,
		db:           db,
		config:       config,
	}

	// Initialize state
	cb.state.Store(StateClosed)
	cb.lastFailTime.Store(time.Time{})
	cb.circuitOpenTime.Store(time.Time{})

	// Load state from database if available
	if db != nil && config.UpdateDB {
		cb.loadFromDB()
	}

	return cb
}

// loadFromDB loads circuit breaker state from database
func (cb *CircuitBreaker) loadFromDB() {
	var circuitState string
	var consecutiveFails int
	var lastFailTime, circuitOpenedAt sql.NullTime

	err := cb.db.QueryRow(`
		SELECT circuit_state, consecutive_fails, last_failure_at, circuit_opened_at
		FROM provider_health
		WHERE provider_id = ?
	`, cb.providerID).Scan(&circuitState, &consecutiveFails, &lastFailTime, &circuitOpenedAt)

	if err == nil {
		cb.state.Store(circuitState)
		cb.failCount.Store(int32(consecutiveFails))

		if lastFailTime.Valid {
			cb.lastFailTime.Store(lastFailTime.Time)
		}

		if circuitOpenedAt.Valid {
			cb.circuitOpenTime.Store(circuitOpenedAt.Time)
		}
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	// Check if request is allowed
	if !cb.AllowRequest() {
		cb.totalRequests.Add(1)
		return ErrCircuitOpen
	}

	// Execute function
	cb.totalRequests.Add(1)
	err := fn()

	// Handle result
	if err != nil {
		cb.OnFailure()
		return err
	}

	cb.OnSuccess()
	return nil
}

// AllowRequest checks if a request is allowed based on circuit state
func (cb *CircuitBreaker) AllowRequest() bool {
	state := cb.GetState()

	switch state {
	case StateClosed:
		// Circuit is closed, allow all requests
		return true

	case StateOpen:
		// Check if recovery timeout has elapsed
		circuitOpenTime := cb.circuitOpenTime.Load().(time.Time)
		if time.Since(circuitOpenTime) > cb.config.RecoveryTimeout {
			// Transition to half-open state
			cb.setState(StateHalfOpen)
			cb.halfOpenTest = false
			return true
		}
		// Circuit is still open, reject request
		return false

	case StateHalfOpen:
		// In half-open state, only allow one test request at a time
		cb.halfOpenMutex.Lock()
		defer cb.halfOpenMutex.Unlock()

		if cb.halfOpenTest {
			// A test request is already in progress
			return false
		}

		// Allow this request as the test request
		cb.halfOpenTest = true
		return true

	default:
		// Unknown state, default to closed
		return true
	}
}

// OnSuccess handles a successful request
func (cb *CircuitBreaker) OnSuccess() {
	cb.totalSuccesses.Add(1)
	state := cb.GetState()

	switch state {
	case StateClosed:
		// Reset failure count on success
		cb.failCount.Store(0)

	case StateHalfOpen:
		// Increment success count in half-open state
		successCount := cb.successCount.Add(1)

		cb.halfOpenMutex.Lock()
		cb.halfOpenTest = false
		cb.halfOpenMutex.Unlock()

		// Check if we've reached success threshold
		if int(successCount) >= cb.config.SuccessThreshold {
			// Fully close the circuit
			cb.setState(StateClosed)
			cb.failCount.Store(0)
			cb.successCount.Store(0)
		}

	case StateOpen:
		// Should not happen, but reset if it does
		cb.setState(StateHalfOpen)
	}

	// Update database
	cb.updateDB()
}

// OnFailure handles a failed request
func (cb *CircuitBreaker) OnFailure() {
	cb.totalFailures.Add(1)
	state := cb.GetState()

	now := time.Now()
	cb.lastFailTime.Store(now)

	fails := cb.failCount.Add(1)

	switch state {
	case StateClosed:
		// Check if we've reached failure threshold
		if int(fails) >= cb.config.FailureThreshold {
			// Open the circuit
			cb.setState(StateOpen)
			cb.circuitOpenTime.Store(now)
		}

	case StateHalfOpen:
		// Failure in half-open state means circuit should reopen
		cb.setState(StateOpen)
		cb.circuitOpenTime.Store(now)
		cb.successCount.Store(0)

		cb.halfOpenMutex.Lock()
		cb.halfOpenTest = false
		cb.halfOpenMutex.Unlock()

	case StateOpen:
		// Already open, just record the failure
		// This shouldn't happen if AllowRequest is working correctly
	}

	// Update database
	cb.updateDB()
}

// GetState returns the current circuit state
func (cb *CircuitBreaker) GetState() string {
	return cb.state.Load().(string)
}

// setState sets the circuit state
func (cb *CircuitBreaker) setState(newState string) {
	oldState := cb.GetState()

	if oldState != newState {
		cb.state.Store(newState)

		// Log state transition
		fmt.Printf("[CircuitBreaker] Provider %s (ID=%d): %s → %s\n",
			cb.providerName, cb.providerID, oldState, newState)
	}
}

// updateDB persists circuit breaker state to database
func (cb *CircuitBreaker) updateDB() {
	if cb.db == nil || !cb.config.UpdateDB {
		return
	}

	state := cb.GetState()
	fails := cb.failCount.Load()

	lastFailTime := cb.lastFailTime.Load().(time.Time)
	var lastFailTimePtr *time.Time
	if !lastFailTime.IsZero() {
		lastFailTimePtr = &lastFailTime
	}

	circuitOpenTime := cb.circuitOpenTime.Load().(time.Time)
	var circuitOpenTimePtr *time.Time
	if !circuitOpenTime.IsZero() {
		circuitOpenTimePtr = &circuitOpenTime
	}

	// Upsert provider_health record
	_, err := cb.db.Exec(`
		INSERT INTO provider_health (
			provider_id, circuit_state, consecutive_fails,
			total_requests, total_failures,
			last_failure_at, circuit_opened_at, last_checked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(provider_id) DO UPDATE SET
			circuit_state = excluded.circuit_state,
			consecutive_fails = excluded.consecutive_fails,
			total_requests = total_requests + 1,
			total_failures = excluded.total_failures,
			last_failure_at = excluded.last_failure_at,
			circuit_opened_at = excluded.circuit_opened_at,
			last_checked_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
	`, cb.providerID, state, fails, cb.totalRequests.Load(), cb.totalFailures.Load(),
		lastFailTimePtr, circuitOpenTimePtr)

	if err != nil {
		fmt.Printf("[CircuitBreaker] Failed to update database: %v\n", err)
	}
}

// GetMetrics returns current circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() CircuitBreakerMetrics {
	totalReqs := cb.totalRequests.Load()
	totalFails := cb.totalFailures.Load()
	totalSuccesses := cb.totalSuccesses.Load()

	var successRate float64
	if totalReqs > 0 {
		successRate = float64(totalSuccesses) / float64(totalReqs) * 100
	}

	lastFail := cb.lastFailTime.Load().(time.Time)
	circuitOpen := cb.circuitOpenTime.Load().(time.Time)

	return CircuitBreakerMetrics{
		ProviderID:       cb.providerID,
		ProviderName:     cb.providerName,
		State:            cb.GetState(),
		ConsecutiveFails: int(cb.failCount.Load()),
		TotalRequests:    totalReqs,
		TotalFailures:    totalFails,
		TotalSuccesses:   totalSuccesses,
		SuccessRate:      successRate,
		LastFailureAt:    lastFail,
		CircuitOpenedAt:  circuitOpen,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.setState(StateClosed)
	cb.failCount.Store(0)
	cb.successCount.Store(0)
	cb.lastFailTime.Store(time.Time{})
	cb.circuitOpenTime.Store(time.Time{})
	cb.updateDB()
}

// CircuitBreakerMetrics holds circuit breaker metrics
type CircuitBreakerMetrics struct {
	ProviderID       int       `json:"provider_id"`
	ProviderName     string    `json:"provider_name"`
	State            string    `json:"state"`
	ConsecutiveFails int       `json:"consecutive_fails"`
	TotalRequests    int64     `json:"total_requests"`
	TotalFailures    int64     `json:"total_failures"`
	TotalSuccesses   int64     `json:"total_successes"`
	SuccessRate      float64   `json:"success_rate"`
	LastFailureAt    time.Time `json:"last_failure_at,omitempty"`
	CircuitOpenedAt  time.Time `json:"circuit_opened_at,omitempty"`
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[int]*CircuitBreaker
	db       *sql.DB
	config   CircuitBreakerConfig
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(db *sql.DB, config CircuitBreakerConfig) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[int]*CircuitBreaker),
		db:       db,
		config:   config,
	}
}

// GetCircuitBreaker returns a circuit breaker for a provider
func (m *CircuitBreakerManager) GetCircuitBreaker(providerID int, providerName string) *CircuitBreaker {
	m.mu.RLock()
	cb, exists := m.breakers[providerID]
	m.mu.RUnlock()

	if exists {
		return cb
	}

	// Create new circuit breaker
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := m.breakers[providerID]; exists {
		return cb
	}

	cb = NewCircuitBreaker(providerID, providerName, m.db, m.config)
	m.breakers[providerID] = cb

	return cb
}

// GetAllMetrics returns metrics for all circuit breakers
func (m *CircuitBreakerManager) GetAllMetrics() []CircuitBreakerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make([]CircuitBreakerMetrics, 0, len(m.breakers))
	for _, cb := range m.breakers {
		metrics = append(metrics, cb.GetMetrics())
	}

	return metrics
}

// ResetCircuitBreaker manually resets a circuit breaker
func (m *CircuitBreakerManager) ResetCircuitBreaker(providerID int) error {
	m.mu.RLock()
	cb, exists := m.breakers[providerID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("circuit breaker not found for provider %d", providerID)
	}

	cb.Reset()
	return nil
}
