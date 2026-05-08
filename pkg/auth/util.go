package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewJTI 生成唯一的 Token ID
func NewJTI() string {
	b := make([]byte, 16)        // 16字节随机数
	_, _ = rand.Read(b)          // 生成随机数
	return hex.EncodeToString(b) // 转换为十六进制字符串
}

// HashToken 对 Token 进行 SHA256 哈希（用于存储 Refresh Token）
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token)) // 计算哈希值
	return hex.EncodeToString(h[:])   // 转换为十六进制字符串
}
