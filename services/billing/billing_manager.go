package billing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// BillingManagerConfig unified billing configuration
type BillingManagerConfig struct {
	Casdoor      *CasdoorConfig      `json:"casdoor"`
	Lago         *LagoConfig         `json:"lago"`
	Payment      *PaymentConfig      `json:"payment"`
	Auth         *AuthConfig         `json:"auth"`
	Billing      *BillingConfig      `json:"billing"`
	Subscription *SubscriptionConfig `json:"subscription"`
}

// BillingManager orchestrates all billing-related services
type BillingManager struct {
	config *BillingManagerConfig

	// Services
	casdoor *CasdoorService
	lago    *LagoService
	payment *PaymentService

	// Middleware
	authMiddleware         *AuthMiddleware
	billingMiddleware      *BillingMiddleware
	subscriptionMiddleware *SubscriptionMiddleware

	// State
	initialized bool
	mu          sync.RWMutex
}

// NewBillingManager creates a new billing manager
func NewBillingManager() *BillingManager {
	return &BillingManager{}
}

// Initialize initializes all billing services with configuration
func (m *BillingManager) Initialize(config *BillingManagerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return fmt.Errorf("billing manager already initialized")
	}

	m.config = config

	// Initialize Casdoor service
	if config.Casdoor != nil && config.Casdoor.Endpoint != "" {
		m.casdoor = NewCasdoorService(config.Casdoor)
	}

	// Initialize Lago service
	if config.Lago != nil && config.Lago.APIURL != "" {
		m.lago = NewLagoService(config.Lago)
	}

	// Initialize Payment service (requires Lago)
	if config.Payment != nil && m.lago != nil {
		var err error
		m.payment, err = NewPaymentService(config.Payment, m.lago)
		if err != nil {
			return fmt.Errorf("failed to initialize payment service: %w", err)
		}
	}

	// Initialize Auth middleware (requires Casdoor)
	if m.casdoor != nil {
		m.authMiddleware = NewAuthMiddleware(m.casdoor, config.Auth)
	}

	// Initialize Billing middleware (requires Lago) - for per-token billing
	if m.lago != nil && config.Billing != nil && config.Billing.Enabled {
		m.billingMiddleware = NewBillingMiddleware(m.lago, config.Billing)
	}

	// Initialize Subscription middleware (requires Lago) - for subscription-only mode
	if m.lago != nil {
		m.subscriptionMiddleware = NewSubscriptionMiddleware(m.lago, config.Subscription)
	}

	m.initialized = true
	return nil
}

// InitializeFromFile loads configuration from file and initializes
func (m *BillingManager) InitializeFromFile(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config BillingManagerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	return m.Initialize(&config)
}

// InitializeFromEnv loads configuration from environment variables
func (m *BillingManager) InitializeFromEnv() error {
	config := &BillingManagerConfig{
		Casdoor: &CasdoorConfig{
			Endpoint:     os.Getenv("CASDOOR_ENDPOINT"),
			ClientID:     os.Getenv("CASDOOR_CLIENT_ID"),
			ClientSecret: os.Getenv("CASDOOR_CLIENT_SECRET"),
			Organization: os.Getenv("CASDOOR_ORGANIZATION"),
			Application:  os.Getenv("CASDOOR_APPLICATION"),
			Certificate:  os.Getenv("CASDOOR_CERTIFICATE"),
		},
		Lago: &LagoConfig{
			APIURL: os.Getenv("LAGO_API_URL"),
			APIKey: os.Getenv("LAGO_API_KEY"),
		},
		Payment: &PaymentConfig{
			AlipayAppID:      os.Getenv("ALIPAY_APP_ID"),
			AlipayPrivateKey: os.Getenv("ALIPAY_PRIVATE_KEY"),
			AlipayPublicKey:  os.Getenv("ALIPAY_PUBLIC_KEY"),
			AlipaySandbox:    os.Getenv("ALIPAY_SANDBOX") == "true",
			WechatAppID:      os.Getenv("WECHAT_APP_ID"),
			WechatMchID:      os.Getenv("WECHAT_MCH_ID"),
			WechatAPIKey:     os.Getenv("WECHAT_API_KEY"),
			WechatAPIKeyV3:   os.Getenv("WECHAT_API_KEY_V3"),
			WechatSerialNo:   os.Getenv("WECHAT_SERIAL_NO"),
			WechatPrivateKey: os.Getenv("WECHAT_PRIVATE_KEY"),
			NotifyURL:        os.Getenv("PAYMENT_NOTIFY_URL"),
			ReturnURL:        os.Getenv("PAYMENT_RETURN_URL"),
		},
		Auth:    DefaultAuthConfig(),
		Billing: DefaultBillingConfig(),
	}

	return m.Initialize(config)
}

// ============================================================
// Service Getters
// ============================================================

// Casdoor returns the Casdoor service
func (m *BillingManager) Casdoor() *CasdoorService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.casdoor
}

// Lago returns the Lago service
func (m *BillingManager) Lago() *LagoService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lago
}

// Payment returns the Payment service
func (m *BillingManager) Payment() *PaymentService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.payment
}

// AuthMiddleware returns the authentication middleware
func (m *BillingManager) AuthMiddleware() *AuthMiddleware {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.authMiddleware
}

// BillingMiddleware returns the billing middleware
func (m *BillingManager) BillingMiddleware() *BillingMiddleware {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.billingMiddleware
}

// SubscriptionMiddleware returns the subscription middleware
func (m *BillingManager) SubscriptionMiddleware() *SubscriptionMiddleware {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.subscriptionMiddleware
}

// ============================================================
// Status Checks
// ============================================================

// IsInitialized checks if manager is initialized
func (m *BillingManager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// IsCasdoorEnabled checks if Casdoor is configured
func (m *BillingManager) IsCasdoorEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.casdoor != nil
}

// IsLagoEnabled checks if Lago is configured
func (m *BillingManager) IsLagoEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lago != nil && m.lago.IsEnabled()
}

// IsPaymentEnabled checks if Payment is configured
func (m *BillingManager) IsPaymentEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.payment != nil
}

// Status returns detailed status of all services
type ServiceStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

func (m *BillingManager) Status() []ServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return []ServiceStatus{
		{
			Name:    "casdoor",
			Enabled: m.casdoor != nil,
			Status:  m.getServiceStatus(m.casdoor != nil),
		},
		{
			Name:    "lago",
			Enabled: m.lago != nil && m.lago.IsEnabled(),
			Status:  m.getServiceStatus(m.lago != nil && m.lago.IsEnabled()),
		},
		{
			Name:    "payment",
			Enabled: m.payment != nil,
			Status:  m.getServiceStatus(m.payment != nil),
		},
		{
			Name:    "auth_middleware",
			Enabled: m.authMiddleware != nil,
			Status:  m.getServiceStatus(m.authMiddleware != nil),
		},
		{
			Name:    "billing_middleware",
			Enabled: m.billingMiddleware != nil,
			Status:  m.getServiceStatus(m.billingMiddleware != nil),
		},
	}
}

func (m *BillingManager) getServiceStatus(enabled bool) string {
	if enabled {
		return "active"
	}
	return "disabled"
}

// ============================================================
// User Operations (Convenience Methods)
// ============================================================

// CreateUser creates a user in both Casdoor (if needed) and Lago
func (m *BillingManager) CreateLagoCustomer(userID, name, email string) error {
	m.mu.RLock()
	lago := m.lago
	m.mu.RUnlock()

	if lago == nil {
		return fmt.Errorf("lago service not initialized")
	}

	customer := &LagoCustomer{
		ExternalID: userID,
		Name:       name,
		Email:      email,
		Currency:   "CNY",
	}

	_, err := lago.CreateCustomer(customer)
	return err
}

// GetUserBalance gets user's credit balance
func (m *BillingManager) GetUserBalance(userID string) (int64, error) {
	m.mu.RLock()
	lago := m.lago
	m.mu.RUnlock()

	if lago == nil {
		return 0, fmt.Errorf("lago service not initialized")
	}

	return lago.GetWalletBalance(userID)
}

// CreateSubscription creates a subscription for user
func (m *BillingManager) CreateSubscription(userID, planCode string) (*LagoSubscription, error) {
	m.mu.RLock()
	lago := m.lago
	m.mu.RUnlock()

	if lago == nil {
		return nil, fmt.Errorf("lago service not initialized")
	}

	sub := &LagoSubscription{
		ExternalID:         fmt.Sprintf("%s-%s", userID, planCode),
		ExternalCustomerID: userID,
		PlanCode:           planCode,
	}

	return lago.CreateSubscription(sub)
}

// GetUserSubscriptions gets user's active subscriptions
func (m *BillingManager) GetUserSubscriptions(userID string) ([]*LagoSubscription, error) {
	m.mu.RLock()
	lago := m.lago
	m.mu.RUnlock()

	if lago == nil {
		return nil, fmt.Errorf("lago service not initialized")
	}

	return lago.GetCustomerSubscriptions(userID)
}

// TopUpCredits adds credits to user's wallet
func (m *BillingManager) TopUpCredits(userID string, amount int64) error {
	m.mu.RLock()
	lago := m.lago
	m.mu.RUnlock()

	if lago == nil {
		return fmt.Errorf("lago service not initialized")
	}

	wallets, err := lago.GetCustomerWallets(userID)
	if err != nil {
		return err
	}

	if len(wallets) == 0 {
		return fmt.Errorf("no wallet found for user")
	}

	// Top up the first active wallet
	for _, w := range wallets {
		if w.Status == "active" {
			_, err = lago.TopUpWallet(w.LagoID, fmt.Sprintf("%d", amount), "0")
			return err
		}
	}

	return fmt.Errorf("no active wallet found")
}

// ReportUsage reports LLM token usage
func (m *BillingManager) ReportUsage(userID, traceID string, inputTokens, outputTokens int) error {
	m.mu.RLock()
	lago := m.lago
	m.mu.RUnlock()

	if lago == nil {
		return fmt.Errorf("lago service not initialized")
	}

	return lago.SendLLMUsage(userID, traceID, inputTokens, outputTokens)
}

// ============================================================
// Configuration Management
// ============================================================

// GetConfig returns current configuration
func (m *BillingManager) GetConfig() *BillingManagerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SaveConfig saves configuration to file
func (m *BillingManager) SaveConfig(configPath string) error {
	m.mu.RLock()
	config := m.config
	m.mu.RUnlock()

	if config == nil {
		return fmt.Errorf("no configuration to save")
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(configPath, data, 0600)
}

// UpdateCasdoorConfig updates Casdoor configuration
func (m *BillingManager) UpdateCasdoorConfig(config *CasdoorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Casdoor = config
	if m.casdoor != nil {
		m.casdoor.UpdateConfig(config)
	} else {
		m.casdoor = NewCasdoorService(config)
		if m.authMiddleware == nil {
			m.authMiddleware = NewAuthMiddleware(m.casdoor, m.config.Auth)
		}
	}
}

// UpdateLagoConfig updates Lago configuration
func (m *BillingManager) UpdateLagoConfig(config *LagoConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Lago = config
	if m.lago != nil {
		m.lago.UpdateConfig(config)
	} else {
		m.lago = NewLagoService(config)
		if m.billingMiddleware == nil {
			m.billingMiddleware = NewBillingMiddleware(m.lago, m.config.Billing)
		}
	}
}

// UpdateBillingConfig updates billing middleware configuration
func (m *BillingManager) UpdateBillingConfig(config *BillingConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Billing = config
	if m.billingMiddleware != nil {
		m.billingMiddleware.UpdateConfig(config)
	}
}

// ============================================================
// Shutdown
// ============================================================

// Shutdown gracefully shuts down all services
func (m *BillingManager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear middleware caches
	if m.authMiddleware != nil {
		m.authMiddleware.ClearCache()
	}
	if m.billingMiddleware != nil {
		m.billingMiddleware.ClearBalanceCache()
	}

	m.initialized = false
	return nil
}

// ============================================================
// Global Instance (Optional Singleton)
// ============================================================

var (
	globalManager     *BillingManager
	globalManagerOnce sync.Once
)

// GetBillingManager returns the global billing manager instance
func GetBillingManager() *BillingManager {
	globalManagerOnce.Do(func() {
		globalManager = NewBillingManager()
	})
	return globalManager
}
