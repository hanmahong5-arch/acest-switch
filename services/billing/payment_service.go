package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/wechat/v3"
)

// PaymentConfig payment gateway configuration
type PaymentConfig struct {
	// Alipay
	AlipayAppID      string `json:"alipay_app_id"`
	AlipayPrivateKey string `json:"alipay_private_key"`
	AlipayPublicKey  string `json:"alipay_public_key"`
	AlipaySandbox    bool   `json:"alipay_sandbox"`

	// WeChat Pay
	WechatAppID     string `json:"wechat_app_id"`
	WechatMchID     string `json:"wechat_mch_id"`
	WechatAPIKey    string `json:"wechat_api_key"`
	WechatAPIKeyV3  string `json:"wechat_api_key_v3"`
	WechatSerialNo  string `json:"wechat_serial_no"`
	WechatPrivateKey string `json:"wechat_private_key"`

	// Callback URLs
	NotifyURL string `json:"notify_url"` // Payment callback URL
	ReturnURL string `json:"return_url"` // Redirect URL after payment
}

// PaymentMethod represents available payment methods
type PaymentMethod string

const (
	PaymentMethodAlipay PaymentMethod = "alipay"
	PaymentMethodWechat PaymentMethod = "wechat"
)

// PaymentStatus represents payment status
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusPaid      PaymentStatus = "paid"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
	PaymentStatusCancelled PaymentStatus = "cancelled"
)

// PaymentOrder represents a payment order
type PaymentOrder struct {
	OrderID       string        `json:"order_id"`
	UserID        string        `json:"user_id"`
	Amount        int64         `json:"amount"`        // Amount in cents (分)
	Currency      string        `json:"currency"`      // CNY
	Credits       int64         `json:"credits"`       // Credits to add
	Method        PaymentMethod `json:"method"`
	Status        PaymentStatus `json:"status"`
	Description   string        `json:"description"`
	TradeNo       string        `json:"trade_no,omitempty"`       // Third-party transaction ID
	PayURL        string        `json:"pay_url,omitempty"`        // Payment URL (for web)
	QRCode        string        `json:"qr_code,omitempty"`        // QR code content
	PrepayID      string        `json:"prepay_id,omitempty"`      // WeChat prepay ID
	CreatedAt     time.Time     `json:"created_at"`
	PaidAt        *time.Time    `json:"paid_at,omitempty"`
	ExpiredAt     time.Time     `json:"expired_at"`
}

// PaymentCallback represents a payment callback
type PaymentCallback struct {
	Method    PaymentMethod `json:"method"`
	OrderID   string        `json:"order_id"`
	TradeNo   string        `json:"trade_no"`
	Amount    int64         `json:"amount"`
	Status    PaymentStatus `json:"status"`
	RawData   string        `json:"raw_data,omitempty"`
}

// PaymentService handles payment operations
type PaymentService struct {
	config       *PaymentConfig
	alipayClient *alipay.Client
	wechatClient *wechat.ClientV3
	lagoService  *LagoService
	mu           sync.RWMutex

	// Order storage (in production, use database)
	orders sync.Map // map[orderID]*PaymentOrder
}

// NewPaymentService creates a new payment service
func NewPaymentService(config *PaymentConfig, lagoService *LagoService) (*PaymentService, error) {
	s := &PaymentService{
		config:      config,
		lagoService: lagoService,
	}

	// Initialize Alipay client
	if config.AlipayAppID != "" && config.AlipayPrivateKey != "" {
		client, err := alipay.NewClient(config.AlipayAppID, config.AlipayPrivateKey, !config.AlipaySandbox)
		if err != nil {
			return nil, fmt.Errorf("failed to create alipay client: %w", err)
		}

		// Set public key for verification (use certificate mode in production)
		if config.AlipayPublicKey != "" {
			client.SetAliPayPublicCertSN(config.AlipayPublicKey)
		}

		s.alipayClient = client
	}

	// Initialize WeChat Pay client
	if config.WechatMchID != "" && config.WechatAPIKeyV3 != "" {
		client, err := wechat.NewClientV3(config.WechatMchID, config.WechatSerialNo, config.WechatAPIKeyV3, config.WechatPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create wechat client: %w", err)
		}
		s.wechatClient = client
	}

	return s, nil
}

// generateOrderID generates a unique order ID
func generateOrderID() string {
	timestamp := time.Now().Format("20060102150405")
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	return fmt.Sprintf("CS%s%s", timestamp, hex.EncodeToString(randomBytes))
}

// CreateRechargeOrder creates a recharge order
func (s *PaymentService) CreateRechargeOrder(userID string, amount int64, method PaymentMethod, description string) (*PaymentOrder, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	// Calculate credits (1 CNY = 1000 credits)
	credits := amount * 10 // amount is in cents, 1 cent = 10 credits

	order := &PaymentOrder{
		OrderID:     generateOrderID(),
		UserID:      userID,
		Amount:      amount,
		Currency:    "CNY",
		Credits:     credits,
		Method:      method,
		Status:      PaymentStatusPending,
		Description: description,
		CreatedAt:   time.Now(),
		ExpiredAt:   time.Now().Add(30 * time.Minute),
	}

	if description == "" {
		order.Description = fmt.Sprintf("CodeSwitch credits recharge - %d credits", credits)
	}

	// Create payment based on method
	var err error
	switch method {
	case PaymentMethodAlipay:
		err = s.createAlipayOrder(order, config)
	case PaymentMethodWechat:
		err = s.createWechatOrder(order, config)
	default:
		return nil, fmt.Errorf("unsupported payment method: %s", method)
	}

	if err != nil {
		return nil, err
	}

	// Store order
	s.orders.Store(order.OrderID, order)

	return order, nil
}

// createAlipayOrder creates an Alipay payment order
func (s *PaymentService) createAlipayOrder(order *PaymentOrder, config *PaymentConfig) error {
	if s.alipayClient == nil {
		return fmt.Errorf("alipay client not configured")
	}

	bm := make(gopay.BodyMap)
	bm.Set("subject", order.Description)
	bm.Set("out_trade_no", order.OrderID)
	bm.Set("total_amount", fmt.Sprintf("%.2f", float64(order.Amount)/100))
	bm.Set("product_code", "FAST_INSTANT_TRADE_PAY")

	// Create page pay (web payment)
	payURL, err := s.alipayClient.TradePagePay(context.Background(), bm)
	if err != nil {
		return fmt.Errorf("failed to create alipay order: %w", err)
	}

	order.PayURL = payURL

	return nil
}

// createWechatOrder creates a WeChat payment order
func (s *PaymentService) createWechatOrder(order *PaymentOrder, config *PaymentConfig) error {
	if s.wechatClient == nil {
		return fmt.Errorf("wechat pay client not configured")
	}

	bm := make(gopay.BodyMap)
	bm.Set("appid", config.WechatAppID)
	bm.Set("mchid", config.WechatMchID)
	bm.Set("description", order.Description)
	bm.Set("out_trade_no", order.OrderID)
	bm.Set("notify_url", config.NotifyURL)
	bm.SetBodyMap("amount", func(bm gopay.BodyMap) {
		bm.Set("total", order.Amount)
		bm.Set("currency", "CNY")
	})

	// Create native payment (QR code)
	resp, err := s.wechatClient.V3TransactionNative(context.Background(), bm)
	if err != nil {
		return fmt.Errorf("failed to create wechat order: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("wechat pay error: %s", resp.Error)
	}

	order.QRCode = resp.Response.CodeUrl

	return nil
}

// GetOrder gets an order by ID
func (s *PaymentService) GetOrder(orderID string) (*PaymentOrder, error) {
	if v, ok := s.orders.Load(orderID); ok {
		return v.(*PaymentOrder), nil
	}
	return nil, fmt.Errorf("order not found: %s", orderID)
}

// HandleAlipayCallback handles Alipay payment callback
func (s *PaymentService) HandleAlipayCallback(params map[string]string) (*PaymentCallback, error) {
	if s.alipayClient == nil {
		return nil, fmt.Errorf("alipay client not configured")
	}

	// Verify signature
	ok, err := alipay.VerifySign(s.config.AlipayPublicKey, params)
	if err != nil || !ok {
		return nil, fmt.Errorf("signature verification failed")
	}

	orderID := params["out_trade_no"]
	tradeNo := params["trade_no"]
	tradeStatus := params["trade_status"]

	callback := &PaymentCallback{
		Method:  PaymentMethodAlipay,
		OrderID: orderID,
		TradeNo: tradeNo,
	}

	// Parse amount
	if totalAmount, ok := params["total_amount"]; ok {
		var amount float64
		fmt.Sscanf(totalAmount, "%f", &amount)
		callback.Amount = int64(amount * 100)
	}

	// Map status
	switch tradeStatus {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		callback.Status = PaymentStatusPaid
	case "TRADE_CLOSED":
		callback.Status = PaymentStatusCancelled
	default:
		callback.Status = PaymentStatusPending
	}

	// Process payment
	if callback.Status == PaymentStatusPaid {
		if err := s.completePayment(callback); err != nil {
			return nil, err
		}
	}

	return callback, nil
}

// WechatNotifyResult represents decrypted WeChat notification
type WechatNotifyResult struct {
	OutTradeNo    string `json:"out_trade_no"`
	TransactionId string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	Amount        struct {
		Total int `json:"total"`
	} `json:"amount"`
}

// HandleWechatCallback handles WeChat payment callback
// Note: The actual decryption depends on gopay version, this is a simplified version
func (s *PaymentService) HandleWechatCallback(notifyReq *wechat.V3NotifyReq) (*PaymentCallback, error) {
	if s.wechatClient == nil {
		return nil, fmt.Errorf("wechat pay client not configured")
	}

	// Decrypt notification using V3 API
	// The exact method depends on gopay version - adjust as needed
	result := new(WechatNotifyResult)
	err := wechat.V3DecryptNotifyCipherTextToStruct(
		notifyReq.Resource.Ciphertext,
		notifyReq.Resource.Nonce,
		notifyReq.Resource.AssociatedData,
		s.config.WechatAPIKeyV3,
		result,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt notification: %w", err)
	}

	callback := &PaymentCallback{
		Method:  PaymentMethodWechat,
		OrderID: result.OutTradeNo,
		TradeNo: result.TransactionId,
		Amount:  int64(result.Amount.Total),
	}

	// Map status
	switch result.TradeState {
	case "SUCCESS":
		callback.Status = PaymentStatusPaid
	case "CLOSED", "REVOKED":
		callback.Status = PaymentStatusCancelled
	case "REFUND":
		callback.Status = PaymentStatusRefunded
	default:
		callback.Status = PaymentStatusPending
	}

	// Process payment
	if callback.Status == PaymentStatusPaid {
		if err := s.completePayment(callback); err != nil {
			return nil, err
		}
	}

	return callback, nil
}

// completePayment completes a successful payment
func (s *PaymentService) completePayment(callback *PaymentCallback) error {
	// Get order
	order, err := s.GetOrder(callback.OrderID)
	if err != nil {
		return err
	}

	// Check if already processed
	if order.Status == PaymentStatusPaid {
		return nil
	}

	// Update order status
	now := time.Now()
	order.Status = PaymentStatusPaid
	order.TradeNo = callback.TradeNo
	order.PaidAt = &now

	// Add credits to Lago wallet
	if s.lagoService != nil && s.lagoService.IsEnabled() {
		// Get user's wallet
		wallets, err := s.lagoService.GetCustomerWallets(order.UserID)
		if err != nil {
			return fmt.Errorf("failed to get wallets: %w", err)
		}

		if len(wallets) == 0 {
			return fmt.Errorf("no wallet found for user: %s", order.UserID)
		}

		// Top up wallet with paid credits
		_, err = s.lagoService.TopUpWallet(
			wallets[0].LagoID,
			fmt.Sprintf("%d", order.Credits),
			"0", // No granted credits
		)
		if err != nil {
			return fmt.Errorf("failed to top up wallet: %w", err)
		}
	}

	// Store updated order
	s.orders.Store(order.OrderID, order)

	return nil
}

// QueryAlipayOrder queries Alipay order status
func (s *PaymentService) QueryAlipayOrder(orderID string) (*PaymentOrder, error) {
	if s.alipayClient == nil {
		return nil, fmt.Errorf("alipay client not configured")
	}

	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", orderID)

	resp, err := s.alipayClient.TradeQuery(context.Background(), bm)
	if err != nil {
		return nil, fmt.Errorf("failed to query order: %w", err)
	}

	order, err := s.GetOrder(orderID)
	if err != nil {
		return nil, err
	}

	// Update status based on response
	switch resp.Response.TradeStatus {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		order.Status = PaymentStatusPaid
		order.TradeNo = resp.Response.TradeNo
	case "TRADE_CLOSED":
		order.Status = PaymentStatusCancelled
	}

	s.orders.Store(orderID, order)
	return order, nil
}

// QueryWechatOrder queries WeChat order status
func (s *PaymentService) QueryWechatOrder(orderID string) (*PaymentOrder, error) {
	if s.wechatClient == nil {
		return nil, fmt.Errorf("wechat pay client not configured")
	}

	resp, err := s.wechatClient.V3TransactionQueryOrder(context.Background(), wechat.OutTradeNo, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query order: %w", err)
	}

	order, err := s.GetOrder(orderID)
	if err != nil {
		return nil, err
	}

	// Update status based on response
	switch resp.Response.TradeState {
	case "SUCCESS":
		order.Status = PaymentStatusPaid
		order.TradeNo = resp.Response.TransactionId
	case "CLOSED", "REVOKED":
		order.Status = PaymentStatusCancelled
	}

	s.orders.Store(orderID, order)
	return order, nil
}

// RefundOrder refunds a paid order
func (s *PaymentService) RefundOrder(orderID, reason string) error {
	order, err := s.GetOrder(orderID)
	if err != nil {
		return err
	}

	if order.Status != PaymentStatusPaid {
		return fmt.Errorf("order is not paid")
	}

	refundID := fmt.Sprintf("RF%s", orderID[2:])

	switch order.Method {
	case PaymentMethodAlipay:
		return s.refundAlipay(order, refundID, reason)
	case PaymentMethodWechat:
		return s.refundWechat(order, refundID, reason)
	default:
		return fmt.Errorf("unsupported payment method")
	}
}

func (s *PaymentService) refundAlipay(order *PaymentOrder, refundID, reason string) error {
	if s.alipayClient == nil {
		return fmt.Errorf("alipay client not configured")
	}

	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", order.OrderID)
	bm.Set("refund_amount", fmt.Sprintf("%.2f", float64(order.Amount)/100))
	bm.Set("out_request_no", refundID)
	bm.Set("refund_reason", reason)

	_, err := s.alipayClient.TradeRefund(context.Background(), bm)
	if err != nil {
		return fmt.Errorf("failed to refund: %w", err)
	}

	order.Status = PaymentStatusRefunded
	s.orders.Store(order.OrderID, order)

	return nil
}

func (s *PaymentService) refundWechat(order *PaymentOrder, refundID, reason string) error {
	if s.wechatClient == nil {
		return fmt.Errorf("wechat pay client not configured")
	}

	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", order.OrderID)
	bm.Set("out_refund_no", refundID)
	bm.Set("reason", reason)
	bm.SetBodyMap("amount", func(bm gopay.BodyMap) {
		bm.Set("refund", order.Amount)
		bm.Set("total", order.Amount)
		bm.Set("currency", "CNY")
	})

	_, err := s.wechatClient.V3Refund(context.Background(), bm)
	if err != nil {
		return fmt.Errorf("failed to refund: %w", err)
	}

	order.Status = PaymentStatusRefunded
	s.orders.Store(order.OrderID, order)

	return nil
}

// UpdateConfig updates the configuration
func (s *PaymentService) UpdateConfig(config *PaymentConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config

	// Reinitialize clients
	if config.AlipayAppID != "" && config.AlipayPrivateKey != "" {
		client, err := alipay.NewClient(config.AlipayAppID, config.AlipayPrivateKey, !config.AlipaySandbox)
		if err != nil {
			return fmt.Errorf("failed to create alipay client: %w", err)
		}
		if config.AlipayPublicKey != "" {
			client.SetAliPayPublicCertSN(config.AlipayPublicKey)
		}
		s.alipayClient = client
	}

	if config.WechatMchID != "" && config.WechatAPIKeyV3 != "" {
		client, err := wechat.NewClientV3(config.WechatMchID, config.WechatSerialNo, config.WechatAPIKeyV3, config.WechatPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to create wechat client: %w", err)
		}
		s.wechatClient = client
	}

	return nil
}

// IsAlipayEnabled checks if Alipay is configured
func (s *PaymentService) IsAlipayEnabled() bool {
	return s.alipayClient != nil
}

// IsWechatEnabled checks if WeChat Pay is configured
func (s *PaymentService) IsWechatEnabled() bool {
	return s.wechatClient != nil
}
