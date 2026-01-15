package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/pkg/models"
)

var (
	ErrNewAPIUnauthorized = errors.New("unauthorized: invalid NEW-API token")
	ErrNewAPIUnavailable  = errors.New("NEW-API service unavailable")
	ErrUserNotFound       = errors.New("user not found")
)

// NewAPIConfig NEW-API 配置
type NewAPIConfig struct {
	URL        string `yaml:"url"`
	AdminToken string `yaml:"admin_token"`
}

// NewAPIClient NEW-API 客户端
type NewAPIClient struct {
	config     *NewAPIConfig
	httpClient *http.Client
}

// NewAPIUserResponse NEW-API 用户信息响应
type NewAPIUserResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    *NewAPIUser    `json:"data"`
}

// NewAPIUser NEW-API 用户信息
type NewAPIUser struct {
	ID           int     `json:"id"`
	Username     string  `json:"username"`
	DisplayName  string  `json:"display_name"`
	Email        string  `json:"email"`
	Role         int     `json:"role"`          // 1=普通用户, 10=管理员, 100=超级管理员
	Status       int     `json:"status"`        // 1=启用, 2=禁用
	Quota        int64   `json:"quota"`         // 剩余配额
	UsedQuota    int64   `json:"used_quota"`    // 已用配额
	RequestCount int64   `json:"request_count"` // 请求次数
	Group        string  `json:"group"`
	AffCode      string  `json:"aff_code"`
	InviterID    int     `json:"inviter_id"`
}

// NewNewAPIClient 创建 NEW-API 客户端
func NewNewAPIClient(cfg *NewAPIConfig) *NewAPIClient {
	return &NewAPIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ValidateToken 验证 NEW-API Token 并获取用户信息
func (c *NewAPIClient) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL+"/api/user/self", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrNewAPIUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrNewAPIUnauthorized
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("NEW-API error: %s", string(body))
	}

	var apiResp NewAPIUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResp.Success || apiResp.Data == nil {
		return nil, ErrUserNotFound
	}

	// 转换为内部用户模型
	user := &models.User{
		ID:           fmt.Sprintf("%d", apiResp.Data.ID),
		NewAPIUserID: fmt.Sprintf("%d", apiResp.Data.ID),
		Username:     apiResp.Data.Username,
		Email:        apiResp.Data.Email,
		Plan:         c.determinePlan(apiResp.Data),
		QuotaTotal:   float64(apiResp.Data.Quota + apiResp.Data.UsedQuota),
		QuotaUsed:    float64(apiResp.Data.UsedQuota),
		IsAdmin:      apiResp.Data.Role >= 10,
		CreatedAt:    time.Now(),
	}

	if apiResp.Data.DisplayName != "" {
		user.Username = apiResp.Data.DisplayName
	}

	return user, nil
}

// GetUserByID 通过 ID 获取用户信息 (管理员接口)
func (c *NewAPIClient) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL+"/api/user/"+userID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.AdminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrNewAPIUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrUserNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NEW-API error: status %d", resp.StatusCode)
	}

	var apiResp NewAPIUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResp.Success || apiResp.Data == nil {
		return nil, ErrUserNotFound
	}

	return &models.User{
		ID:           fmt.Sprintf("%d", apiResp.Data.ID),
		NewAPIUserID: fmt.Sprintf("%d", apiResp.Data.ID),
		Username:     apiResp.Data.Username,
		Email:        apiResp.Data.Email,
		Plan:         c.determinePlan(apiResp.Data),
		QuotaTotal:   float64(apiResp.Data.Quota + apiResp.Data.UsedQuota),
		QuotaUsed:    float64(apiResp.Data.UsedQuota),
		IsAdmin:      apiResp.Data.Role >= 10,
	}, nil
}

// GetUserQuota 获取用户配额
func (c *NewAPIClient) GetUserQuota(ctx context.Context, token string) (remaining int64, err error) {
	user, err := c.ValidateToken(ctx, token)
	if err != nil {
		return 0, err
	}
	return int64(user.QuotaTotal - user.QuotaUsed), nil
}

func (c *NewAPIClient) determinePlan(user *NewAPIUser) string {
	if user.Role >= 100 {
		return "admin"
	}
	if user.Role >= 10 {
		return "pro"
	}
	if user.Group != "" {
		return user.Group
	}
	return "free"
}

// QuotaInfo 配额详情
type QuotaInfo struct {
	UserID        string  `json:"user_id"`
	QuotaTotal    float64 `json:"quota_total"`
	QuotaUsed     float64 `json:"quota_used"`
	QuotaRemain   float64 `json:"quota_remain"`
	RequestCount  int64   `json:"request_count"`
}

var (
	ErrQuotaInsufficient = errors.New("insufficient quota")
)

// GetQuotaInfo 获取用户配额详细信息
func (c *NewAPIClient) GetQuotaInfo(ctx context.Context, token string) (*QuotaInfo, error) {
	user, err := c.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	return &QuotaInfo{
		UserID:      user.ID,
		QuotaTotal:  user.QuotaTotal,
		QuotaUsed:   user.QuotaUsed,
		QuotaRemain: user.QuotaTotal - user.QuotaUsed,
	}, nil
}

// CheckQuota 检查用户配额是否足够
// estimatedCost: 预估消耗的配额（以 NEW-API 的配额单位，通常是 0.0001 USD = 1 配额）
// 返回: (是否足够, 剩余配额, 错误)
func (c *NewAPIClient) CheckQuota(ctx context.Context, token string, estimatedCost float64) (bool, float64, error) {
	quotaInfo, err := c.GetQuotaInfo(ctx, token)
	if err != nil {
		return false, 0, err
	}

	if quotaInfo.QuotaRemain < estimatedCost {
		return false, quotaInfo.QuotaRemain, ErrQuotaInsufficient
	}

	return true, quotaInfo.QuotaRemain, nil
}

// GetQuotaInfoByUserID 通过用户 ID 获取配额信息（管理员接口）
func (c *NewAPIClient) GetQuotaInfoByUserID(ctx context.Context, userID string) (*QuotaInfo, error) {
	user, err := c.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &QuotaInfo{
		UserID:      user.ID,
		QuotaTotal:  user.QuotaTotal,
		QuotaUsed:   user.QuotaUsed,
		QuotaRemain: user.QuotaTotal - user.QuotaUsed,
	}, nil
}
