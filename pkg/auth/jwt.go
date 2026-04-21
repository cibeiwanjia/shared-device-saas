package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义 JWT Claims
type Claims struct {
	UserID    int64    `json:"user_id"`
	TenantID  int64    `json:"tenant_id"`
	SessionID string   `json:"session_id"`
	DeviceID  string   `json:"device_id"`
	Roles     []string `json:"roles"`
	jwt.RegisteredClaims
}

// TokenPair Access Token + Refresh Token 对
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // Access Token 有效期（秒）
}

// JWTManager JWT 签发与解析
type JWTManager struct {
	accessSecret  string
	refreshSecret string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewJWTManager 创建 JWT 管理器
func NewJWTManager(accessSecret, refreshSecret string, accessExpiry, refreshExpiry time.Duration) *JWTManager {
	return &JWTManager{
		accessSecret:  accessSecret,
		refreshSecret: refreshSecret,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// GenerateTokenPair 签发 Access Token + Refresh Token
func (m *JWTManager) GenerateTokenPair(userID, tenantID int64, sessionID, deviceID string, roles []string) (*TokenPair, error) {
	now := time.Now()
	jti := NewJTI()

	accessClaims := Claims{
		UserID:    userID,
		TenantID:  tenantID,
		SessionID: sessionID,
		DeviceID:  deviceID,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiry)),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(m.accessSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshClaims := Claims{
		UserID:    userID,
		TenantID:  tenantID,
		SessionID: sessionID,
		DeviceID:  deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        NewJTI(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiry)),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(m.refreshSecret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		ExpiresIn:    int64(m.accessExpiry.Seconds()),
	}, nil
}

// ParseAccessToken 解析 Access Token
func (m *JWTManager) ParseAccessToken(tokenStr string) (*Claims, error) {
	return m.parseToken(tokenStr, m.accessSecret)
}

// ParseRefreshToken 解析 Refresh Token
func (m *JWTManager) ParseRefreshToken(tokenStr string) (*Claims, error) {
	return m.parseToken(tokenStr, m.refreshSecret)
}

func (m *JWTManager) parseToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
