package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// RustfsProvider RustFS 存储（兼容 S3 协议，复用 MinIO SDK）
type RustfsProvider struct {
	endpoint  string
	accessKey string
	secretKey string
	bucket    string
	useSSL    bool
	// TODO: 复用 minio.Client 连接 RustFS
}

// NewRustfsProvider 创建 RustFS Provider
func NewRustfsProvider(cfg *StorageConfig) (*RustfsProvider, error) {
	if cfg.RustfsEndpoint == "" || cfg.RustfsAccessKey == "" || cfg.RustfsBucket == "" {
		return nil, fmt.Errorf("rustfs config incomplete")
	}
	return &RustfsProvider{
		endpoint:  cfg.RustfsEndpoint,
		accessKey: cfg.RustfsAccessKey,
		secretKey: cfg.RustfsSecretKey,
		bucket:    cfg.RustfsBucket,
		useSSL:    cfg.RustfsUseSSL,
	}, nil
}

func (p *RustfsProvider) Upload(ctx context.Context, bucket, key string, reader io.Reader, contentType string) (string, error) {
	// TODO: 复用 minio.Client.PutObject 连接 RustFS
	scheme := "http"
	if p.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, p.endpoint, p.bucket, key), nil
}

func (p *RustfsProvider) Delete(ctx context.Context, bucket, key string) error {
	// TODO: 复用 minio.Client.RemoveObject
	return nil
}

func (p *RustfsProvider) GetSignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	// TODO: 复用 minio.Client.PresignedGetObject
	return fmt.Sprintf("http://%s/%s/%s?signed=todo", p.endpoint, p.bucket, key), nil
}
