package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/pkg/models"
)

// ===== Lago Client =====

// LagoClient is a client for Lago billing API
type LagoClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewLagoClient creates a new Lago client
func NewLagoClient(baseURL, apiKey string) *LagoClient {
	return &LagoClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TestConnection tests the connection to Lago
func (c *LagoClient) TestConnection() models.TestConnectionResponse {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/customers", nil)
	if err != nil {
		return models.TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Connection failed: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return models.TestConnectionResponse{
			Success: true,
			Message: "Connection successful",
		}
	}

	body, _ := io.ReadAll(resp.Body)
	return models.TestConnectionResponse{
		Success: false,
		Message: fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
	}
}

// CreateSubscription creates a subscription in Lago
func (c *LagoClient) CreateSubscription(customerID, planCode string) (string, error) {
	payload := map[string]interface{}{
		"subscription": map[string]interface{}{
			"external_customer_id": customerID,
			"plan_code":            planCode,
			"billing_time":         "anniversary",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/subscriptions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Lago API error: %s", string(respBody))
	}

	var result struct {
		Subscription struct {
			LagoID string `json:"lago_id"`
		} `json:"subscription"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Subscription.LagoID, nil
}

// CancelSubscription cancels a subscription in Lago
func (c *LagoClient) CancelSubscription(externalID string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/api/v1/subscriptions/"+externalID, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Lago API error: %s", string(body))
	}

	return nil
}

// CreateWallet creates a wallet in Lago
func (c *LagoClient) CreateWallet(customerID, name, currency, rateAmount string) (string, error) {
	payload := map[string]interface{}{
		"wallet": map[string]interface{}{
			"external_customer_id": customerID,
			"name":                 name,
			"currency":             currency,
			"rate_amount":          rateAmount,
			"granted_credits":      "0",
			"paid_credits":         "0",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/wallets", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Lago API error: %s", string(respBody))
	}

	var result struct {
		Wallet struct {
			LagoID string `json:"lago_id"`
		} `json:"wallet"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Wallet.LagoID, nil
}

// TopUpWallet tops up a wallet in Lago
func (c *LagoClient) TopUpWallet(walletID, paidCredits, grantedCredits string) error {
	payload := map[string]interface{}{
		"wallet_transaction": map[string]interface{}{
			"wallet_id":       walletID,
			"paid_credits":    paidCredits,
			"granted_credits": grantedCredits,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/wallet_transactions", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Lago API error: %s", string(respBody))
	}

	return nil
}

// ===== Casdoor Client =====

// CasdoorClient is a client for Casdoor authentication
type CasdoorClient struct {
	endpoint     string
	clientID     string
	clientSecret string
	organization string
	application  string
	certificate  string
	httpClient   *http.Client
}

// NewCasdoorClient creates a new Casdoor client
func NewCasdoorClient(endpoint, clientID, clientSecret, organization, application, certificate string) *CasdoorClient {
	return &CasdoorClient{
		endpoint:     endpoint,
		clientID:     clientID,
		clientSecret: clientSecret,
		organization: organization,
		application:  application,
		certificate:  certificate,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TestConnection tests the connection to Casdoor
func (c *CasdoorClient) TestConnection() models.TestConnectionResponse {
	url := fmt.Sprintf("%s/api/get-application?id=%s/%s", c.endpoint, c.organization, c.application)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return models.TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Connection failed: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return models.TestConnectionResponse{
			Success: true,
			Message: "Connection successful",
		}
	}

	body, _ := io.ReadAll(resp.Body)
	return models.TestConnectionResponse{
		Success: false,
		Message: fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
	}
}

// GetUser gets a user from Casdoor
func (c *CasdoorClient) GetUser(userID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/get-user?id=%s/%s", c.endpoint, c.organization, userID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Casdoor API error: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// ===== Alipay Client =====

// AlipayClient is a client for Alipay payments
type AlipayClient struct {
	appID      string
	privateKey string
	publicKey  string
	sandbox    bool
	notifyURL  string
	returnURL  string
	httpClient *http.Client
}

// NewAlipayClient creates a new Alipay client
func NewAlipayClient(appID, privateKey, publicKey string, sandbox bool, notifyURL, returnURL string) *AlipayClient {
	return &AlipayClient{
		appID:      appID,
		privateKey: privateKey,
		publicKey:  publicKey,
		sandbox:    sandbox,
		notifyURL:  notifyURL,
		returnURL:  returnURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TestConnection tests the Alipay configuration
func (c *AlipayClient) TestConnection() models.TestConnectionResponse {
	// Validate configuration
	if c.appID == "" {
		return models.TestConnectionResponse{
			Success: false,
			Message: "App ID is not configured",
		}
	}
	if c.privateKey == "" {
		return models.TestConnectionResponse{
			Success: false,
			Message: "Private key is not configured",
		}
	}

	// In production, we would make a test API call
	// For now, just validate configuration exists
	return models.TestConnectionResponse{
		Success: true,
		Message: fmt.Sprintf("Configuration valid (sandbox: %v)", c.sandbox),
	}
}

// CreatePayment creates a payment with Alipay
func (c *AlipayClient) CreatePayment(orderNo string, amountCents int64, description string) (string, error) {
	// In production, this would use the Alipay SDK to create a payment
	// For demo purposes, return a placeholder URL
	baseURL := "https://openapi.alipay.com"
	if c.sandbox {
		baseURL = "https://openapi.alipaydev.com"
	}

	// This is a simplified version - real implementation would use proper signing
	payURL := fmt.Sprintf("%s/gateway.do?app_id=%s&out_trade_no=%s&total_amount=%.2f",
		baseURL, c.appID, orderNo, float64(amountCents)/100)

	return payURL, nil
}

// Refund refunds a payment via Alipay
func (c *AlipayClient) Refund(orderNo string, amountCents int64) error {
	// In production, this would use the Alipay SDK to process refund
	// For demo purposes, just return success
	return nil
}

// ===== WeChat Pay Client =====

// WechatClient is a client for WeChat Pay
type WechatClient struct {
	appID      string
	mchID      string
	apiKey     string
	apiKeyV3   string
	serialNo   string
	privateKey string
	notifyURL  string
	httpClient *http.Client
}

// NewWechatClient creates a new WeChat Pay client
func NewWechatClient(appID, mchID, apiKey, apiKeyV3, serialNo, privateKey, notifyURL string) *WechatClient {
	return &WechatClient{
		appID:      appID,
		mchID:      mchID,
		apiKey:     apiKey,
		apiKeyV3:   apiKeyV3,
		serialNo:   serialNo,
		privateKey: privateKey,
		notifyURL:  notifyURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TestConnection tests the WeChat Pay configuration
func (c *WechatClient) TestConnection() models.TestConnectionResponse {
	// Validate configuration
	if c.appID == "" {
		return models.TestConnectionResponse{
			Success: false,
			Message: "App ID is not configured",
		}
	}
	if c.mchID == "" {
		return models.TestConnectionResponse{
			Success: false,
			Message: "Merchant ID is not configured",
		}
	}

	// In production, we would make a test API call
	return models.TestConnectionResponse{
		Success: true,
		Message: "Configuration valid",
	}
}

// CreatePayment creates a payment with WeChat Pay
func (c *WechatClient) CreatePayment(orderNo string, amountCents int64, description string) (string, error) {
	// In production, this would use the WeChat Pay SDK to create a native payment
	// For demo purposes, return a placeholder URL
	// Real implementation would return code_url for QR code payment

	payURL := fmt.Sprintf("weixin://wxpay/bizpayurl?sr=%s&total=%d", orderNo, amountCents)
	return payURL, nil
}

// Refund refunds a payment via WeChat Pay
func (c *WechatClient) Refund(orderNo string, amountCents int64) error {
	// In production, this would use the WeChat Pay SDK to process refund
	// For demo purposes, just return success
	return nil
}
