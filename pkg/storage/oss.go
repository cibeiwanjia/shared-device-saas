package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// OSSProvider 阿里云 OSS 存储
type OSSProvider struct {
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	bucket          string
	// TODO: 接入阿里云 OSS SDK 后替换为 oss.Client
}

// NewOSSProvider 创建阿里云 OSS Provider
func NewOSSProvider(cfg *StorageConfig) (*OSSProvider, error) {
	if cfg.OSSEndpoint == "" || cfg.OSSAccessKeyID == "" || cfg.OSSBucket == "" {
		return nil, fmt.Errorf("oss config incomplete")
	}
	return &OSSProvider{
		endpoint:        cfg.OSSEndpoint,
		accessKeyID:     cfg.OSSAccessKeyID,
		accessKeySecret: cfg.OSSAccessKeySecret,
		bucket:          cfg.OSSBucket,
	}, nil
}

func (p *OSSProvider) Upload(ctx context.Context, bucket, key string, reader io.Reader, contentType string) (string, error) {
	// TODO: 接入 oss.Client.PutObject
	return fmt.Sprintf("https://%s.%s/%s", p.bucket, p.endpoint, key), nil
}

func (p *OSSProvider) Delete(ctx context.Context, bucket, key string) error {
	// TODO: 接入 oss.Client.DeleteObject
	return nil
}

func (p *OSSProvider) GetSignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	// TODO: 接入 oss.Client.SignURL
	return fmt.Sprintf("https://%s.%s/%s?sign=todo&expires=%d", p.bucket, p.endpoint, key, int64(expiry.Seconds())), nil
}
