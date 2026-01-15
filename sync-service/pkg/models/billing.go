package models

import (
	"time"
)

// ===== Subscription Types =====

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus string

const (
	SubscriptionActive     SubscriptionStatus = "active"
	SubscriptionPending    SubscriptionStatus = "pending"
	SubscriptionCanceled   SubscriptionStatus = "canceled"
	SubscriptionTerminated SubscriptionStatus = "terminated"
	SubscriptionPastDue    SubscriptionStatus = "past_due"
)

// BillingTime represents when billing occurs
type BillingTime string

const (
	BillingTimeCalendar    BillingTime = "calendar"
	BillingTimeAnniversary BillingTime = "anniversary"
)

// Subscription represents a user subscription
type Subscription struct {
	ID          string             `json:"id"`
	ExternalID  string             `json:"external_id"` // Lago subscription ID
	UserID      string             `json:"user_id"`
	Username    string             `json:"username,omitempty"`
	PlanCode    string             `json:"plan_code"`
	PlanName    string             `json:"plan_name"`
	Status      SubscriptionStatus `json:"status"`
	BillingTime BillingTime        `json:"billing_time"`
	StartedAt   time.Time          `json:"started_at"`
	EndingAt    *time.Time         `json:"ending_at,omitempty"`
	CanceledAt  *time.Time         `json:"canceled_at,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
}

// SubscriptionListResponse represents paginated subscriptions
type SubscriptionListResponse struct {
	Subscriptions []Subscription `json:"subscriptions"`
	Total         int            `json:"total"`
	Page          int            `json:"page"`
	PageSize      int            `json:"page_size"`
}

// PlanInterval represents billing interval
type PlanInterval string

const (
	PlanIntervalMonthly PlanInterval = "monthly"
	PlanIntervalYearly  PlanInterval = "yearly"
	PlanIntervalWeekly  PlanInterval = "weekly"
)

// Plan represents a subscription plan
type Plan struct {
	Code            string       `json:"code"`
	Name            string       `json:"name"`
	Description     string       `json:"description,omitempty"`
	AmountCents     int64        `json:"amount_cents"`
	AmountCurrency  string       `json:"amount_currency"`
	Interval        PlanInterval `json:"interval"`
	TrialPeriodDays int          `json:"trial_period_days,omitempty"`
	PayInAdvance    bool         `json:"pay_in_advance"`
}

// PlanListResponse represents list of plans
type PlanListResponse struct {
	Plans []Plan `json:"plans"`
}

// ===== Wallet/Balance Types =====

// WalletStatus represents wallet status
type WalletStatus string

const (
	WalletActive     WalletStatus = "active"
	WalletTerminated WalletStatus = "terminated"
)

// Wallet represents a user's wallet/balance
type Wallet struct {
	ID                      string       `json:"id"`
	LagoID                  string       `json:"lago_id"` // Lago wallet ID
	UserID                  string       `json:"user_id"`
	Username                string       `json:"username,omitempty"`
	Name                    string       `json:"name"`
	Status                  WalletStatus `json:"status"`
	Currency                string       `json:"currency"`
	BalanceCents            int64        `json:"balance_cents"`
	ConsumedCredits         float64      `json:"consumed_credits"`
	OngoingBalanceCents     int64        `json:"ongoing_balance_cents"`
	CreditsBalance          float64      `json:"credits_balance"`
	OngoingUsageBalanceCents int64       `json:"ongoing_usage_balance_cents"`
	RateAmount              string       `json:"rate_amount"`
	CreatedAt               time.Time    `json:"created_at"`
	LastTransactionAt       *time.Time   `json:"last_transaction_at,omitempty"`
}

// WalletListResponse represents paginated wallets
type WalletListResponse struct {
	Wallets  []Wallet `json:"wallets"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// WalletTransactionType represents transaction direction
type WalletTransactionType string

const (
	WalletTransactionInbound  WalletTransactionType = "inbound"
	WalletTransactionOutbound WalletTransactionType = "outbound"
)

// WalletTransactionKind represents the kind of transaction
type WalletTransactionKind string

const (
	WalletTransactionPaidCredits WalletTransactionKind = "paid_credits"
	WalletTransactionGranted     WalletTransactionKind = "granted"
	WalletTransactionUsage       WalletTransactionKind = "usage"
	WalletTransactionRefund      WalletTransactionKind = "refund"
)

// WalletTransactionStatus represents transaction status
type WalletTransactionStatus string

const (
	WalletTransactionPending WalletTransactionStatus = "pending"
	WalletTransactionSettled WalletTransactionStatus = "settled"
)

// WalletTransaction represents a wallet transaction
type WalletTransaction struct {
	ID              string                  `json:"id"`
	WalletID        string                  `json:"wallet_id"`
	UserID          string                  `json:"user_id"`
	Type            WalletTransactionType   `json:"type"`
	TransactionType WalletTransactionKind   `json:"transaction_type"`
	Status          WalletTransactionStatus `json:"status"`
	Amount          string                  `json:"amount"`
	CreditAmount    string                  `json:"credit_amount"`
	SettledAt       *time.Time              `json:"settled_at,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
}

// WalletTransactionListResponse represents paginated wallet transactions
type WalletTransactionListResponse struct {
	Transactions []WalletTransaction `json:"transactions"`
	Total        int                 `json:"total"`
}

// ===== Payment Types =====

// PaymentMethod represents payment method
type PaymentMethod string

const (
	PaymentMethodAlipay PaymentMethod = "alipay"
	PaymentMethodWechat PaymentMethod = "wechat"
	PaymentMethodStripe PaymentMethod = "stripe"
	PaymentMethodManual PaymentMethod = "manual"
)

// PaymentStatus represents payment status
type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "pending"
	PaymentPaid     PaymentStatus = "paid"
	PaymentFailed   PaymentStatus = "failed"
	PaymentRefunded PaymentStatus = "refunded"
	PaymentCanceled PaymentStatus = "canceled"
)

// Payment represents a payment record
type Payment struct {
	ID             string        `json:"id"`
	OrderNo        string        `json:"order_no"`
	UserID         string        `json:"user_id"`
	Username       string        `json:"username,omitempty"`
	AmountCents    int64         `json:"amount_cents"`
	Currency       string        `json:"currency"`
	Method         PaymentMethod `json:"method"`
	Status         PaymentStatus `json:"status"`
	Description    string        `json:"description,omitempty"`
	WalletID       string        `json:"wallet_id,omitempty"`
	SubscriptionID string        `json:"subscription_id,omitempty"`
	PayURL         string        `json:"pay_url,omitempty"`
	PaidAt         *time.Time    `json:"paid_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// PaymentListResponse represents paginated payments
type PaymentListResponse struct {
	Payments []Payment `json:"payments"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// ===== Configuration Types =====

// BillingConfig represents billing configuration
type BillingConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Casdoor
	CasdoorEndpoint     string `json:"casdoor_endpoint" yaml:"casdoor_endpoint"`
	CasdoorClientID     string `json:"casdoor_client_id" yaml:"casdoor_client_id"`
	CasdoorClientSecret string `json:"casdoor_client_secret" yaml:"casdoor_client_secret"`
	CasdoorOrganization string `json:"casdoor_organization" yaml:"casdoor_organization"`
	CasdoorApplication  string `json:"casdoor_application" yaml:"casdoor_application"`
	CasdoorCertificate  string `json:"casdoor_certificate" yaml:"casdoor_certificate"`

	// Lago
	LagoAPIURL string `json:"lago_api_url" yaml:"lago_api_url"`
	LagoAPIKey string `json:"lago_api_key" yaml:"lago_api_key"`

	// Alipay
	AlipayAppID      string `json:"alipay_app_id" yaml:"alipay_app_id"`
	AlipayPrivateKey string `json:"alipay_private_key" yaml:"alipay_private_key"`
	AlipayPublicKey  string `json:"alipay_public_key" yaml:"alipay_public_key"`
	AlipaySandbox    bool   `json:"alipay_sandbox" yaml:"alipay_sandbox"`

	// WeChat Pay
	WechatAppID      string `json:"wechat_app_id" yaml:"wechat_app_id"`
	WechatMchID      string `json:"wechat_mch_id" yaml:"wechat_mch_id"`
	WechatAPIKey     string `json:"wechat_api_key" yaml:"wechat_api_key"`
	WechatAPIKeyV3   string `json:"wechat_api_key_v3" yaml:"wechat_api_key_v3"`
	WechatSerialNo   string `json:"wechat_serial_no" yaml:"wechat_serial_no"`
	WechatPrivateKey string `json:"wechat_private_key" yaml:"wechat_private_key"`

	// Callback URLs
	PaymentNotifyURL string `json:"payment_notify_url" yaml:"payment_notify_url"`
	PaymentReturnURL string `json:"payment_return_url" yaml:"payment_return_url"`

	// Subscription settings
	GracePeriodHours    int  `json:"grace_period_hours" yaml:"grace_period_hours"`
	RequireSubscription bool `json:"require_subscription" yaml:"require_subscription"`
}

// BillingServiceStatus represents a billing service status
type BillingServiceStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

// BillingStatus represents overall billing status
type BillingStatus struct {
	Enabled  bool                   `json:"enabled"`
	Services []BillingServiceStatus `json:"services"`
}

// ===== User Quick Query Types =====

// UserSubscriptionStatus represents user's subscription status
type UserSubscriptionStatus struct {
	HasActiveSubscription bool          `json:"has_active_subscription"`
	Subscription          *Subscription `json:"subscription,omitempty"`
	InGracePeriod         bool          `json:"in_grace_period"`
	ExpiresAt             *time.Time    `json:"expires_at,omitempty"`
}

// UserBalance represents user's balance summary
type UserBalance struct {
	BalanceCents int64    `json:"balance_cents"`
	Currency     string   `json:"currency"`
	Wallets      []Wallet `json:"wallets"`
}

// ===== Request/Response Types =====

// CreateSubscriptionRequest represents subscription creation request
type CreateSubscriptionRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	PlanCode string `json:"plan_code" binding:"required"`
}

// CreateWalletRequest represents wallet creation request
type CreateWalletRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	Name       string `json:"name,omitempty"`
	Currency   string `json:"currency,omitempty"`
	RateAmount string `json:"rate_amount,omitempty"`
}

// TopUpWalletRequest represents wallet top-up request
type TopUpWalletRequest struct {
	PaidCredits    string `json:"paid_credits" binding:"required"`
	GrantedCredits string `json:"granted_credits,omitempty"`
}

// CreatePaymentRequest represents payment creation request
type CreatePaymentRequest struct {
	UserID      string        `json:"user_id" binding:"required"`
	AmountCents int64         `json:"amount_cents" binding:"required,min=1"`
	Method      PaymentMethod `json:"method" binding:"required,oneof=alipay wechat"`
	Description string        `json:"description,omitempty"`
	WalletID    string        `json:"wallet_id,omitempty"`
}

// RefundPaymentRequest represents refund request
type RefundPaymentRequest struct {
	AmountCents int64  `json:"amount_cents,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// ConfirmPaymentRequest represents manual payment confirmation
type ConfirmPaymentRequest struct {
	Note string `json:"note,omitempty"`
}

// TestConnectionResponse represents connection test result
type TestConnectionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// MessageResponse represents a simple message response
type MessageResponse struct {
	Message string `json:"message"`
}
