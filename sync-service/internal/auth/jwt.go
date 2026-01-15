package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret          string        `yaml:"secret"`
	Issuer          string        `yaml:"issuer"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
}

// Claims JWT Claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	DeviceID string `json:"device_id"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// JWTManager JWT 管理器
type JWTManager struct {
	config *JWTConfig
}

// NewJWTManager 创建 JWT 管理器
func NewJWTManager(cfg *JWTConfig) *JWTManager {
	return &JWTManager{config: cfg}
}

// GenerateAccessToken 生成访问令牌
func (m *JWTManager) GenerateAccessToken(userID, username, deviceID string, isAdmin bool) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		DeviceID: deviceID,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.AccessTokenTTL)),
			ID:        generateTokenID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.Secret))
}

// GenerateRefreshToken 生成刷新令牌
func (m *JWTManager) GenerateRefreshToken(userID, deviceID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    m.config.Issuer,
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.config.RefreshTokenTTL)),
		ID:        deviceID + ":" + generateTokenID(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.Secret))
}

// ValidateToken 验证令牌
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(m.config.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ValidateRefreshToken 验证刷新令牌
func (m *JWTManager) ValidateRefreshToken(tokenString string) (userID string, deviceID string, err error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(m.config.Secret), nil
	})

	if err != nil {
		return "", "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", "", ErrInvalidToken
	}

	return claims.Subject, extractDeviceID(claims.ID), nil
}

// GetAccessTokenTTL 获取访问令牌有效期
func (m *JWTManager) GetAccessTokenTTL() time.Duration {
	return m.config.AccessTokenTTL
}

func generateTokenID() string {
	return time.Now().Format("20060102150405.000000")
}

func extractDeviceID(jti string) string {
	for i, c := range jti {
		if c == ':' {
			return jti[:i]
		}
	}
	return ""
}
