package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义 JWT Claims
type Claims struct {
	UserID    string   `json:"user_id"` // MongoDB ObjectID.Hex() (改为 string)
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
	accessSecret  string        // Access Token 密钥
	refreshSecret string        // Refresh Token 密钥
	accessExpiry  time.Duration // Access Token 有效期（秒）
	refreshExpiry time.Duration // Refresh Token 有效期（秒）
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
func (m *JWTManager) GenerateTokenPair(userID string, tenantID int64, sessionID, deviceID string, roles []string) (*TokenPair, error) {
	now := time.Now() // 当前时间
	jti := NewJTI()   // 生成唯一的 Token ID

	accessClaims := Claims{
		UserID:    userID,    // 直接使用 MongoDB ObjectID.Hex()
		TenantID:  tenantID,  // 租户ID
		SessionID: sessionID, // Token会话ID
		DeviceID:  deviceID,  // 设备ID
		Roles:     roles,     // 角色
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiry)),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims) // 签发 Access Token
	accessStr, err := accessToken.SignedString([]byte(m.accessSecret))     // 签发 Access Token 字符串
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshClaims := Claims{
		UserID:    userID,    // 直接使用 MongoDB ObjectID.Hex()
		TenantID:  tenantID,  // 租户ID
		SessionID: sessionID, // Token会话ID
		DeviceID:  deviceID,  // 设备ID
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        NewJTI(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiry)),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims) // 签发 Refresh Token
	refreshStr, err := refreshToken.SignedString([]byte(m.refreshSecret))    // 签发 Refresh Token 字符串
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessStr,                       // Access Token 字符串
		RefreshToken: refreshStr,                      // Refresh Token 字符串
		ExpiresIn:    int64(m.accessExpiry.Seconds()), // Access Token 有效期（秒）
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
