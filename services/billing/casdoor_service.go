package billing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CasdoorConfig Casdoor configuration
type CasdoorConfig struct {
	Endpoint     string `json:"endpoint"`      // e.g., "http://localhost:8000"
	ClientID     string `json:"client_id"`     // Application client ID
	ClientSecret string `json:"client_secret"` // Application client secret
	Organization string `json:"organization"`  // Organization name
	Application  string `json:"application"`   // Application name
	Certificate  string `json:"certificate"`   // JWT certificate for token verification
}

// CasdoorUser represents a Casdoor user
type CasdoorUser struct {
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Avatar      string `json:"avatar"`
	Type        string `json:"type"`
	IsAdmin     bool   `json:"isAdmin"`
	CreatedTime string `json:"createdTime"`
	UpdatedTime string `json:"updatedTime"`
}

// CasdoorToken represents OAuth token response
type CasdoorToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token,omitempty"`
}

// CasdoorClaims JWT claims from Casdoor
type CasdoorClaims struct {
	jwt.RegisteredClaims
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Avatar      string `json:"avatar"`
	Type        string `json:"type"`
	IsAdmin     bool   `json:"isAdmin"`
}

// CasdoorService handles Casdoor authentication
type CasdoorService struct {
	config     *CasdoorConfig
	httpClient *http.Client
	jwtKey     interface{}
	mu         sync.RWMutex
}

// NewCasdoorService creates a new Casdoor service
func NewCasdoorService(config *CasdoorConfig) *CasdoorService {
	return &CasdoorService{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetAuthURL returns the OAuth authorization URL
func (s *CasdoorService) GetAuthURL(redirectURI, state string) string {
	params := url.Values{}
	params.Set("client_id", s.config.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "openid profile email")
	params.Set("state", state)

	return fmt.Sprintf("%s/login/oauth/authorize?%s", s.config.Endpoint, params.Encode())
}

// ExchangeToken exchanges authorization code for tokens
func (s *CasdoorService) ExchangeToken(code, redirectURI string) (*CasdoorToken, error) {
	tokenURL := fmt.Sprintf("%s/api/login/oauth/access_token", s.config.Endpoint)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token CasdoorToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	return &token, nil
}

// RefreshToken refreshes an access token
func (s *CasdoorService) RefreshToken(refreshToken string) (*CasdoorToken, error) {
	tokenURL := fmt.Sprintf("%s/api/login/oauth/refresh_token", s.config.Endpoint)

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var token CasdoorToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	return &token, nil
}

// ParseToken parses and validates a JWT token
func (s *CasdoorService) ParseToken(tokenString string) (*CasdoorClaims, error) {
	// For simplicity, we'll use the certificate from config
	// In production, you should fetch the JWKS from Casdoor
	token, err := jwt.ParseWithClaims(tokenString, &CasdoorClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			// Try HMAC if RSA fails
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(s.config.ClientSecret), nil
		}

		// Parse RSA public key from certificate
		if s.config.Certificate != "" {
			return jwt.ParseRSAPublicKeyFromPEM([]byte(s.config.Certificate))
		}

		return nil, fmt.Errorf("no certificate configured")
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*CasdoorClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// GetUserInfo gets user info from access token
func (s *CasdoorService) GetUserInfo(accessToken string) (*CasdoorUser, error) {
	userInfoURL := fmt.Sprintf("%s/api/userinfo", s.config.Endpoint)

	req, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: %s", string(body))
	}

	var user CasdoorUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &user, nil
}

// GetUser gets a specific user by ID
func (s *CasdoorService) GetUser(userID string) (*CasdoorUser, error) {
	userURL := fmt.Sprintf("%s/api/get-user?id=%s/%s", s.config.Endpoint, s.config.Organization, userID)

	req, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(s.config.ClientID, s.config.ClientSecret)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user: %s", string(body))
	}

	var result struct {
		Status string       `json:"status"`
		Msg    string       `json:"msg"`
		Data   *CasdoorUser `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("failed to get user: %s", result.Msg)
	}

	return result.Data, nil
}

// Logout invalidates a token
func (s *CasdoorService) Logout(accessToken string) error {
	logoutURL := fmt.Sprintf("%s/api/logout", s.config.Endpoint)

	data := url.Values{}
	data.Set("id_token_hint", accessToken)

	req, err := http.NewRequest("POST", logoutURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// IntrospectToken checks if a token is valid
func (s *CasdoorService) IntrospectToken(token string) (bool, error) {
	introspectURL := fmt.Sprintf("%s/api/login/oauth/introspect", s.config.Endpoint)

	data := url.Values{}
	data.Set("token", token)
	data.Set("token_type_hint", "access_token")

	req, err := http.NewRequest("POST", introspectURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.config.ClientID, s.config.ClientSecret)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to introspect token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Active bool `json:"active"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Active, nil
}

// GetConfig returns the current configuration
func (s *CasdoorService) GetConfig() *CasdoorConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates the configuration
func (s *CasdoorService) UpdateConfig(config *CasdoorConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}
