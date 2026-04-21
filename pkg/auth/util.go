package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewJTI 生成唯一的 Token ID
func NewJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HashToken 对 Token 进行 SHA256 哈希（用于存储 Refresh Token）
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
