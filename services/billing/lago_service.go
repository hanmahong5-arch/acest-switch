package billing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// LagoConfig Lago configuration
type LagoConfig struct {
	APIURL string `json:"api_url"` // e.g., "http://localhost:3001"
	APIKey string `json:"api_key"` // Lago API key
}

// LagoCustomer represents a Lago customer
type LagoCustomer struct {
	LagoID             string            `json:"lago_id,omitempty"`
	ExternalID         string            `json:"external_id"`
	Name               string            `json:"name"`
	Email              string            `json:"email,omitempty"`
	Phone              string            `json:"phone,omitempty"`
	Currency           string            `json:"currency,omitempty"`
	Timezone           string            `json:"timezone,omitempty"`
	BillingAddress     *Address          `json:"billing_configuration,omitempty"`
	Metadata           []MetadataItem    `json:"metadata,omitempty"`
}

// Address represents billing address
type Address struct {
	AddressLine1 string `json:"address_line1,omitempty"`
	AddressLine2 string `json:"address_line2,omitempty"`
	City         string `json:"city,omitempty"`
	State        string `json:"state,omitempty"`
	Zipcode      string `json:"zipcode,omitempty"`
	Country      string `json:"country,omitempty"`
}

// MetadataItem represents metadata key-value pair
type MetadataItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// LagoSubscription represents a Lago subscription
type LagoSubscription struct {
	LagoID             string    `json:"lago_id,omitempty"`
	ExternalID         string    `json:"external_id"`
	ExternalCustomerID string    `json:"external_customer_id"`
	PlanCode           string    `json:"plan_code"`
	Name               string    `json:"name,omitempty"`
	Status             string    `json:"status,omitempty"`
	StartedAt          time.Time `json:"started_at,omitempty"`
	EndingAt           time.Time `json:"ending_at,omitempty"`
	TerminatedAt       time.Time `json:"terminated_at,omitempty"`
}

// LagoWallet represents a Lago wallet (prepaid credits)
type LagoWallet struct {
	LagoID             string  `json:"lago_id,omitempty"`
	ExternalCustomerID string  `json:"external_customer_id"`
	Name               string  `json:"name,omitempty"`
	Currency           string  `json:"currency"`
	RateAmount         string  `json:"rate_amount"`          // Credit rate (e.g., "1.0")
	CreditsBalance     string  `json:"credits_balance"`      // Current balance
	Balance            string  `json:"balance_cents"`        // Balance in cents
	ConsumedCredits    string  `json:"consumed_credits"`
	Status             string  `json:"status,omitempty"`
	ExpirationAt       string  `json:"expiration_at,omitempty"`
}

// LagoWalletTransaction represents a wallet transaction
type LagoWalletTransaction struct {
	LagoID         string `json:"lago_id,omitempty"`
	WalletID       string `json:"wallet_id,omitempty"`
	PaidCredits    string `json:"paid_credits,omitempty"`
	GrantedCredits string `json:"granted_credits,omitempty"`
	Status         string `json:"status,omitempty"`
	TransactionType string `json:"transaction_type,omitempty"`
}

// LagoEvent represents a usage event
type LagoEvent struct {
	TransactionID      string                 `json:"transaction_id"`
	ExternalCustomerID string                 `json:"external_customer_id"`
	Code               string                 `json:"code"`                 // Billable metric code
	Timestamp          int64                  `json:"timestamp,omitempty"`  // Unix timestamp
	Properties         map[string]interface{} `json:"properties,omitempty"` // Event properties
}

// LagoInvoice represents an invoice
type LagoInvoice struct {
	LagoID       string `json:"lago_id"`
	Number       string `json:"number"`
	Status       string `json:"status"`
	PaymentStatus string `json:"payment_status"`
	Currency     string `json:"currency"`
	TotalAmount  int64  `json:"total_amount_cents"`
	TaxAmount    int64  `json:"taxes_amount_cents"`
}

// LagoService handles Lago billing operations
type LagoService struct {
	config     *LagoConfig
	httpClient *http.Client
	mu         sync.RWMutex
}

// NewLagoService creates a new Lago service
func NewLagoService(config *LagoConfig) *LagoService {
	return &LagoService{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request to Lago API
func (s *LagoService) doRequest(method, endpoint string, body interface{}) ([]byte, error) {
	s.mu.RLock()
	apiURL := s.config.APIURL
	apiKey := s.config.APIKey
	s.mu.RUnlock()

	url := fmt.Sprintf("%s/api/v1/%s", apiURL, endpoint)

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ============================================================
// Customer Operations
// ============================================================

// CreateCustomer creates a new customer in Lago
func (s *LagoService) CreateCustomer(customer *LagoCustomer) (*LagoCustomer, error) {
	body := map[string]interface{}{
		"customer": customer,
	}

	respBody, err := s.doRequest("POST", "customers", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Customer *LagoCustomer `json:"customer"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Customer, nil
}

// GetCustomer gets a customer by external ID
func (s *LagoService) GetCustomer(externalID string) (*LagoCustomer, error) {
	respBody, err := s.doRequest("GET", "customers/"+externalID, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Customer *LagoCustomer `json:"customer"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Customer, nil
}

// UpdateCustomer updates an existing customer
func (s *LagoService) UpdateCustomer(customer *LagoCustomer) (*LagoCustomer, error) {
	body := map[string]interface{}{
		"customer": customer,
	}

	respBody, err := s.doRequest("PUT", "customers/"+customer.ExternalID, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Customer *LagoCustomer `json:"customer"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Customer, nil
}

// ============================================================
// Subscription Operations
// ============================================================

// CreateSubscription creates a new subscription
func (s *LagoService) CreateSubscription(sub *LagoSubscription) (*LagoSubscription, error) {
	body := map[string]interface{}{
		"subscription": sub,
	}

	respBody, err := s.doRequest("POST", "subscriptions", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Subscription *LagoSubscription `json:"subscription"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Subscription, nil
}

// GetSubscription gets a subscription by external ID
func (s *LagoService) GetSubscription(externalID string) (*LagoSubscription, error) {
	respBody, err := s.doRequest("GET", "subscriptions/"+externalID, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Subscription *LagoSubscription `json:"subscription"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Subscription, nil
}

// TerminateSubscription terminates a subscription
func (s *LagoService) TerminateSubscription(externalID string) (*LagoSubscription, error) {
	respBody, err := s.doRequest("DELETE", "subscriptions/"+externalID, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Subscription *LagoSubscription `json:"subscription"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Subscription, nil
}

// GetCustomerSubscriptions gets all subscriptions for a customer
func (s *LagoService) GetCustomerSubscriptions(externalCustomerID string) ([]*LagoSubscription, error) {
	endpoint := fmt.Sprintf("subscriptions?external_customer_id=%s", externalCustomerID)
	respBody, err := s.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Subscriptions []*LagoSubscription `json:"subscriptions"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Subscriptions, nil
}

// ============================================================
// Wallet (Prepaid Credits) Operations
// ============================================================

// CreateWallet creates a new wallet for a customer
func (s *LagoService) CreateWallet(wallet *LagoWallet) (*LagoWallet, error) {
	body := map[string]interface{}{
		"wallet": wallet,
	}

	respBody, err := s.doRequest("POST", "wallets", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Wallet *LagoWallet `json:"wallet"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Wallet, nil
}

// GetCustomerWallets gets all wallets for a customer
func (s *LagoService) GetCustomerWallets(externalCustomerID string) ([]*LagoWallet, error) {
	endpoint := fmt.Sprintf("wallets?external_customer_id=%s", externalCustomerID)
	respBody, err := s.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Wallets []*LagoWallet `json:"wallets"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Wallets, nil
}

// TopUpWallet adds credits to a wallet
func (s *LagoService) TopUpWallet(walletID string, paidCredits, grantedCredits string) (*LagoWalletTransaction, error) {
	body := map[string]interface{}{
		"wallet_transaction": map[string]interface{}{
			"wallet_id":       walletID,
			"paid_credits":    paidCredits,
			"granted_credits": grantedCredits,
		},
	}

	respBody, err := s.doRequest("POST", "wallet_transactions", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		WalletTransaction *LagoWalletTransaction `json:"wallet_transaction"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.WalletTransaction, nil
}

// GetWalletBalance gets the current wallet balance for a customer
func (s *LagoService) GetWalletBalance(externalCustomerID string) (int64, error) {
	wallets, err := s.GetCustomerWallets(externalCustomerID)
	if err != nil {
		return 0, err
	}

	if len(wallets) == 0 {
		return 0, nil
	}

	// Parse balance from the first active wallet
	var balance int64
	for _, w := range wallets {
		if w.Status == "active" {
			fmt.Sscanf(w.CreditsBalance, "%d", &balance)
			break
		}
	}

	return balance, nil
}

// ============================================================
// Event (Usage) Operations
// ============================================================

// SendEvent sends a usage event to Lago
func (s *LagoService) SendEvent(event *LagoEvent) error {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	body := map[string]interface{}{
		"event": event,
	}

	_, err := s.doRequest("POST", "events", body)
	return err
}

// SendBatchEvents sends multiple usage events
func (s *LagoService) SendBatchEvents(events []*LagoEvent) error {
	for _, event := range events {
		if event.Timestamp == 0 {
			event.Timestamp = time.Now().Unix()
		}
	}

	body := map[string]interface{}{
		"events": events,
	}

	_, err := s.doRequest("POST", "events/batch", body)
	return err
}

// SendLLMUsage sends LLM token usage event
func (s *LagoService) SendLLMUsage(customerID, traceID string, inputTokens, outputTokens int) error {
	totalTokens := inputTokens + outputTokens

	event := &LagoEvent{
		TransactionID:      traceID,
		ExternalCustomerID: customerID,
		Code:               "llm_tokens",
		Properties: map[string]interface{}{
			"tokens":        totalTokens,
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}

	return s.SendEvent(event)
}

// ============================================================
// Invoice Operations
// ============================================================

// GetCustomerInvoices gets all invoices for a customer
func (s *LagoService) GetCustomerInvoices(externalCustomerID string, limit int) ([]*LagoInvoice, error) {
	endpoint := fmt.Sprintf("invoices?external_customer_id=%s&per_page=%d", externalCustomerID, limit)
	respBody, err := s.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Invoices []*LagoInvoice `json:"invoices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Invoices, nil
}

// ============================================================
// Configuration
// ============================================================

// GetConfig returns the current configuration
func (s *LagoService) GetConfig() *LagoConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates the configuration
func (s *LagoService) UpdateConfig(config *LagoConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// IsEnabled checks if Lago is configured
func (s *LagoService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config != nil && s.config.APIURL != "" && s.config.APIKey != ""
}
