package admin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/pkg/models"
	"github.com/google/uuid"
)

// BillingService handles billing operations
type BillingService struct {
	mu sync.RWMutex

	// Configuration
	config     models.BillingConfig
	configPath string

	// In-memory storage (for demo; production should use database)
	subscriptions map[string]*models.Subscription
	wallets       map[string]*models.Wallet
	transactions  map[string][]models.WalletTransaction // walletID -> transactions
	payments      map[string]*models.Payment
	plans         map[string]*models.Plan

	// Indices
	subscriptionsByUser map[string][]string // userID -> []subscriptionID
	walletsByUser       map[string][]string // userID -> []walletID
	paymentsByUser      map[string][]string // userID -> []paymentID

	// External service clients
	lagoClient    *LagoClient
	casdoorClient *CasdoorClient
	alipayClient  *AlipayClient
	wechatClient  *WechatClient
}

// NewBillingService creates a new billing service
func NewBillingService(configPath string) *BillingService {
	s := &BillingService{
		configPath:          configPath,
		subscriptions:       make(map[string]*models.Subscription),
		wallets:             make(map[string]*models.Wallet),
		transactions:        make(map[string][]models.WalletTransaction),
		payments:            make(map[string]*models.Payment),
		plans:               make(map[string]*models.Plan),
		subscriptionsByUser: make(map[string][]string),
		walletsByUser:       make(map[string][]string),
		paymentsByUser:      make(map[string][]string),
	}

	// Load configuration
	s.loadConfig()

	// Initialize default plans
	s.initDefaultPlans()

	// Initialize external clients if config is valid
	s.initClients()

	return s
}

// loadConfig loads billing configuration from file
func (s *BillingService) loadConfig() {
	if s.configPath == "" {
		homeDir, _ := os.UserHomeDir()
		s.configPath = filepath.Join(homeDir, ".code-switch", "billing-config.json")
	}

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		// Use default config
		s.config = models.BillingConfig{
			Enabled:             false,
			GracePeriodHours:    24,
			RequireSubscription: true,
		}
		return
	}

	if err := json.Unmarshal(data, &s.config); err != nil {
		s.config = models.BillingConfig{
			Enabled:             false,
			GracePeriodHours:    24,
			RequireSubscription: true,
		}
	}
}

// saveConfig saves billing configuration to file
func (s *BillingService) saveConfig() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.configPath, data, 0600)
}

// initDefaultPlans initializes default subscription plans
func (s *BillingService) initDefaultPlans() {
	s.plans["monthly_basic"] = &models.Plan{
		Code:           "monthly_basic",
		Name:           "Basic Monthly",
		Description:    "Basic plan with monthly billing",
		AmountCents:    2900,
		AmountCurrency: "CNY",
		Interval:       models.PlanIntervalMonthly,
		PayInAdvance:   true,
	}
	s.plans["yearly_basic"] = &models.Plan{
		Code:           "yearly_basic",
		Name:           "Basic Yearly",
		Description:    "Basic plan with yearly billing (2 months free)",
		AmountCents:    29900,
		AmountCurrency: "CNY",
		Interval:       models.PlanIntervalYearly,
		PayInAdvance:   true,
	}
	s.plans["monthly_pro"] = &models.Plan{
		Code:           "monthly_pro",
		Name:           "Pro Monthly",
		Description:    "Pro plan with monthly billing",
		AmountCents:    9900,
		AmountCurrency: "CNY",
		Interval:       models.PlanIntervalMonthly,
		PayInAdvance:   true,
	}
	s.plans["yearly_pro"] = &models.Plan{
		Code:           "yearly_pro",
		Name:           "Pro Yearly",
		Description:    "Pro plan with yearly billing (2 months free)",
		AmountCents:    99900,
		AmountCurrency: "CNY",
		Interval:       models.PlanIntervalYearly,
		PayInAdvance:   true,
	}
}

// initClients initializes external service clients
func (s *BillingService) initClients() {
	if s.config.LagoAPIURL != "" && s.config.LagoAPIKey != "" {
		s.lagoClient = NewLagoClient(s.config.LagoAPIURL, s.config.LagoAPIKey)
	}
	if s.config.CasdoorEndpoint != "" && s.config.CasdoorClientID != "" {
		s.casdoorClient = NewCasdoorClient(
			s.config.CasdoorEndpoint,
			s.config.CasdoorClientID,
			s.config.CasdoorClientSecret,
			s.config.CasdoorOrganization,
			s.config.CasdoorApplication,
			s.config.CasdoorCertificate,
		)
	}
	if s.config.AlipayAppID != "" && s.config.AlipayPrivateKey != "" {
		s.alipayClient = NewAlipayClient(
			s.config.AlipayAppID,
			s.config.AlipayPrivateKey,
			s.config.AlipayPublicKey,
			s.config.AlipaySandbox,
			s.config.PaymentNotifyURL,
			s.config.PaymentReturnURL,
		)
	}
	if s.config.WechatAppID != "" && s.config.WechatMchID != "" {
		s.wechatClient = NewWechatClient(
			s.config.WechatAppID,
			s.config.WechatMchID,
			s.config.WechatAPIKey,
			s.config.WechatAPIKeyV3,
			s.config.WechatSerialNo,
			s.config.WechatPrivateKey,
			s.config.PaymentNotifyURL,
		)
	}
}

// ===== Configuration =====

// GetConfig returns the current billing configuration
func (s *BillingService) GetConfig() models.BillingConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates billing configuration
func (s *BillingService) UpdateConfig(config models.BillingConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
	if err := s.saveConfig(); err != nil {
		return err
	}

	// Reinitialize clients with new config
	s.initClients()
	return nil
}

// GetStatus returns billing system status
func (s *BillingService) GetStatus() models.BillingStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := []models.BillingServiceStatus{
		{
			Name:    "Casdoor",
			Enabled: s.casdoorClient != nil,
			Status:  s.getCasdoorStatus(),
		},
		{
			Name:    "Lago",
			Enabled: s.lagoClient != nil,
			Status:  s.getLagoStatus(),
		},
		{
			Name:    "Alipay",
			Enabled: s.alipayClient != nil,
			Status:  s.getAlipayStatus(),
		},
		{
			Name:    "WeChat Pay",
			Enabled: s.wechatClient != nil,
			Status:  s.getWechatStatus(),
		},
	}

	return models.BillingStatus{
		Enabled:  s.config.Enabled,
		Services: services,
	}
}

func (s *BillingService) getCasdoorStatus() string {
	if s.casdoorClient == nil {
		return "not configured"
	}
	return "configured"
}

func (s *BillingService) getLagoStatus() string {
	if s.lagoClient == nil {
		return "not configured"
	}
	return "configured"
}

func (s *BillingService) getAlipayStatus() string {
	if s.alipayClient == nil {
		return "not configured"
	}
	return "configured"
}

func (s *BillingService) getWechatStatus() string {
	if s.wechatClient == nil {
		return "not configured"
	}
	return "configured"
}

// ===== Connection Tests =====

// TestCasdoorConnection tests Casdoor connection
func (s *BillingService) TestCasdoorConnection() models.TestConnectionResponse {
	s.mu.RLock()
	client := s.casdoorClient
	s.mu.RUnlock()

	if client == nil {
		return models.TestConnectionResponse{
			Success: false,
			Message: "Casdoor not configured",
		}
	}
	return client.TestConnection()
}

// TestLagoConnection tests Lago connection
func (s *BillingService) TestLagoConnection() models.TestConnectionResponse {
	s.mu.RLock()
	client := s.lagoClient
	s.mu.RUnlock()

	if client == nil {
		return models.TestConnectionResponse{
			Success: false,
			Message: "Lago not configured",
		}
	}
	return client.TestConnection()
}

// TestPaymentConnection tests payment provider connection
func (s *BillingService) TestPaymentConnection(method string) models.TestConnectionResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch method {
	case "alipay":
		if s.alipayClient == nil {
			return models.TestConnectionResponse{
				Success: false,
				Message: "Alipay not configured",
			}
		}
		return s.alipayClient.TestConnection()
	case "wechat":
		if s.wechatClient == nil {
			return models.TestConnectionResponse{
				Success: false,
				Message: "WeChat Pay not configured",
			}
		}
		return s.wechatClient.TestConnection()
	default:
		return models.TestConnectionResponse{
			Success: false,
			Message: "Unknown payment method",
		}
	}
}

// ===== Plans =====

// ListPlans returns all available plans
func (s *BillingService) ListPlans() models.PlanListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plans := make([]models.Plan, 0, len(s.plans))
	for _, plan := range s.plans {
		plans = append(plans, *plan)
	}
	return models.PlanListResponse{Plans: plans}
}

// ===== Subscriptions =====

// ListSubscriptions returns paginated subscriptions
func (s *BillingService) ListSubscriptions(page, pageSize int, userID, status, planCode string) models.SubscriptionListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*models.Subscription
	for _, sub := range s.subscriptions {
		if userID != "" && sub.UserID != userID {
			continue
		}
		if status != "" && string(sub.Status) != status {
			continue
		}
		if planCode != "" && sub.PlanCode != planCode {
			continue
		}
		filtered = append(filtered, sub)
	}

	// Sort by created_at descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)

	// Pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(filtered) {
		return models.SubscriptionListResponse{
			Subscriptions: []models.Subscription{},
			Total:         total,
			Page:          page,
			PageSize:      pageSize,
		}
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	result := make([]models.Subscription, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, *filtered[i])
	}

	return models.SubscriptionListResponse{
		Subscriptions: result,
		Total:         total,
		Page:          page,
		PageSize:      pageSize,
	}
}

// GetSubscription returns a subscription by ID
func (s *BillingService) GetSubscription(id string) (*models.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subscriptions[id]
	if !ok {
		return nil, fmt.Errorf("subscription not found")
	}
	return sub, nil
}

// CreateSubscription creates a new subscription
func (s *BillingService) CreateSubscription(req models.CreateSubscriptionRequest) (*models.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate plan
	plan, ok := s.plans[req.PlanCode]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", req.PlanCode)
	}

	now := time.Now()
	sub := &models.Subscription{
		ID:          uuid.New().String(),
		ExternalID:  "", // Set by Lago if integrated
		UserID:      req.UserID,
		PlanCode:    req.PlanCode,
		PlanName:    plan.Name,
		Status:      models.SubscriptionActive,
		BillingTime: models.BillingTimeAnniversary,
		StartedAt:   now,
		CreatedAt:   now,
	}

	// Calculate ending date based on interval
	switch plan.Interval {
	case models.PlanIntervalMonthly:
		endDate := now.AddDate(0, 1, 0)
		sub.EndingAt = &endDate
	case models.PlanIntervalYearly:
		endDate := now.AddDate(1, 0, 0)
		sub.EndingAt = &endDate
	case models.PlanIntervalWeekly:
		endDate := now.AddDate(0, 0, 7)
		sub.EndingAt = &endDate
	}

	// If Lago is configured, create subscription there
	if s.lagoClient != nil {
		externalID, err := s.lagoClient.CreateSubscription(req.UserID, req.PlanCode)
		if err != nil {
			return nil, fmt.Errorf("failed to create subscription in Lago: %w", err)
		}
		sub.ExternalID = externalID
	}

	s.subscriptions[sub.ID] = sub
	s.subscriptionsByUser[req.UserID] = append(s.subscriptionsByUser[req.UserID], sub.ID)

	return sub, nil
}

// CancelSubscription cancels a subscription
func (s *BillingService) CancelSubscription(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[id]
	if !ok {
		return fmt.Errorf("subscription not found")
	}

	if sub.Status != models.SubscriptionActive {
		return fmt.Errorf("subscription is not active")
	}

	// If Lago is configured, cancel there
	if s.lagoClient != nil && sub.ExternalID != "" {
		if err := s.lagoClient.CancelSubscription(sub.ExternalID); err != nil {
			return fmt.Errorf("failed to cancel subscription in Lago: %w", err)
		}
	}

	now := time.Now()
	sub.Status = models.SubscriptionCanceled
	sub.CanceledAt = &now

	return nil
}

// ReactivateSubscription reactivates a canceled subscription
func (s *BillingService) ReactivateSubscription(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[id]
	if !ok {
		return fmt.Errorf("subscription not found")
	}

	if sub.Status != models.SubscriptionCanceled {
		return fmt.Errorf("subscription is not canceled")
	}

	sub.Status = models.SubscriptionActive
	sub.CanceledAt = nil

	return nil
}

// ===== Wallets =====

// ListWallets returns paginated wallets
func (s *BillingService) ListWallets(page, pageSize int, userID, status string) models.WalletListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*models.Wallet
	for _, wallet := range s.wallets {
		if userID != "" && wallet.UserID != userID {
			continue
		}
		if status != "" && string(wallet.Status) != status {
			continue
		}
		filtered = append(filtered, wallet)
	}

	// Sort by created_at descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)

	// Pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(filtered) {
		return models.WalletListResponse{
			Wallets:  []models.Wallet{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		}
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	result := make([]models.Wallet, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, *filtered[i])
	}

	return models.WalletListResponse{
		Wallets:  result,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

// GetWallet returns a wallet by ID
func (s *BillingService) GetWallet(id string) (*models.Wallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wallet, ok := s.wallets[id]
	if !ok {
		return nil, fmt.Errorf("wallet not found")
	}
	return wallet, nil
}

// CreateWallet creates a new wallet
func (s *BillingService) CreateWallet(req models.CreateWalletRequest) (*models.Wallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}

	name := req.Name
	if name == "" {
		name = "Default Wallet"
	}

	rateAmount := req.RateAmount
	if rateAmount == "" {
		rateAmount = "1.0"
	}

	wallet := &models.Wallet{
		ID:                       uuid.New().String(),
		LagoID:                   "",
		UserID:                   req.UserID,
		Name:                     name,
		Status:                   models.WalletActive,
		Currency:                 currency,
		BalanceCents:             0,
		ConsumedCredits:          0,
		OngoingBalanceCents:      0,
		CreditsBalance:           0,
		OngoingUsageBalanceCents: 0,
		RateAmount:               rateAmount,
		CreatedAt:                time.Now(),
	}

	// If Lago is configured, create wallet there
	if s.lagoClient != nil {
		lagoID, err := s.lagoClient.CreateWallet(req.UserID, name, currency, rateAmount)
		if err != nil {
			return nil, fmt.Errorf("failed to create wallet in Lago: %w", err)
		}
		wallet.LagoID = lagoID
	}

	s.wallets[wallet.ID] = wallet
	s.walletsByUser[req.UserID] = append(s.walletsByUser[req.UserID], wallet.ID)

	return wallet, nil
}

// TopUpWallet tops up a wallet
func (s *BillingService) TopUpWallet(walletID string, req models.TopUpWalletRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, ok := s.wallets[walletID]
	if !ok {
		return fmt.Errorf("wallet not found")
	}

	// If Lago is configured, top up there
	if s.lagoClient != nil && wallet.LagoID != "" {
		if err := s.lagoClient.TopUpWallet(wallet.LagoID, req.PaidCredits, req.GrantedCredits); err != nil {
			return fmt.Errorf("failed to top up wallet in Lago: %w", err)
		}
	}

	// Record transaction
	now := time.Now()
	tx := models.WalletTransaction{
		ID:              uuid.New().String(),
		WalletID:        walletID,
		UserID:          wallet.UserID,
		Type:            models.WalletTransactionInbound,
		TransactionType: models.WalletTransactionPaidCredits,
		Status:          models.WalletTransactionSettled,
		Amount:          req.PaidCredits,
		CreditAmount:    req.PaidCredits,
		SettledAt:       &now,
		CreatedAt:       now,
	}
	s.transactions[walletID] = append(s.transactions[walletID], tx)

	if req.GrantedCredits != "" {
		grantTx := models.WalletTransaction{
			ID:              uuid.New().String(),
			WalletID:        walletID,
			UserID:          wallet.UserID,
			Type:            models.WalletTransactionInbound,
			TransactionType: models.WalletTransactionGranted,
			Status:          models.WalletTransactionSettled,
			Amount:          req.GrantedCredits,
			CreditAmount:    req.GrantedCredits,
			SettledAt:       &now,
			CreatedAt:       now,
		}
		s.transactions[walletID] = append(s.transactions[walletID], grantTx)
	}

	wallet.LastTransactionAt = &now

	return nil
}

// GetWalletTransactions returns wallet transactions
func (s *BillingService) GetWalletTransactions(walletID string, page, pageSize int) models.WalletTransactionListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	txs, ok := s.transactions[walletID]
	if !ok {
		return models.WalletTransactionListResponse{
			Transactions: []models.WalletTransaction{},
			Total:        0,
		}
	}

	// Sort by created_at descending
	sorted := make([]models.WalletTransaction, len(txs))
	copy(sorted, txs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	total := len(sorted)

	// Pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(sorted) {
		return models.WalletTransactionListResponse{
			Transactions: []models.WalletTransaction{},
			Total:        total,
		}
	}
	if end > len(sorted) {
		end = len(sorted)
	}

	return models.WalletTransactionListResponse{
		Transactions: sorted[start:end],
		Total:        total,
	}
}

// ===== Payments =====

// ListPayments returns paginated payments
func (s *BillingService) ListPayments(page, pageSize int, userID, status, method string, startTime, endTime *time.Time) models.PaymentListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*models.Payment
	for _, payment := range s.payments {
		if userID != "" && payment.UserID != userID {
			continue
		}
		if status != "" && string(payment.Status) != status {
			continue
		}
		if method != "" && string(payment.Method) != method {
			continue
		}
		if startTime != nil && payment.CreatedAt.Before(*startTime) {
			continue
		}
		if endTime != nil && payment.CreatedAt.After(*endTime) {
			continue
		}
		filtered = append(filtered, payment)
	}

	// Sort by created_at descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)

	// Pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(filtered) {
		return models.PaymentListResponse{
			Payments: []models.Payment{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		}
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	result := make([]models.Payment, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, *filtered[i])
	}

	return models.PaymentListResponse{
		Payments: result,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

// GetPayment returns a payment by ID
func (s *BillingService) GetPayment(id string) (*models.Payment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	payment, ok := s.payments[id]
	if !ok {
		return nil, fmt.Errorf("payment not found")
	}
	return payment, nil
}

// CreatePayment creates a new payment
func (s *BillingService) CreatePayment(req models.CreatePaymentRequest) (*models.Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	payment := &models.Payment{
		ID:          uuid.New().String(),
		OrderNo:     fmt.Sprintf("PAY%s%d", now.Format("20060102150405"), now.UnixNano()%10000),
		UserID:      req.UserID,
		AmountCents: req.AmountCents,
		Currency:    "CNY",
		Method:      req.Method,
		Status:      models.PaymentPending,
		Description: req.Description,
		WalletID:    req.WalletID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create payment with provider
	var payURL string
	var err error

	switch req.Method {
	case models.PaymentMethodAlipay:
		if s.alipayClient == nil {
			return nil, fmt.Errorf("Alipay not configured")
		}
		payURL, err = s.alipayClient.CreatePayment(payment.OrderNo, req.AmountCents, req.Description)
	case models.PaymentMethodWechat:
		if s.wechatClient == nil {
			return nil, fmt.Errorf("WeChat Pay not configured")
		}
		payURL, err = s.wechatClient.CreatePayment(payment.OrderNo, req.AmountCents, req.Description)
	default:
		return nil, fmt.Errorf("unsupported payment method: %s", req.Method)
	}

	if err != nil {
		return nil, err
	}

	payment.PayURL = payURL
	s.payments[payment.ID] = payment
	s.paymentsByUser[req.UserID] = append(s.paymentsByUser[req.UserID], payment.ID)

	return payment, nil
}

// RefundPayment refunds a payment
func (s *BillingService) RefundPayment(id string, req models.RefundPaymentRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	payment, ok := s.payments[id]
	if !ok {
		return fmt.Errorf("payment not found")
	}

	if payment.Status != models.PaymentPaid {
		return fmt.Errorf("payment is not paid")
	}

	refundAmount := req.AmountCents
	if refundAmount == 0 {
		refundAmount = payment.AmountCents
	}

	// Process refund with provider
	switch payment.Method {
	case models.PaymentMethodAlipay:
		if s.alipayClient != nil {
			if err := s.alipayClient.Refund(payment.OrderNo, refundAmount); err != nil {
				return fmt.Errorf("failed to refund via Alipay: %w", err)
			}
		}
	case models.PaymentMethodWechat:
		if s.wechatClient != nil {
			if err := s.wechatClient.Refund(payment.OrderNo, refundAmount); err != nil {
				return fmt.Errorf("failed to refund via WeChat Pay: %w", err)
			}
		}
	}

	payment.Status = models.PaymentRefunded
	payment.UpdatedAt = time.Now()

	return nil
}

// ConfirmPayment manually confirms a payment
func (s *BillingService) ConfirmPayment(id string, req models.ConfirmPaymentRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	payment, ok := s.payments[id]
	if !ok {
		return fmt.Errorf("payment not found")
	}

	if payment.Status != models.PaymentPending {
		return fmt.Errorf("payment is not pending")
	}

	now := time.Now()
	payment.Status = models.PaymentPaid
	payment.PaidAt = &now
	payment.UpdatedAt = now

	return nil
}

// ===== User Quick Queries =====

// GetUserSubscriptionStatus returns user's subscription status
func (s *BillingService) GetUserSubscriptionStatus(userID string) models.UserSubscriptionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subIDs := s.subscriptionsByUser[userID]
	var activeSub *models.Subscription
	var inGracePeriod bool

	now := time.Now()
	gracePeriod := time.Duration(s.config.GracePeriodHours) * time.Hour

	for _, subID := range subIDs {
		sub := s.subscriptions[subID]
		if sub == nil {
			continue
		}

		if sub.Status == models.SubscriptionActive {
			activeSub = sub
			break
		}

		// Check grace period for canceled subscriptions
		if sub.Status == models.SubscriptionCanceled && sub.EndingAt != nil {
			if now.Before(sub.EndingAt.Add(gracePeriod)) {
				activeSub = sub
				inGracePeriod = true
				break
			}
		}
	}

	result := models.UserSubscriptionStatus{
		HasActiveSubscription: activeSub != nil,
		InGracePeriod:         inGracePeriod,
	}

	if activeSub != nil {
		result.Subscription = activeSub
		result.ExpiresAt = activeSub.EndingAt
	}

	return result
}

// GetUserBalance returns user's balance summary
func (s *BillingService) GetUserBalance(userID string) models.UserBalance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	walletIDs := s.walletsByUser[userID]
	var totalBalance int64
	var currency string
	var wallets []models.Wallet

	for _, walletID := range walletIDs {
		wallet := s.wallets[walletID]
		if wallet == nil || wallet.Status != models.WalletActive {
			continue
		}
		totalBalance += wallet.BalanceCents
		currency = wallet.Currency
		wallets = append(wallets, *wallet)
	}

	if currency == "" {
		currency = "CNY"
	}

	return models.UserBalance{
		BalanceCents: totalBalance,
		Currency:     currency,
		Wallets:      wallets,
	}
}
