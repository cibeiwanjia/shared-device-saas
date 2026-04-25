package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// MinioProvider MinIO 存储
type MinioProvider struct {
	endpoint  string
	accessKey string
	secretKey string
	bucket    string
	useSSL    bool
	// TODO: 接入 minio.Client
}

// NewMinioProvider 创建 MinIO Provider
func NewMinioProvider(cfg *StorageConfig) (*MinioProvider, error) {
	if cfg.MinioEndpoint == "" || cfg.MinioAccessKey == "" || cfg.MinioBucket == "" {
		return nil, fmt.Errorf("minio config incomplete")
	}
	return &MinioProvider{
		endpoint:  cfg.MinioEndpoint,
		accessKey: cfg.MinioAccessKey,
		secretKey: cfg.MinioSecretKey,
		bucket:    cfg.MinioBucket,
		useSSL:    cfg.MinioUseSSL,
	}, nil
}

func (p *MinioProvider) Upload(ctx context.Context, bucket, key string, reader io.Reader, contentType string) (string, error) {
	// TODO: 接入 minio.Client.PutObject
	scheme := "http"
	if p.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, p.endpoint, p.bucket, key), nil
}

func (p *MinioProvider) Delete(ctx context.Context, bucket, key string) error {
	// TODO: 接入 minio.Client.RemoveObject
	return nil
}

func (p *MinioProvider) GetSignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	// TODO: 接入 minio.Client.PresignedGetObject
	return fmt.Sprintf("http://%s/%s/%s?signed=todo", p.endpoint, p.bucket, key), nil
}
