package services

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ProviderRelayServiceWithCircuitBreaker extends ProviderRelayService with circuit breaker support
type ProviderRelayServiceWithCircuitBreaker struct {
	*ProviderRelayService
	circuitBreakerManager *CircuitBreakerManager
	providerServiceV2     *ProviderServiceV2
	db                    *sql.DB
	mu                    sync.RWMutex
}

// NewProviderRelayServiceWithCircuitBreaker creates a new service with circuit breaker support
func NewProviderRelayServiceWithCircuitBreaker(
	providerService *ProviderService,
	providerServiceV2 *ProviderServiceV2,
	db *sql.DB,
	addr string,
) *ProviderRelayServiceWithCircuitBreaker {

	baseService := NewProviderRelayService(providerService, addr)

	// Create circuit breaker manager
	cbConfig := DefaultCircuitBreakerConfig()
	cbManager := NewCircuitBreakerManager(db, cbConfig)

	return &ProviderRelayServiceWithCircuitBreaker{
		ProviderRelayService:  baseService,
		circuitBreakerManager: cbManager,
		providerServiceV2:     providerServiceV2,
		db:                    db,
	}
}

// selectProviderWithCircuitBreaker selects a provider considering circuit breaker state
func (prs *ProviderRelayServiceWithCircuitBreaker) selectProviderWithCircuitBreaker(
	kind string,
	model string,
) (*Provider, *CircuitBreaker, error) {

	// Load providers (use V2 if available, fallback to V1)
	var providers []Provider
	var err error

	if prs.providerServiceV2 != nil {
		providers, err = prs.providerServiceV2.GetAvailableProviders(kind)
	} else {
		providers, err = prs.providerService.LoadProviders(kind)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load providers: %w", err)
		}

		// Filter enabled providers
		active := make([]Provider, 0, len(providers))
		for _, p := range providers {
			if p.Enabled && p.APIURL != "" && p.APIKey != "" {
				active = append(active, p)
			}
		}
		providers = active
	}

	if len(providers) == 0 {
		return nil, nil, fmt.Errorf("no providers available")
	}

	// Filter providers by model support
	candidates := make([]Provider, 0)
	for _, p := range providers {
		if model == "" || p.IsModelSupported(model) {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no providers support model: %s", model)
	}

	// Filter providers by circuit breaker state
	healthyCandidates := make([]Provider, 0)
	for _, p := range candidates {
		cb := prs.circuitBreakerManager.GetCircuitBreaker(p.ID, p.Name)
		if cb.AllowRequest() {
			healthyCandidates = append(healthyCandidates, p)
		} else {
			fmt.Printf("[Circuit Breaker] Provider %s (ID=%d) is %s, skipping\n",
				p.Name, p.ID, cb.GetState())
		}
	}

	if len(healthyCandidates) == 0 {
		return nil, nil, fmt.Errorf("no healthy providers available (all circuit breakers open)")
	}

	// Select provider using round-robin or priority
	var selected Provider
	if prs.ProviderRelayService.IsRoundRobinEnabled() {
		// Round-robin selection
		idx := int(atomic.AddUint64(&prs.ProviderRelayService.rrCounter, 1)-1) % len(healthyCandidates)
		selected = healthyCandidates[idx]
	} else {
		// Priority-based selection (lowest priority level first)
		selected = healthyCandidates[0]
	}

	// Get circuit breaker for selected provider
	cb := prs.circuitBreakerManager.GetCircuitBreaker(selected.ID, selected.Name)

	return &selected, cb, nil
}

// executeRequestWithCircuitBreaker executes a request with circuit breaker protection
func (prs *ProviderRelayServiceWithCircuitBreaker) executeRequestWithCircuitBreaker(
	provider *Provider,
	cb *CircuitBreaker,
	targetURL string,
	bodyBytes []byte,
	headers map[string]string,
	isStream bool,
) (*http.Response, error) {

	var resp *http.Response

	// Wrap request in circuit breaker
	cbErr := cb.Call(func() error {
		// Create request
		req, reqErr := http.NewRequest("POST", targetURL, bytes.NewReader(bodyBytes))
		if reqErr != nil {
			return fmt.Errorf("failed to create request: %w", reqErr)
		}

		// Set headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Execute request
		client := &http.Client{
			Timeout: prs.getHTTPTimeout(isStream),
		}

		var execErr error
		resp, execErr = client.Do(req)
		if execErr != nil {
			return fmt.Errorf("request failed: %w", execErr)
		}

		// Check status code
		if resp.StatusCode >= 500 {
			// 5xx errors are considered failures
			resp.Body.Close()
			return fmt.Errorf("server error: %d", resp.StatusCode)
		}

		return nil
	})

	if cbErr != nil {
		if cbErr == ErrCircuitOpen {
			return nil, fmt.Errorf("circuit breaker is open for provider %s", provider.Name)
		}
		return nil, cbErr
	}

	return resp, nil
}

// getHTTPTimeout returns appropriate timeout based on request type
func (prs *ProviderRelayServiceWithCircuitBreaker) getHTTPTimeout(isStream bool) time.Duration {
	if isStream {
		return 300 * time.Second // 5 minutes for streaming
	}
	return 60 * time.Second // 1 minute for non-streaming
}

// handleProxyRequestWithCircuitBreaker handles a proxy request with circuit breaker support
func (prs *ProviderRelayServiceWithCircuitBreaker) handleProxyRequestWithCircuitBreaker(
	c *gin.Context,
	kind string,
	endpoint string,
) {
	// Read request body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Extract model and stream flag
	requestedModel := extractModel(bodyBytes)
	isStream := extractStreamFlag(bodyBytes)

	// Select provider with circuit breaker
	provider, cb, err := prs.selectProviderWithCircuitBreaker(kind, requestedModel)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": err.Error(),
			"type":  "provider_unavailable",
		})
		return
	}

	fmt.Printf("[Circuit Breaker] Selected provider: %s (ID=%d, State=%s)\n",
		provider.Name, provider.ID, cb.GetState())

	// Build target URL
	targetURL := provider.APIURL + endpoint

	// Prepare headers
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"
	headers["Authorization"] = "Bearer " + provider.APIKey

	// Copy additional headers from original request
	for key, values := range c.Request.Header {
		if len(values) > 0 && !isSystemHeader(key) {
			headers[key] = values[0]
		}
	}

	// Execute request with circuit breaker
	resp, err := prs.executeRequestWithCircuitBreaker(
		provider, cb, targetURL, bodyBytes, headers, isStream,
	)

	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":    err.Error(),
			"provider": provider.Name,
		})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// Stream or buffer response
	c.Status(resp.StatusCode)

	if isStream {
		// Stream response
		c.Stream(func(w io.Writer) bool {
			buf := make([]byte, 1024)
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
			}
			return err == nil
		})
	} else {
		// Buffer response
		io.Copy(c.Writer, resp.Body)
	}
}

// GetCircuitBreakerMetrics returns metrics for all circuit breakers
func (prs *ProviderRelayServiceWithCircuitBreaker) GetCircuitBreakerMetrics() []CircuitBreakerMetrics {
	return prs.circuitBreakerManager.GetAllMetrics()
}

// ResetCircuitBreaker manually resets a circuit breaker
func (prs *ProviderRelayServiceWithCircuitBreaker) ResetCircuitBreaker(providerID int) error {
	return prs.circuitBreakerManager.ResetCircuitBreaker(providerID)
}

// Helper functions

func extractModel(bodyBytes []byte) string {
	// Extract model from JSON body using gjson
	result := gjson.GetBytes(bodyBytes, "model")
	if result.Exists() {
		return result.String()
	}
	return ""
}

func extractStreamFlag(bodyBytes []byte) bool {
	// Extract stream flag from JSON body
	result := gjson.GetBytes(bodyBytes, "stream")
	if result.Exists() {
		return result.Bool()
	}
	return false
}

func isSystemHeader(key string) bool {
	systemHeaders := []string{
		"Host",
		"Content-Length",
		"Transfer-Encoding",
		"Connection",
	}

	for _, h := range systemHeaders {
		if strings.EqualFold(key, h) {
			return true
		}
	}
	return false
}
