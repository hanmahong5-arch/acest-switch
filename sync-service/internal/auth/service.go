package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aspect-code/codeswitch/sync-service/pkg/models"
)

// Service 认证服务
type Service struct {
	jwt       *JWTManager
	newapi    *NewAPIClient
	logger    *slog.Logger

	// 内存缓存 (生产环境应使用 Redis)
	userCache sync.Map // map[userID]*models.User
	tokenCache sync.Map // map[token]*models.User
}

// NewService 创建认证服务
func NewService(jwtCfg *JWTConfig, newapiCfg *NewAPIConfig, logger *slog.Logger) *Service {
	return &Service{
		jwt:    NewJWTManager(jwtCfg),
		newapi: NewNewAPIClient(newapiCfg),
		logger: logger,
	}
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, req *models.AuthRequest) (*models.AuthResponse, error) {
	// 1. 验证 NEW-API Token
	user, err := s.newapi.ValidateToken(ctx, req.Token)
	if err != nil {
		s.logger.Warn("Login failed: invalid NEW-API token", "error", err)
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.logger.Info("User authenticated via NEW-API",
		"user_id", user.ID,
		"username", user.Username,
		"device_id", req.DeviceID,
	)

	// 2. 生成 JWT Token
	accessToken, err := s.jwt.GenerateAccessToken(user.ID, user.Username, req.DeviceID, user.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwt.GenerateRefreshToken(user.ID, req.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 3. 缓存用户信息
	s.userCache.Store(user.ID, user)

	// 4. 记录设备信息
	device := &models.Device{
		ID:            models.GenerateID(),
		UserID:        user.ID,
		DeviceID:      req.DeviceID,
		DeviceName:    req.DeviceName,
		DeviceType:    req.DeviceType,
		ClientVersion: req.ClientVersion,
		LastSeenAt:    time.Now(),
	}
	s.logger.Info("Device registered", "device", device)

	return &models.AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwt.GetAccessTokenTTL().Seconds()),
	}, nil
}

// ValidateToken 验证访问令牌
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.jwt.ValidateToken(tokenString)
}

// RefreshToken 刷新令牌
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	// 1. 验证刷新令牌
	userID, deviceID, err := s.jwt.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// 2. 获取用户信息 (从缓存或重新获取)
	var user *models.User
	if cached, ok := s.userCache.Load(userID); ok {
		user = cached.(*models.User)
	} else {
		// 需要重新从 NEW-API 获取，这里简化处理
		user = &models.User{
			ID:       userID,
			Username: "user_" + userID,
		}
	}

	// 3. 生成新的访问令牌
	accessToken, err := s.jwt.GenerateAccessToken(user.ID, user.Username, deviceID, user.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// 4. 生成新的刷新令牌
	newRefreshToken, err := s.jwt.GenerateRefreshToken(user.ID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &models.AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(s.jwt.GetAccessTokenTTL().Seconds()),
	}, nil
}

// GetUser 获取用户信息
func (s *Service) GetUser(userID string) (*models.User, bool) {
	if cached, ok := s.userCache.Load(userID); ok {
		return cached.(*models.User), true
	}
	return nil, false
}

// Logout 用户登出
func (s *Service) Logout(userID, deviceID string) {
	s.logger.Info("User logged out", "user_id", userID, "device_id", deviceID)
	// 在生产环境中，应该将令牌加入黑名单
}
